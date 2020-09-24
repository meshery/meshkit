module github.com/layer5io/gokit

go 1.13

replace github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f

require (
	contrib.go.opencensus.io/exporter/zipkin v0.1.2 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/kr/pretty v0.2.1 // indirect
	github.com/sirupsen/logrus v1.6.0
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)
