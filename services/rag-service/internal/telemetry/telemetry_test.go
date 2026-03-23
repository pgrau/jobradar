package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/pgrau/jobradar/services/rag-service/internal/telemetry"
)

func TestSetup_ReturnsShutdownFunc(t *testing.T) {
	// Even with an unreachable endpoint, Setup should succeed —
	// OTel exporters connect lazily, not at initialization time.
	shutdown, err := telemetry.Setup("localhost:4317", "rag-service-test", "test")
	if err != nil {
		t.Fatalf("expected Setup to succeed with unreachable endpoint, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = shutdown(ctx)
}

func TestSetup_DifferentEnvironments(t *testing.T) {
	envs := []string{"local", "hetzner", "test"}

	for _, env := range envs {
		t.Run(env, func(t *testing.T) {
			shutdown, err := telemetry.Setup("localhost:4317", "rag-service-test", env)
			if err != nil {
				t.Fatalf("Setup failed for env=%s: %v", env, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = shutdown(ctx)
		})
	}
}
