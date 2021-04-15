package tracing

import (
	"go.opentelemetry.io/otel/attribute"
)

type Attrs map[string]string

// keyValueSlice converts our internal representation of kv pairs to the tracing
// SDK's kv representation.
//
func keyValueSlice(attrs Attrs) []attribute.KeyValue {
	var (
		res = make([]attribute.KeyValue, len(attrs))
		idx = 0
	)

	for k, v := range attrs {
		res[idx] = attribute.String(k, v)
		idx++
	}

	return res
}
