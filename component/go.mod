module go.opentelemetry.io/collector/component

go 1.19

require (
	github.com/stretchr/testify v1.8.2
	go.opentelemetry.io/collector v0.75.0
	go.opentelemetry.io/collector/confmap v0.75.0
	go.opentelemetry.io/otel/metric v0.37.0
	go.opentelemetry.io/otel/trace v1.14.0
	go.uber.org/multierr v1.10.0
	go.uber.org/zap v1.24.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/featuregate v0.75.0 // indirect
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.opentelemetry.io/collector => ../

replace go.opentelemetry.io/collector/confmap => ../confmap

replace go.opentelemetry.io/collector/consumer => ../consumer

replace go.opentelemetry.io/collector/exporter => ../exporter

replace go.opentelemetry.io/collector/featuregate => ../featuregate

replace go.opentelemetry.io/collector/pdata => ../pdata

replace go.opentelemetry.io/collector/receiver => ../receiver

replace go.opentelemetry.io/collector/semconv => ../semconv

replace go.opentelemetry.io/collector/extension/zpagesextension => ../extension/zpagesextension

retract v0.69.0 // Release failed, use v0.69.1
