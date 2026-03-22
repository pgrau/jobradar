package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the embedder service.
// Values are loaded exclusively from environment variables —
// injected by Kubernetes (ConfigMap/Secret) in production,
// and by the Makefile (include .env) in local development.
type Config struct {
	// Service
	Env      string `env:"ENV"              envDefault:"local"`
	GRPCPort int    `env:"EMBEDDER_GRPC_PORT" envDefault:"50051"`

	// LiteLLM
	LiteLLMBaseURL string `env:"LITELLM_BASE_URL"`
	LiteLLMAPIKey  string `env:"LITELLM_API_KEY"`

	// Embedding model
	EmbedModel      string `env:"OLLAMA_EMBED_MODEL"      envDefault:"mxbai-embed-large"`
	EmbedDimensions int    `env:"GEMINI_EMBED_DIMENSIONS"  envDefault:"1024"`

	// Valkey
	ValkeyAddr     string `env:"VALKEY_ADDR"`
	ValkeyPassword string `env:"VALKEY_PASSWORD" envDefault:""`
	ValkeyDB       int    `env:"VALKEY_DB"       envDefault:"0"`

	// Cache TTL for embeddings
	EmbedCacheTTL string `env:"VALKEY_LLM_CACHE_TTL" envDefault:"24h"`

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
	if c.LiteLLMBaseURL == "" {
		return fmt.Errorf("LITELLM_BASE_URL is required")
	}
	if c.LiteLLMAPIKey == "" {
		return fmt.Errorf("LITELLM_API_KEY is required")
	}
	if c.ValkeyAddr == "" {
		return fmt.Errorf("VALKEY_ADDR is required")
	}
	if c.OTELEndpoint == "" {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT is required")
	}
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("EMBEDDER_GRPC_PORT must be between 1 and 65535, got %d", c.GRPCPort)
	}
	if c.EmbedDimensions != 768 && c.EmbedDimensions != 1024 && c.EmbedDimensions != 1536 {
		return fmt.Errorf("GEMINI_EMBED_DIMENSIONS must be 768, 1024 or 1536, got %d", c.EmbedDimensions)
	}
	return nil
}
