module go.opentelemetry.io/otel/example/opencensus

go 1.14

replace (
	go.opentelemetry.io/otel => ../..
	go.opentelemetry.io/otel/bridge/opencensus => ../../bridge/opencensus
	go.opentelemetry.io/otel/exporters/stdout => ../../exporters/stdout
	go.opentelemetry.io/otel/sdk => ../../sdk
)

require (
	go.opencensus.io v0.22.6-0.20201102222123-380f4078db9f
	go.opentelemetry.io/otel v1.2.0
	go.opentelemetry.io/otel/bridge/opencensus v1.2.0
	go.opentelemetry.io/otel/exporters/stdout v1.2.0
	go.opentelemetry.io/otel/sdk v1.2.0
)
