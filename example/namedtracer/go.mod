module go.opentelemetry.io/otel/example/namedtracer

go 1.14

replace (
	go.opentelemetry.io/otel => ../..
	go.opentelemetry.io/otel/exporters/stdout => ../../exporters/stdout
	go.opentelemetry.io/otel/sdk => ../../sdk
)

require (
	go.opentelemetry.io/otel v1.2.0
	go.opentelemetry.io/otel/exporters/stdout v1.2.0
	go.opentelemetry.io/otel/sdk v1.2.0
)
