package tracing_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/concourse/concourse/atc/tracing"
)

func noTracing() {
	doSomething()
}

func noop(ctx context.Context) {
	tracing.StartSpan(ctx, "component", nil)
	doSomething()
}

func noopWithAttrs(ctx context.Context) {
	tracing.StartSpan(ctx, "component", tracing.Attrs{"a": "b"})
	doSomething()
}

func doSomething() {
	fmt.Fprintf(ioutil.Discard, "aa")
}

func BenchmarkNoTracing(b *testing.B) {
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		noTracing()
	}
}

func BenchmarkNoopTracerWithAttrs(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()

	for n := 0; n < b.N; n++ {
		noopWithAttrs(ctx)
	}
}

func BenchmarkNoopTracer(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()

	for n := 0; n < b.N; n++ {
		noop(ctx)
	}
}
