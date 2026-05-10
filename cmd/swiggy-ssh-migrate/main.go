package main

import (
	"context"
	"log"
	"os"
	"strings"

	store "swiggy-ssh/internal/infrastructure/persistence/postgres"
	"swiggy-ssh/internal/platform/config"
)

func main() {
	cfg := config.Load()
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	var err error
	switch command {
	case "up":
		err = store.MigrateUp(context.Background(), cfg.DatabaseURL)
	case "down":
		err = store.MigrateDown(context.Background(), cfg.DatabaseURL)
	case "drop":
		if !allowDrop(cfg.AppEnv) {
			log.Fatalf("refusing drop in APP_ENV=%q (allowed: local, development, test)", cfg.AppEnv)
		}
		err = store.MigrateDrop(context.Background(), cfg.DatabaseURL)
	default:
		log.Fatalf("unknown command %q (supported: up, down, drop)", command)
	}

	if err != nil {
		log.Fatalf("migrations %s failed: %v", command, err)
	}

	log.Printf("migrations %s complete", command)
}

func allowDrop(appEnv string) bool {
	if appEnv == "" {
		return true
	}

	switch strings.ToLower(appEnv) {
	case "local", "development", "test":
		return true
	default:
		return false
	}
}
