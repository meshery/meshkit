package tracing

// This file is kept for backward compatibility but the implementation
// has been moved to tracing.go with modern OpenTelemetry APIs.
// The generic InitTracer function in tracing.go supports OTLP exporters
// which are compatible with Zipkin (and other backends) via the OTLP protocol.
