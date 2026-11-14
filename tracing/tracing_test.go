package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func TestInitTracer(t *testing.T) {
	// Note: These tests validate configuration without attempting real connections
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "missing service name",
			config: Config{
				ServiceVersion: "1.0.0",
				Endpoint:       "localhost:4317",
				Insecure:       true,
			},
			wantErr: true,
		},
		{
			name: "missing endpoint",
			config: Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Insecure:       true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := InitTracer(ctx, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewResource(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "full configuration",
			config: Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "production",
			},
		},
		{
			name: "minimal configuration",
			config: Config{
				ServiceName: "test-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			res, err := newResource(ctx, tt.config)
			if err != nil {
				t.Errorf("newResource() error = %v", err)
				return
			}
			if res == nil {
				t.Error("newResource() returned nil resource")
				return
			}

			// Verify service name is set correctly
			attrs := res.Attributes()
			var foundServiceName bool
			for _, attr := range attrs {
				if attr.Key == semconv.ServiceNameKey {
					if attr.Value.AsString() != tt.config.ServiceName {
						t.Errorf("Expected service name %s, got %s", tt.config.ServiceName, attr.Value.AsString())
					}
					foundServiceName = true
					break
				}
			}
			if !foundServiceName {
				t.Error("Service name attribute not found in resource")
			}
		})
	}
}

func TestTracerProviderWithMockExporter(t *testing.T) {
	// Test that we can create a tracer provider with a mock exporter
	exporter := tracetest.NewInMemoryExporter()

	ctx := context.Background()
	res, err := newResource(ctx, Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
	})
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(res),
	)
	defer func() {
		_ = tp.Shutdown(ctx)
	}()

	// Set as global provider
	otel.SetTracerProvider(tp)

	// Verify we can get a tracer
	tracer := otel.Tracer("test")
	if tracer == nil {
		t.Error("Failed to get tracer from provider")
	}

	// Create a test span
	_, span := tracer.Start(ctx, "test-operation")
	span.End()

	// Verify span was recorded
	if len(exporter.GetSpans()) != 1 {
		t.Errorf("Expected 1 span, got %d", len(exporter.GetSpans()))
	}
}

func TestHTTPMiddleware(t *testing.T) {
	// Set up a test tracer provider with in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with middleware
	wrapped := HTTPMiddleware(handler, "test-operation")

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrapped.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got %v", rec.Body.String())
	}
}

func TestHTTPHandler(t *testing.T) {
	// Set up a test tracer provider with in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Create a handler function
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Handler OK"))
	}

	// Wrap with HTTPHandler
	wrapped := HTTPHandler(handler, "test-handler")

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrapped(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rec.Code)
	}
	if rec.Body.String() != "Handler OK" {
		t.Errorf("expected body 'Handler OK', got %v", rec.Body.String())
	}
}

func TestNewTransport(t *testing.T) {
	// Test with nil base transport
	transport := NewTransport(nil)
	if transport == nil {
		t.Error("NewTransport(nil) returned nil")
	}

	// Test with custom base transport
	customTransport := &http.Transport{}
	transport = NewTransport(customTransport)
	if transport == nil {
		t.Error("NewTransport(customTransport) returned nil")
	}
}
func TestInitTracerDisabled(t *testing.T) {
	ctx := context.Background()
	enabled := false

	cfg := Config{
		Enabled: &enabled,
	}

	tp, err := InitTracer(ctx, cfg)
	if err != nil {
		t.Fatalf("expected no error when tracing is disabled, got %v", err)
	}

	if tp != nil {
		t.Fatalf("expected nil tracer provider when tracing is disabled")
	}
}
