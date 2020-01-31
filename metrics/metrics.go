// Package metrics is responsible for providing support metric instrumentation.
//
// It's meant to be used alongside the codebase, wherever capturing a
// measurement makes sense.
//
// For instance:
//
// 	func doSomething () {
// 	        metrics.
// 	        	Counter("actions").
// 	        	Add(context.Background(), int64(1), nil)
// 	}
//
// would increment a counter to be scraped, in the case of a pull-based exporter
// (like, Prometheus), and emit a measurement in the case of a push-based one.
//
//
package metrics

import (
	"go.opentelemetry.io/otel/api/metric"
)

const meterName = "concourse"

var (
	// Meter is responsible for taking care of measurements of different
	// measurements.
	//
	Meter    metric.Meter = metric.NoopMeter{}
	counters              = map[string]metric.Int64Counter{}
)

// Counter instantiates (if needed) a counter with the name `name`.
//
func Counter(name string) *metric.Int64Counter {
	var found bool

	counter, found := counters[name]
	if !found {
		counter = Meter.NewInt64Counter(name)
		counters[name] = counter
	}

	return &counter
}
