package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInitTracer(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				Endpoint:       "localhost:4317",
				Insecure:       true,
			},
			wantErr: false,
		},
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
		{
			name: "minimal configuration",
			config: Config{
				ServiceName: "test-service",
				Endpoint:    "localhost:4317",
				Insecure:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tp, err := InitTracer(ctx, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tp != nil {
				// Clean up
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
			res, err := newResource(tt.config)
			if err != nil {
				t.Errorf("newResource() error = %v", err)
				return
			}
			if res == nil {
				t.Error("newResource() returned nil resource")
			}
		})
	}
}

func TestHTTPMiddleware(t *testing.T) {
	// Set up a test tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(tracetest.NewSpanRecorder()),
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
	// Set up a test tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(tracetest.NewSpanRecorder()),
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
