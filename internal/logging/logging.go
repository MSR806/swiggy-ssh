package logging

import (
	"log/slog"
	"os"
)

// New returns a structured logger configured for local development.
func New(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if appEnv == "local" {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
