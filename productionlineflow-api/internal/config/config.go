package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Env      string         `json:"env"`
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Redis    RedisConfig    `json:"redis"`
	Auth     AuthConfig     `json:"auth"`
	Flows    FlowsConfig    `json:"flows"`
	Log      LogConfig      `json:"log"`
}

type ServerConfig struct {
	Port            int      `json:"port"`
	ReadTimeoutSec  int      `json:"read_timeout_sec"`
	WriteTimeoutSec int      `json:"write_timeout_sec"`
	AllowedOrigins  []string `json:"allowed_origins"`
}

type DatabaseConfig struct {
	DSN      string `json:"dsn"`
	MaxConns int    `json:"max_conns"`
}

type RedisConfig struct {
	Addr string `json:"addr"`
	DB   int    `json:"db"`
}

type AuthConfig struct {
	JWTSecret         string `json:"jwt_secret"`
	JWTIssuer         string `json:"jwt_issuer"`
	AccessTTLMin      int    `json:"access_ttl_min"`
	RefreshTTLHours   int    `json:"refresh_ttl_hours"` // Rolling authentication idle timeout, renewed by authenticated API activity.
	MinPasswordLength int    `json:"min_password_length"`
}

type FlowsConfig struct {
	SessionTTLMin int `json:"session_ttl_min"`
}

type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

func Load() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	if env != "dev" && env != "test" && env != "preprod" && env != "prod" {
		return nil, fmt.Errorf("invalid APP_ENV %q (want dev|test|preprod|prod)", env)
	}

	dir := os.Getenv("CONFIG_DIR")
	if dir == "" {
		dir = "config"
	}

	path := filepath.Join(dir, fmt.Sprintf("config.%s.json", env))
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var c Config
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if c.Env != env {
		return nil, fmt.Errorf("config env %q does not match APP_ENV %q", c.Env, env)
	}
	if port := os.Getenv("PORT"); port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT %q: %w", port, err)
		}
		c.Server.Port = parsedPort
	}
	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		c.Database.DSN = dsn
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Validate() error {
	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be greater than zero")
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required")
	}
	if c.Auth.JWTIssuer == "" {
		return fmt.Errorf("auth.jwt_issuer is required")
	}
	if c.Auth.AccessTTLMin <= 0 {
		return fmt.Errorf("auth.access_ttl_min must be greater than zero")
	}
	if c.Auth.RefreshTTLHours <= 0 {
		return fmt.Errorf("auth.refresh_ttl_hours must be greater than zero")
	}
	if c.Auth.MinPasswordLength < 8 {
		return fmt.Errorf("auth.min_password_length must be at least 8")
	}
	if c.Flows.SessionTTLMin <= 0 {
		return fmt.Errorf("flows.session_ttl_min must be greater than zero")
	}
	return nil
}
