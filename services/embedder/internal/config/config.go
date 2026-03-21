package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the embedder service.
// Values are loaded exclusively from environment variables —
// injected by Kubernetes (ConfigMap/Secret) in production,
// and by the Makefile (include .env + .env.local) in local development.
type Config struct {
	// Service
	Env      string `env:"ENV"      envDefault:"local"`
	GRPCPort int    `env:"EMBEDDER_GRPC_PORT" envDefault:"50051"`

	// LiteLLM
	LiteLLMBaseURL string `env:"LITELLM_BASE_URL" required:"true"`
	LiteLLMAPIKey  string `env:"LITELLM_API_KEY"  required:"true"`

	// Embedding model — set per environment via .env.local / .env.hetzner
	EmbedModel      string `env:"OLLAMA_EMBED_MODEL"    envDefault:"mxbai-embed-large"`
	EmbedDimensions int    `env:"GEMINI_EMBED_DIMENSIONS" envDefault:"1024"`

	// Valkey
	ValkeyAddr     string `env:"VALKEY_ADDR"     required:"true"`
	ValkeyPassword string `env:"VALKEY_PASSWORD" envDefault:""`
	ValkeyDB       int    `env:"VALKEY_DB"       envDefault:"0"`

	// Cache TTL for embeddings (avoids redundant LiteLLM calls)
	EmbedCacheTTL string `env:"VALKEY_LLM_CACHE_TTL" envDefault:"24h"`

	// OpenTelemetry
	OTELEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" required:"true"`
}

// Load reads configuration from environment variables.
// Returns a descriptive error if any required variable is missing.
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
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("EMBEDDER_GRPC_PORT must be between 1 and 65535, got %d", c.GRPCPort)
	}

	if c.EmbedDimensions != 1024 && c.EmbedDimensions != 768 && c.EmbedDimensions != 1536 {
		return fmt.Errorf("GEMINI_EMBED_DIMENSIONS must be 768, 1024 or 1536, got %d", c.EmbedDimensions)
	}

	return nil
}
