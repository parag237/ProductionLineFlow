package main

import (
	"context"
	"log/slog"
	"testing"

	"productionlineflow-api/internal/config"
)

func TestNewLoggerUsesConfiguredLevel(t *testing.T) {
	logger, err := newLogger(config.LogConfig{Level: "warn", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	if logger.Handler().Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected info logs to be disabled")
	}
	if !logger.Handler().Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("expected warning logs to be enabled")
	}
}

func TestNewLoggerRejectsUnsupportedSettings(t *testing.T) {
	tests := []struct {
		name   string
		config config.LogConfig
	}{
		{name: "level", config: config.LogConfig{Level: "verbose", Format: "json"}},
		{name: "format", config: config.LogConfig{Level: "info", Format: "yaml"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newLogger(test.config); err == nil {
				t.Fatal("expected unsupported logger settings to fail")
			}
		})
	}
}