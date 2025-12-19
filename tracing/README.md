# OpenTelemetry Tracing Package

This package provides OpenTelemetry tracing instrumentation for Meshery components.

## Features

- **Easy Initialization**: Simple setup with the `InitTracer` function
- **Standardized Tags**: Consistent service identification with `service.name`, `service.version`, and `environment`
- **Context Propagation**: Automatic W3C Trace Context header propagation across service boundaries
- **HTTP Middleware**: Ready-to-use middleware for tracing HTTP requests
- **Client Instrumentation**: HTTP client transport wrapper for outgoing requests

## Quick Start

### Initialize Tracing

```go
import (
    "context"
    "github.com/meshery/meshkit/tracing"
)

func main() {
    ctx := context.Background()

    cfg := tracing.Config{
        ServiceName:    "meshery-server",
        ServiceVersion: "v0.6.0",
        Environment:    "production",
        Endpoint:       "localhost:4317",  // OTLP collector endpoint
        Insecure:       false,             // Use false in production with TLS
    }

    tp, err := tracing.InitTracer(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to initialize tracer: %v", err)
    }
    defer tp.Shutdown(ctx)
}
```

### HTTP Server Middleware

Wrap your HTTP handler to automatically create spans for incoming requests:

```go
import (
    "net/http"
    "github.com/meshery/meshkit/tracing"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api", apiHandler)

    // Wrap with tracing middleware
    tracedHandler := tracing.HTTPMiddleware(mux, "meshery-api")

    http.ListenAndServe(":8080", tracedHandler)
}
```

### HTTP Client Instrumentation

Instrument HTTP clients to propagate trace context to downstream services:

```go
import (
    "net/http"
    "github.com/meshery/meshkit/tracing"
)

func makeRequest() {
    client := &http.Client{
        Transport: tracing.NewTransport(nil),
    }

    resp, err := client.Get("http://remote-service/api")
    // Trace context is automatically propagated
}
```

## Configuration

### Config Fields

- **ServiceName** (required): Name identifying your service
- **ServiceVersion** (optional): Version of your service
- **Environment** (optional): Deployment environment (e.g., "production", "staging", "development")
- **Endpoint** (required): OTLP collector endpoint (e.g., "localhost:4317")
- **Insecure** (optional, defaults to false): Set to true for non-TLS connections (development only). When false (default), TLS is used for secure connections.

## Integration Examples

### Meshery Server

```go
// In server/main.go
func main() {
    ctx := context.Background()
    
    cfg := tracing.Config{
        ServiceName:    "meshery-server",
        ServiceVersion: version.GetVersion(),
        Environment:    os.Getenv("ENVIRONMENT"),
        Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
        Insecure:       os.Getenv("ENVIRONMENT") == "development",
    }
    
    tp, err := tracing.InitTracer(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to initialize tracer: %v", err)
    }
    defer tp.Shutdown(ctx)
    
    // Wrap router with tracing
    router := setupRouter()
    tracedRouter := tracing.HTTPMiddleware(router, "meshery-server")
    
    http.ListenAndServe(":9081", tracedRouter)
}
```

### Remote Provider

```go
// In remote provider main.go
func main() {
    ctx := context.Background()
    
    cfg := tracing.Config{
        ServiceName:    "meshery-cloud",
        ServiceVersion: version,
        Environment:    env,
        Endpoint:       otelEndpoint,
        Insecure:       isLocalDev,
    }
    
    tp, err := tracing.InitTracer(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to initialize tracer: %v", err)
    }
    defer tp.Shutdown(ctx)
    
    // For Echo framework
    e := echo.New()
    
    // Wrap HTTP client for calls to Meshery Server
    client := &http.Client{
        Transport: tracing.NewTransport(nil),
    }
}
```

## W3C Trace Context Propagation

The package automatically configures W3C Trace Context propagation. This ensures that:

1. Incoming requests with `traceparent` headers are properly linked to parent traces
2. Outgoing requests include `traceparent` headers for distributed tracing
3. Trace context flows seamlessly between services

## OTLP Collector Setup

The tracing package requires an OpenTelemetry Collector endpoint. Common setups:

### Local Development (Docker)

```bash
docker run -d --name jaeger \
  -p 4317:4317 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

### Production

Configure your OTLP endpoint in environment variables:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=collector.example.com:4317
export ENVIRONMENT=production
```

## Viewing Traces

After setting up the collector, traces can be viewed in:

- Jaeger UI: http://localhost:16686
- Or your configured tracing backend (Zipkin, Tempo, etc.)

## Best Practices

1. **Always call `tp.Shutdown(ctx)`** on application exit to flush remaining spans
2. **Use meaningful operation names** in middleware to identify endpoints
3. **Set environment** to distinguish traces across deployments
4. **Use TLS in production** by setting `Insecure: false`
5. **Instrument HTTP clients** that make calls to other services for full distributed tracing

## Package Structure

```
tracing/
├── tracing.go        # Core initialization and configuration
├── middleware.go     # HTTP middleware and client instrumentation
├── tracing_test.go   # Unit tests
├── example_test.go   # Example usage
├── jaeger.go         # Backward compatibility note
├── zipkin.go         # Backward compatibility note
└── README.md         # This file
```
