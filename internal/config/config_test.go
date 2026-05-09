package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_ALGORITHM", "")
	_ = os.Setenv("SERVER_HOST", "127.0.0.1")
	_ = os.Setenv("SERVER_PORT", "8080")
	_ = os.Setenv("POSTGRES_HOST", "localhost")
	_ = os.Setenv("POSTGRES_PORT", "5432")
	_ = os.Setenv("POSTGRES_USER", "postgres")
	_ = os.Setenv("POSTGRES_DB", "ticket_db")
	_ = os.Setenv("POSTGRES_PASSWORD", "postgres")
	_ = os.Setenv("POSTGRES_SSLMODE", "disable")
	_ = os.Setenv("POSTGRES_MAX_OPEN_CONNS", "10")
	_ = os.Setenv("POSTGRES_MAX_IDLE_CONNS", "5")
	_ = os.Setenv("POSTGRES_CONN_MAX_LIFETIME", "60")
	_ = os.Setenv("LOG_LEVEL", "debug")

	cfg := NewConfig()
	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 8080 {
		t.Fatalf("unexpected server config: %+v", cfg.Server)
	}
	if cfg.Postgres.Host != "localhost" || cfg.Logger.Level != "debug" {
		t.Fatalf("unexpected postgres/logger config: %+v %+v", cfg.Postgres, cfg.Logger)
	}
	if cfg.Auth.JWTSecret != "dev-only-secret" || cfg.Auth.JWTAlgorithm != "HS256" {
		t.Fatalf("unexpected auth defaults: %+v", cfg.Auth)
	}
}

func TestNewConfig_JWTFromEnvOverridesDefault(t *testing.T) {
	t.Setenv("JWT_SECRET", "prod-secret")
	t.Setenv("JWT_ALGORITHM", "HS512")

	cfg := NewConfig()
	if cfg.Auth.JWTSecret != "prod-secret" || cfg.Auth.JWTAlgorithm != "HS512" {
		t.Fatalf("expected JWT from env: %+v", cfg.Auth)
	}
}
