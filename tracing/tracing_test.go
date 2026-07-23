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
	falseVal := false
	trueVal := true

	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		wantNilProv bool
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
			name: "missing endpoint is a no-op, not an error",
			config: Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Insecure:       true,
			},
			wantErr:     false,
			wantNilProv: true,
		},
		{
			name: "explicitly disabled is a no-op, even with endpoint set",
			config: Config{
				ServiceName: "test-service",
				Endpoint:    "localhost:4317",
				Enabled:     &falseVal,
			},
			wantErr:     false,
			wantNilProv: true,
		},
		{
			name: "explicitly enabled with empty endpoint is still a no-op",
			config: Config{
				ServiceName: "test-service",
				Enabled:     &trueVal,
			},
			wantErr:     false,
			wantNilProv: true,
		},
		{
			name: "enabled with valid endpoint returns a real provider",
			config: Config{
				ServiceName: "test-service",
				Endpoint:    "localhost:4317",
				Insecure:    true,
				Enabled:     &trueVal,
			},
			wantErr:     false,
			wantNilProv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prevProvider := otel.GetTracerProvider()
			prevHandler := otel.GetErrorHandler()
			prevPropagator := otel.GetTextMapPropagator()
			t.Cleanup(func() {
				otel.SetTracerProvider(prevProvider)
				otel.SetErrorHandler(prevHandler)
				otel.SetTextMapPropagator(prevPropagator)
			})
			ctx := context.Background()
			tp, err := InitTracer(ctx, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Only check provider state when no error was expected —
			// error cases always return a nil provider, which is correct
			// but not meaningfully described by wantNilProv.
			if !tt.wantErr {
				if tt.wantNilProv && tp != nil {
					t.Errorf("InitTracer() expected nil provider, got %v", tp)
				}
				if !tt.wantNilProv && tp == nil {
					t.Errorf("InitTracer() expected non-nil provider, got nil")
				}
			}
			if tp != nil {
				_ = tp.Shutdown(ctx)
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
