package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Config holds the configuration parameters for tracing
type Config struct {
	// ServiceName is the name of the service being traced
	ServiceName string
	// ServiceVersion is the version of the service
	ServiceVersion string
	// Environment is the deployment environment (e.g., "production", "staging", "development")
	Environment string
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317")
	Endpoint string
	// Insecure determines whether to use an insecure connection (no TLS)
	Insecure bool
}

// InitTracer initializes and configures the global OpenTelemetry trace provider
// It sets up OTLP gRPC exporter, resource attributes, and W3C trace context propagation
func InitTracer(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	// Validate configuration
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	// Configure OTLP exporter options
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	// Create OTLP gRPC exporter
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service identification
	res, err := newResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator for W3C trace context
	// This is critical for distributed tracing across service boundaries
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// newResource creates a resource with service identification attributes
func newResource(cfg Config) (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	}

	// Add optional service version
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, resource.WithAttributes(
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		))
	}

	// Add optional deployment environment
	if cfg.Environment != "" {
		attrs = append(attrs, resource.WithAttributes(
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		))
	}

	return resource.New(
		context.Background(),
		attrs...,
	)
}
