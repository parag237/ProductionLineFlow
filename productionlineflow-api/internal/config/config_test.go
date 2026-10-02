package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesAPPENVAndCONFIGDIRForPreprod(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.preprod.json")
	if err := os.WriteFile(configPath, []byte(`{"env":"preprod","server":{"port":8080,"read_timeout_sec":15,"write_timeout_sec":30,"allowed_origins":["http://localhost:5173"]},"database":{"dsn":"postgres://app:app@localhost:5432/warehouse_dev?sslmode=disable","max_conns":10},"redis":{"addr":"localhost:6379","db":0},"auth":{"jwt_secret":"dev-only-secret","jwt_issuer":"productionlineflow-api","access_ttl_min":15,"refresh_ttl_hours":168,"min_password_length":10},"flows":{"session_ttl_min":60},"log":{"level":"debug","format":"text"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	oldEnv := os.Getenv("APP_ENV")
	oldDir := os.Getenv("CONFIG_DIR")
	oldPort := os.Getenv("PORT")
	oldAllowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	defer func() {
		_ = os.Setenv("APP_ENV", oldEnv)
		_ = os.Setenv("CONFIG_DIR", oldDir)
		_ = os.Setenv("PORT", oldPort)
		_ = os.Setenv("ALLOWED_ORIGINS", oldAllowedOrigins)
	}()

	if err := os.Setenv("APP_ENV", "preprod"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("CONFIG_DIR", tempDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("PORT", "10000"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("ALLOWED_ORIGINS", "https://productionlineflow.netlify.app, https://preview.example.net"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Env != "preprod" {
		t.Fatalf("env mismatch: got %q", cfg.Env)
	}
	if cfg.Server.Port != 10000 {
		t.Fatalf("port mismatch: got %d", cfg.Server.Port)
	}
	if len(cfg.Server.AllowedOrigins) != 2 || cfg.Server.AllowedOrigins[0] != "https://productionlineflow.netlify.app" || cfg.Server.AllowedOrigins[1] != "https://preview.example.net" {
		t.Fatalf("allowed origins mismatch: got %v", cfg.Server.AllowedOrigins)
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
