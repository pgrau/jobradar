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

	ragv1 "github.com/pgrau/jobradar/proto/rag/v1"
	"github.com/pgrau/jobradar/services/rag-service/internal/config"
	"github.com/pgrau/jobradar/services/rag-service/internal/db"
	"github.com/pgrau/jobradar/services/rag-service/internal/handler"
	"github.com/pgrau/jobradar/services/rag-service/internal/telemetry"

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
		logger.Error("rag-service exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("rag-service starting",
		"env", cfg.Env,
		"grpc_port", cfg.GRPCPort,
	)

	// --- OTel (traces + metrics) ---
	shutdown, err := telemetry.Setup(cfg.OTELEndpoint, "rag-service", cfg.Env)
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

	postgres, err := db.NewPostgres(startCtx, cfg.PostgresDSN, logger)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer postgres.Close()

	// --- gRPC server ---
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", cfg.GRPCPort, err)
	}

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	healthSrv := health.NewServer()
	ragv1.RegisterRAGServiceServer(srv, handler.NewRAGHandler(postgres, logger))
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	reflection.Register(srv)

	healthSrv.SetServingStatus("rag-service", grpc_health_v1.HealthCheckResponse_SERVING)

	// --- Graceful shutdown ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("rag-service ready", "port", cfg.GRPCPort)
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

	healthSrv.SetServingStatus("rag-service", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.Info("rag-service stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("shutdown timeout exceeded, forcing stop")
		srv.Stop()
	}

	return errors.Join(<-serverErr)
}
