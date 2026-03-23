package config_test

import (
	"testing"

	"github.com/pgrau/jobradar/services/rag-service/internal/config"
)

func TestLoad_RequiredFields(t *testing.T) {
	base := map[string]string{
		"POSTGRES_DSN":                   "postgres://user:pass@localhost:5432/jobradar",
		"OTEL_EXPORTER_OTLP_ENDPOINT":    "localhost:4317",
	}

	tests := []struct {
		name    string
		envVars map[string]string
		wantErr string
	}{
		{
			name:    "missing POSTGRES_DSN",
			envVars: without(base, "POSTGRES_DSN"),
			wantErr: "POSTGRES_DSN",
		},
		{
			name:    "missing OTEL_EXPORTER_OTLP_ENDPOINT",
			envVars: without(base, "OTEL_EXPORTER_OTLP_ENDPOINT"),
			wantErr: "OTEL_EXPORTER_OTLP_ENDPOINT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != "" {
				if got := err.Error(); !contains(got, tt.wantErr) {
					t.Errorf("expected error to contain %q, got %q", tt.wantErr, got)
				}
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/jobradar")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GRPCPort != 50052 {
		t.Errorf("expected default GRPCPort 50052, got %d", cfg.GRPCPort)
	}
	if cfg.Env != "local" {
		t.Errorf("expected default Env 'local', got %q", cfg.Env)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/jobradar")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("RAG_GRPC_PORT", "99999")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid port, got nil")
	}
	if !contains(err.Error(), "RAG_GRPC_PORT") {
		t.Errorf("expected error to mention RAG_GRPC_PORT, got %q", err.Error())
	}
}

// --- helpers ---

func without(m map[string]string, key string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
