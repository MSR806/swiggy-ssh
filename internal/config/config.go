package config

import (
	"os"
	"time"
)

const (
	defaultAppEnv         = "local"
	defaultSwiggyProvider = "mock"
	defaultSSHAddr        = ":2222"
	defaultSSHHostKeyPath = ".local/ssh_host_ed25519_key"
	defaultHTTPAddr       = ":8080"
	defaultDatabaseURL    = "postgres://swiggy:swiggy@localhost:5432/swiggy_ssh?sslmode=disable"
	defaultRedisURL       = "redis://localhost:6379/0"
	defaultPublicBaseURL  = "http://localhost:8080"

	// DefaultDevTokenEncryptionKey is the LOCAL DEVELOPMENT ONLY fallback encryption key.
	// This key is publicly known from the source code.
	// TOKEN_ENCRYPTION_KEY must be set to a securely generated value in production.
	DefaultDevTokenEncryptionKey = "c3dpZ2d5LXNzaC1sb2NhbC1kZXYta2V5LTMyYnl0ZXM"
)

// Config holds process configuration loaded from env with local defaults.
type Config struct {
	AppEnv             string
	SwiggyProvider     string
	SSHAddr            string
	SSHHostKeyPath     string
	HTTPAddr           string
	DatabaseURL        string
	RedisURL           string
	PublicBaseURL      string
	LoginCodeTTL       time.Duration
	TokenEncryptionKey string // base64url-encoded 32-byte AES-256 key; required in production
}

// Load reads runtime configuration from environment variables.
func Load() Config {
	return Config{
		AppEnv:             envOrDefault("APP_ENV", defaultAppEnv),
		SwiggyProvider:     envOrDefault("SWIGGY_PROVIDER", defaultSwiggyProvider),
		SSHAddr:            envOrDefault("SSH_ADDR", defaultSSHAddr),
		SSHHostKeyPath:     envOrDefault("SSH_HOST_KEY_PATH", defaultSSHHostKeyPath),
		HTTPAddr:           envOrDefault("HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:        envOrDefault("DATABASE_URL", defaultDatabaseURL),
		RedisURL:           envOrDefault("REDIS_URL", defaultRedisURL),
		PublicBaseURL:      envOrDefault("PUBLIC_BASE_URL", defaultPublicBaseURL),
		LoginCodeTTL:       envDuration("LOGIN_CODE_TTL", 10*time.Minute),
		TokenEncryptionKey: envOrDefault("TOKEN_ENCRYPTION_KEY", DefaultDevTokenEncryptionKey),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}

	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
