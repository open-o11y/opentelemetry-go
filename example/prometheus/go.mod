module go.opentelemetry.io/otel/example/prometheus

go 1.14

replace (
	go.opentelemetry.io/otel => ../..
	go.opentelemetry.io/otel/exporters/metric/prometheus => ../../exporters/metric/prometheus
	go.opentelemetry.io/otel/sdk => ../../sdk
)

require (
	go.opentelemetry.io/otel v1.2.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v1.2.0
)
