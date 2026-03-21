module github.com/pgrau/jobradar

go 1.26

require (
	// --- gRPC + Protobuf ---
	google.golang.org/grpc v1.71.0
	google.golang.org/protobuf v1.36.5

	// --- gRPC gateway (REST ↔ gRPC for api-gateway) ---
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.0

	// --- OpenTelemetry ---
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/sdk v1.35.0
	go.opentelemetry.io/otel/sdk/metric v1.35.0
	go.opentelemetry.io/otel/metric v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.35.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0

	// --- Kafka ---
	github.com/twmb/franz-go v1.18.1
	github.com/twmb/franz-go/pkg/kadm v1.14.0

	// --- Valkey / Redis ---
	github.com/valkey-io/valkey-go v1.0.54

	// --- PostgreSQL ---
	github.com/jackc/pgx/v5 v5.7.2

	// --- HTTP client (LiteLLM) ---
	github.com/openai/openai-go v0.1.0-alpha.62

	// --- MinIO (CV storage) ---
	github.com/minio/minio-go/v7 v7.0.87

	// --- JWT ---
	github.com/golang-jwt/jwt/v5 v5.2.2

	// --- Config ---
	github.com/caarlos0/env/v11 v11.3.1

	// --- PDF text extraction ---
	github.com/ledongthuc/pdf v0.0.0-20240201131950-da5b75280b06

	// --- gRPC health ---
	google.golang.org/grpc/health v1.0.0

	// --- Logging ---
	// log/slog is part of Go stdlib since 1.21 — no external dependency needed
)