package config_test

import (
	"os"
	"testing"

	"github.com/pgrau/jobradar/services/embedder/internal/config"
)

func TestLoad_Success(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://litellm:4000")
	t.Setenv("LITELLM_API_KEY", "sk-test")
	t.Setenv("VALKEY_ADDR", "valkey:6379")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "alloy:4317")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LiteLLMBaseURL != "http://litellm:4000" {
		t.Errorf("expected LiteLLMBaseURL http://litellm:4000, got %s", cfg.LiteLLMBaseURL)
	}
	if cfg.LiteLLMAPIKey != "sk-test" {
		t.Errorf("expected LiteLLMAPIKey sk-test, got %s", cfg.LiteLLMAPIKey)
	}
	if cfg.ValkeyAddr != "valkey:6379" {
		t.Errorf("expected ValkeyAddr valkey:6379, got %s", cfg.ValkeyAddr)
	}
	if cfg.GRPCPort != 50051 {
		t.Errorf("expected GRPCPort 50051, got %d", cfg.GRPCPort)
	}
	if cfg.EmbedDimensions != 1024 {
		t.Errorf("expected EmbedDimensions 1024, got %d", cfg.EmbedDimensions)
	}
}

func TestLoad_MissingRequiredVars_ReturnsError(t *testing.T) {
	allKeys := []string{
		"LITELLM_BASE_URL", "LITELLM_API_KEY",
		"VALKEY_ADDR", "OTEL_EXPORTER_OTLP_ENDPOINT",
	}

	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name: "missing LITELLM_BASE_URL",
			envVars: map[string]string{
				"LITELLM_API_KEY":             "sk-test",
				"VALKEY_ADDR":                 "valkey:6379",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "alloy:4317",
			},
		},
		{
			name: "missing LITELLM_API_KEY",
			envVars: map[string]string{
				"LITELLM_BASE_URL":            "http://litellm:4000",
				"VALKEY_ADDR":                 "valkey:6379",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "alloy:4317",
			},
		},
		{
			name: "missing VALKEY_ADDR",
			envVars: map[string]string{
				"LITELLM_BASE_URL":            "http://litellm:4000",
				"LITELLM_API_KEY":             "sk-test",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "alloy:4317",
			},
		},
		{
			name: "missing OTEL_EXPORTER_OTLP_ENDPOINT",
			envVars: map[string]string{
				"LITELLM_BASE_URL": "http://litellm:4000",
				"LITELLM_API_KEY":  "sk-test",
				"VALKEY_ADDR":      "valkey:6379",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and unset all relevant env vars
			saved := make(map[string]string)
			for _, key := range allKeys {
				saved[key] = os.Getenv(key)
				os.Unsetenv(key)
			}
			// Restore after test
			t.Cleanup(func() {
				for key, val := range saved {
					if val != "" {
						os.Setenv(key, val)
					} else {
						os.Unsetenv(key)
					}
				}
			})
			// Set only the provided vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://litellm:4000")
	t.Setenv("LITELLM_API_KEY", "sk-test")
	t.Setenv("VALKEY_ADDR", "valkey:6379")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "alloy:4317")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "local" {
		t.Errorf("expected default Env=local, got %s", cfg.Env)
	}
	if cfg.GRPCPort != 50051 {
		t.Errorf("expected default GRPCPort=50051, got %d", cfg.GRPCPort)
	}
	if cfg.EmbedDimensions != 1024 {
		t.Errorf("expected default EmbedDimensions=1024, got %d", cfg.EmbedDimensions)
	}
	if cfg.EmbedCacheTTL != "24h" {
		t.Errorf("expected default EmbedCacheTTL=24h, got %s", cfg.EmbedCacheTTL)
	}
}

func TestLoad_InvalidGRPCPort_ReturnsError(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://litellm:4000")
	t.Setenv("LITELLM_API_KEY", "sk-test")
	t.Setenv("VALKEY_ADDR", "valkey:6379")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "alloy:4317")
	t.Setenv("EMBEDDER_GRPC_PORT", "99999")

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid GRPC port 99999, got nil")
	}
}

func TestLoad_InvalidEmbedDimensions_ReturnsError(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://litellm:4000")
	t.Setenv("LITELLM_API_KEY", "sk-test")
	t.Setenv("VALKEY_ADDR", "valkey:6379")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "alloy:4317")
	t.Setenv("GEMINI_EMBED_DIMENSIONS", "512")

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid embed dimensions 512, got nil")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://litellm:4000")
	t.Setenv("LITELLM_API_KEY", "sk-test")
	t.Setenv("VALKEY_ADDR", "valkey:6379")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "alloy:4317")
	t.Setenv("ENV", "hetzner")
	t.Setenv("EMBEDDER_GRPC_PORT", "50052")
	t.Setenv("VALKEY_LLM_CACHE_TTL", "12h")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "hetzner" {
		t.Errorf("expected Env=hetzner, got %s", cfg.Env)
	}
	if cfg.GRPCPort != 50052 {
		t.Errorf("expected GRPCPort=50052, got %d", cfg.GRPCPort)
	}
	if cfg.EmbedCacheTTL != "12h" {
		t.Errorf("expected EmbedCacheTTL=12h, got %s", cfg.EmbedCacheTTL)
	}
}
