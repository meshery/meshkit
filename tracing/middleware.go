package tracing

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPMiddleware wraps an HTTP handler with OpenTelemetry tracing
// It automatically creates spans for incoming HTTP requests and propagates trace context
func HTTPMiddleware(handler http.Handler, serviceName string) http.Handler {
	return otelhttp.NewHandler(handler, serviceName)
}

// HTTPHandler wraps an HTTP handler function with OpenTelemetry tracing
// It automatically creates spans for incoming HTTP requests and propagates trace context
func HTTPHandler(handler http.HandlerFunc, operation string) http.HandlerFunc {
	return otelhttp.NewHandler(handler, operation).ServeHTTP
}

// NewTransport creates an HTTP transport instrumented with OpenTelemetry
// Use this when making HTTP client requests to propagate trace context
func NewTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}
