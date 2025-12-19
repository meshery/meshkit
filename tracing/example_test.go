package tracing_test

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/meshery/meshkit/tracing"
)

// ExampleInitTracer demonstrates how to initialize OpenTelemetry tracing
func ExampleInitTracer() {
	ctx := context.Background()

	// Configure tracer
	cfg := tracing.Config{
		ServiceName:    "meshery-server",
		ServiceVersion: "v0.6.0",
		Environment:    "development",
		Endpoint:       "localhost:4317",
		Insecure:       true, // Use true for local development, false for production
	}

	// Initialize tracer
	tp, err := tracing.InitTracer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}

	// Ensure proper shutdown
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	fmt.Println("Tracer initialized successfully")
	// Output: Tracer initialized successfully
}

// ExampleHTTPMiddleware demonstrates how to use HTTP middleware for tracing
func ExampleHTTPMiddleware() {
	// Your existing HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	// Wrap with tracing middleware
	tracedHandler := tracing.HTTPMiddleware(handler, "my-api")

	// Use the wrapped handler with your server
	_ = tracedHandler // In practice, use with http.ListenAndServe or your router
}

// ExampleNewTransport demonstrates how to use instrumented HTTP client
func ExampleNewTransport() {
	// Create HTTP client with tracing
	client := &http.Client{
		Transport: tracing.NewTransport(nil),
	}

	// Use the client as normal - trace context will be automatically propagated
	_, _ = client.Get("http://example.com")
}
