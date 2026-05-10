package main

import (
	"context"
	"encoding/base64"
	"os/signal"
	"syscall"

	"swiggy-ssh/internal/auth"
	"swiggy-ssh/internal/cache"
	"swiggy-ssh/internal/config"
	"swiggy-ssh/internal/crypto"
	"swiggy-ssh/internal/httpserver"
	"swiggy-ssh/internal/identity"
	"swiggy-ssh/internal/logging"
	"swiggy-ssh/internal/sshserver"
	"swiggy-ssh/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logger := logging.New(cfg.AppEnv)

	if cfg.AppEnv == "production" && cfg.TokenEncryptionKey == config.DefaultDevTokenEncryptionKey {
		logger.ErrorContext(ctx, "TOKEN_ENCRYPTION_KEY must be set in production; refusing to start with the dev default key")
		return
	}

	keyBytes, err := base64.RawURLEncoding.DecodeString(cfg.TokenEncryptionKey)
	if err != nil {
		logger.ErrorContext(ctx, "invalid TOKEN_ENCRYPTION_KEY: base64 decode failed", "error", err)
		return
	}
	encryptor, err := crypto.NewAESGCMEncryptor(keyBytes)
	if err != nil {
		logger.ErrorContext(ctx, "invalid TOKEN_ENCRYPTION_KEY", "error", err)
		return
	}

	postgresStore, err := store.NewPostgresStore(ctx, cfg.DatabaseURL, encryptor)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize postgres store", "error", err)
		return
	}
	defer postgresStore.Close()

	redisClient, err := cache.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize redis client", "error", err)
		return
	}
	defer redisClient.Close()

	loginCodeSvc := cache.NewRedisLoginCodeService(redisClient, cfg.LoginCodeTTL)
	logger.InfoContext(ctx, "login code service ready", "ttl", cfg.LoginCodeTTL)

	authSvc := auth.NewAuthService(postgresStore, loginCodeSvc)

	resolver := identity.NewResolver(postgresStore)
	tracker := identity.NewSessionTracker(postgresStore)
	server := sshserver.New(cfg.SSHAddr, cfg.SSHHostKeyPath, logger, resolver, tracker, loginCodeSvc, cfg.PublicBaseURL, authSvc)
	httpSrv := httpserver.New(cfg.HTTPAddr, logger, loginCodeSvc)

	logger.InfoContext(ctx, "swiggy-ssh scaffold startup",
		"app_env", cfg.AppEnv,
		"provider", cfg.SwiggyProvider,
		"ssh_addr", cfg.SSHAddr,
		"ssh_host_key_path", cfg.SSHHostKeyPath,
		"http_addr", cfg.HTTPAddr,
		"public_base_url", cfg.PublicBaseURL,
	)

	// Run SSH and HTTP servers concurrently; stop both on signal or first error.
	errCh := make(chan error, 2)
	go func() { errCh <- server.Start(ctx) }()
	go func() { errCh <- httpSrv.Start(ctx) }()

	if err := <-errCh; err != nil {
		logger.ErrorContext(ctx, "server exited with error", "error", err)
	}
	// ctx cancellation (from signal) stops both servers.
}
