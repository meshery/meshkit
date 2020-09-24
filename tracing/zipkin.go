package tracing

// import (
// 	oczipkin "contrib.go.opencensus.io/exporter/zipkin"
// 	kitoc "github.com/go-kit/kit/tracing/opencensus"
// 	zipkin "github.com/openzipkin/zipkin-go"
// 	httpreporter "github.com/openzipkin/zipkin-go/reporter/http"
// 	"go.opencensus.io/trace"
// )

// type zipkin struct {
// 	url string
// 	reporter
// 	localEndpoint
// 	exporter
// }

// func NewZipkin() (Handler, error) {
// 	// Always sample our traces for this demo.
// 	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

// 	// Register our trace exporter.
// 	trace.RegisterExporter(exporter)

// 	// Wrap our service endpoints with OpenCensus tracing middleware.
// 	endpoints.Hello = kitoc.TraceEndpoint("gokit:endpoint hello")(endpoints.Hello)

// 	// Add the GO kit HTTP transport middleware to our serverOptions.
// 	serverOptions = append(serverOptions, kitoc.GRPCServerTrace())
// }
