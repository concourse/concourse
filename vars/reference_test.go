package vars_test

import (
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo/v2"
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
				ref:    vars.Reference{Path: "hello.world", Fields: []string{"a", "foo:bar", "other field", "another/field"}},
				result: `"hello.world".a."foo:bar"."other field"."another/field"`,
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

	Describe("ParseReference", func() {
		for _, tt := range []struct {
			desc string
			raw  string
			ref  vars.Reference
			err  string
		}{
			{
				desc: "path",
				raw:  "hello",
				ref:  vars.Reference{Path: "hello", Fields: []string{}},
			},
			{
				desc: "path with fields",
				raw:  "hello.a.b",
				ref:  vars.Reference{Path: "hello", Fields: []string{"a", "b"}},
			},
			{
				desc: "segments contain special chars",
				raw:  `"hello.world"."a.b"."foo:bar"`,
				ref:  vars.Reference{Path: "hello.world", Fields: []string{"a.b", "foo:bar"}},
			},
			{
				desc: "segments contain special chars",
				raw:  `"hello.world".a."foo:bar"`,
				ref:  vars.Reference{Path: "hello.world", Fields: []string{"a", "foo:bar"}},
			},
			{
				desc: "var source",
				raw:  "source:hello",
				ref:  vars.Reference{Source: "source", Path: "hello", Fields: []string{}},
			},
			{
				desc: "path with colon and no var source",
				raw:  `"my:path"."field.1"."field.2"`,
				ref:  vars.Reference{Path: "my:path", Fields: []string{"field.1", "field.2"}},
			},
			{
				desc: "quoted var source",
				raw:  `"some-source":path`,
				err:  `invalid var '"some-source":path': source must not be quoted`,
			},
			{
				desc: "empty path segment",
				raw:  `vault:.field`,
				err:  `invalid var 'vault:.field': empty field`,
			},
			{
				desc: "empty quoted path segment",
				raw:  `vault:"".field`,
				err:  `invalid var 'vault:"".field': empty field`,
			},
			{
				desc: "no path segments",
				raw:  `vault:`,
				err:  `invalid var 'vault:': empty field`,
			},
			{
				desc: "trims spaces in path segments",
				raw:  `hello .world `,
				ref:  vars.Reference{Path: "hello", Fields: []string{"world"}},
			},
			{
				desc: "does not trim spaces in quoted segments",
				raw:  `" hello "."world "`,
				ref:  vars.Reference{Path: " hello ", Fields: []string{"world "}},
			},
		} {
			tt := tt

			It(tt.desc, func() {
				ref, err := vars.ParseReference(tt.raw)
				if tt.err == "" {
					Expect(err).ToNot(HaveOccurred())
					Expect(ref).To(Equal(tt.ref))
				} else {
					Expect(err).To(MatchError(tt.err))
				}
			})
		}
	})
})
