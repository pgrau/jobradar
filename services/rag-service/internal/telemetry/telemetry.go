package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// ShutdownFunc must be called on service shutdown to flush pending telemetry.
type ShutdownFunc func(ctx context.Context) error

// Setup initialises the OTel tracer and meter providers, registers them
// as globals, and returns a single shutdown function that flushes both.
//
// All services in JobRadar call this once in main — traces and metrics
// are exported to Grafana Alloy via OTLP gRPC.
func Setup(otlpEndpoint, serviceName, env string) (ShutdownFunc, error) {
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			semconv.DeploymentEnvironmentKey.String(env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTel resource: %w", err)
	}

	tp, err := newTracerProvider(otlpEndpoint, res)
	if err != nil {
		return nil, fmt.Errorf("creating tracer provider: %w", err)
	}

	mp, err := newMeterProvider(otlpEndpoint, res)
	if err != nil {
		_ = tp.Shutdown(context.Background())
		return nil, fmt.Errorf("creating meter provider: %w", err)
	}

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		var traceErr, metricErr error
		traceErr = tp.Shutdown(ctx)
		metricErr = mp.Shutdown(ctx)
		if traceErr != nil {
			return fmt.Errorf("shutting down tracer provider: %w", traceErr)
		}
		if metricErr != nil {
			return fmt.Errorf("shutting down meter provider: %w", metricErr)
		}
		return nil
	}

	return shutdown, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// Meter returns a named meter from the global provider.
func Meter(name string) metric.Meter {
	return otel.GetMeterProvider().Meter(name)
}

// --- private ---

func newTracerProvider(endpoint string, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	), nil
}

func newMeterProvider(endpoint string, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetricgrpc.New(
		context.Background(),
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	), nil
}
