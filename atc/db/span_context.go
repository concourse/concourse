package db

import (
	"context"

	"github.com/concourse/concourse/tracing"
)

type SpanContext map[string]string

func NewSpanContext(ctx context.Context) SpanContext {
	sc := SpanContext{}
	tracing.Inject(ctx, sc)
	return sc
}

func (sc SpanContext) Get(key string) string {
	if sc == nil {
		return ""
	}
	return sc[key]
}

func (sc SpanContext) Set(key, value string) {
	if sc != nil {
		sc[key] = value
	}
}
