package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesAPPENVAndCONFIGDIR(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.dev.json")
	if err := os.WriteFile(configPath, []byte(`{"env":"dev","server":{"port":8080,"read_timeout_sec":15,"write_timeout_sec":30,"allowed_origins":["http://localhost:5173"]},"database":{"dsn":"postgres://app:app@localhost:5432/warehouse_dev?sslmode=disable","max_conns":10},"redis":{"addr":"localhost:6379","db":0},"auth":{"jwt_secret":"dev-only-secret","jwt_issuer":"productionlineflow-api","access_ttl_min":15,"refresh_ttl_hours":168,"min_password_length":10},"flows":{"session_ttl_min":60},"log":{"level":"debug","format":"text"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	oldEnv := os.Getenv("APP_ENV")
	oldDir := os.Getenv("CONFIG_DIR")
	defer func() {
		_ = os.Setenv("APP_ENV", oldEnv)
		_ = os.Setenv("CONFIG_DIR", oldDir)
	}()

	if err := os.Setenv("APP_ENV", "dev"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("CONFIG_DIR", tempDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Env != "dev" {
		t.Fatalf("env mismatch: got %q", cfg.Env)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("port mismatch: got %d", cfg.Server.Port)
	}
}

func TestLoadRejectsInvalidAPPENV(t *testing.T) {
	oldEnv := os.Getenv("APP_ENV")
	defer func() { _ = os.Setenv("APP_ENV", oldEnv) }()
	if err := os.Setenv("APP_ENV", "qa"); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid APP_ENV to fail")
	}
}
