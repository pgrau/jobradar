---
name: scaffold-grpc-main
description: Generates the cmd/main.go for a JobRadar gRPC service with correct graceful shutdown pattern. Invoke when creating a new gRPC service. Argument: service name (e.g. rag-service).
user-invocable: false
---

Create `services/$ARGUMENTS/cmd/main.go` following this exact pattern.
Replace `$ARGUMENTS` with the service name, `$PROTO_PACKAGE` with the proto import alias, `$REGISTER_FN` with the proto registration function, `$HANDLER` with the handler constructor, `$PORT_ENV` with the port config field, and `$GRPC_PORT` with the default port number.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	// TODO: add proto import
	// TODO: add internal package imports
	"github.com/pgrau/jobradar/services/$ARGUMENTS/internal/config"
	"github.com/pgrau/jobradar/services/$ARGUMENTS/internal/handler"
	"github.com/pgrau/jobradar/services/$ARGUMENTS/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	shutdownTimeout = 15 * time.Second
	startupTimeout  = 30 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(logger); err != nil {
		logger.Error("$ARGUMENTS exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("$ARGUMENTS starting",
		"env", cfg.Env,
		"grpc_port", cfg.GRPCPort,
	)

	// --- OTel (traces + metrics) ---
	shutdown, err := telemetry.Setup(cfg.OTELEndpoint, "$ARGUMENTS", cfg.Env)
	if err != nil {
		return fmt.Errorf("setting up telemetry: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			logger.Error("telemetry shutdown error", "error", err)
		}
	}()

	// --- Dependencies with startup timeout ---
	startCtx, startCancel := context.WithTimeout(context.Background(), startupTimeout)
	defer startCancel()

	// TODO: initialise dependencies (db, cache, clients) using startCtx
	_ = startCtx

	// --- gRPC server ---
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", cfg.GRPCPort, err)
	}

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	healthSrv := health.NewServer()
	// TODO: register proto server
	// protoPackage.RegisterXxxServiceServer(srv, handler.NewXxxHandler(..., logger))
	_ = handler.New$ARGUMENTSHandler // silence unused import until wired
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	reflection.Register(srv)

	healthSrv.SetServingStatus("$ARGUMENTS", grpc_health_v1.HealthCheckResponse_SERVING)

	// --- Graceful shutdown ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("$ARGUMENTS ready", "port", cfg.GRPCPort)
		if err := srv.Serve(lis); err != nil {
			serverErr <- fmt.Errorf("grpc serve: %w", err)
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	healthSrv.SetServingStatus("$ARGUMENTS", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.Info("$ARGUMENTS stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("shutdown timeout exceeded, forcing stop")
		srv.Stop()
	}

	return errors.Join(<-serverErr)
}
```

After generating the file, fill in the TODO comments with the actual proto registration and dependency wiring specific to the service being implemented.
