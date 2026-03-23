package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the rag-service.
// Values are loaded exclusively from environment variables —
// injected by Kubernetes (ConfigMap/Secret) in production,
// and by the Makefile (include .env) in local development.
type Config struct {
	// Service
	Env      string `env:"ENV"              envDefault:"local"`
	GRPCPort int    `env:"RAG_GRPC_PORT"    envDefault:"50052"`

	// PostgreSQL
	PostgresDSN string `env:"POSTGRES_DSN"`

	// OpenTelemetry
	OTELEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing environment variables: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.PostgresDSN == "" {
		return fmt.Errorf("POSTGRES_DSN is required")
	}
	if c.OTELEndpoint == "" {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT is required")
	}
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("RAG_GRPC_PORT must be between 1 and 65535, got %d", c.GRPCPort)
	}
	return nil
}
