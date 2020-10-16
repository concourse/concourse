package tracing

import (
	"go.opentelemetry.io/otel/label"
)

type Attrs map[string]string

// keyValueSlice converts our internal representation of kv pairs to the tracing
// SDK's kv representation.
//
func keyValueSlice(attrs Attrs) []label.KeyValue {
	var (
		res = make([]label.KeyValue, len(attrs))
		idx = 0
	)

	for k, v := range attrs {
		res[idx] = label.String(k, v)
		idx++
	}

	return res
}
