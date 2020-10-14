package vars_test

import (
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reference", func() {
	Describe("String", func() {
		for _, tt := range []struct {
			desc   string
			ref    vars.Reference
			result string
		}{
			{
				desc:   "path",
				ref:    vars.Reference{Path: "hello"},
				result: "hello",
			},
			{
				desc:   "path with fields",
				ref:    vars.Reference{Path: "hello", Fields: []string{"a", "b"}},
				result: "hello.a.b",
			},
			{
				desc:   "segments contain special chars",
				ref:    vars.Reference{Path: "hello.world", Fields: []string{"a.b", "foo:bar"}},
				result: `"hello.world"."a.b"."foo:bar"`,
			},
			{
				desc:   "segments contain special chars",
				ref:    vars.Reference{Path: "hello.world", Fields: []string{"a", "foo:bar"}},
				result: `"hello.world".a."foo:bar"`,
			},
			{
				desc:   "var source",
				ref:    vars.Reference{Source: "source", Path: "hello"},
				result: "source:hello",
			},
		} {
			tt := tt

			It(tt.desc, func() {
				Expect(tt.ref.String()).To(Equal(tt.result))
			})
		}
	})
})
