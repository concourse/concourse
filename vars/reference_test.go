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
				desc: "local var source",
				raw:  ".:hello",
				ref:  vars.Reference{Source: ".", Path: "hello", Fields: []string{}},
			},
			{
				desc: "path with colon and no var source",
				raw:  `"my:path"."field.1"."field.2"`,
				ref:  vars.Reference{Path: "my:path", Fields: []string{"field.1", "field.2"}},
			},
			{
				desc: "quoted var source",
				raw:  `"some-source":path`,
				ref:  vars.Reference{Source: "some-source", Path: "path", Fields: []string{}},
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
			{
				desc: "parses quoted source",
				raw:  `"src":foo`,
				ref:  vars.Reference{Source: "src", Path: "foo", Fields: []string{}},
			},
			{
				desc: "errors on stray double quotes in source",
				raw:  `"src:foo`,
				err:  `invalid var '"src:foo': contains unclosed quotes`,
			},
			{
				desc: "errors on stray double quotes in path",
				raw:  `src:"foo`,
				err:  `invalid var 'src:"foo': contains unclosed quotes`,
			},
			{
				desc: "errors on stray double quotes in field",
				raw:  `src:foo."bar.baz`,
				err:  `invalid var 'src:foo."bar.baz': contains unclosed quotes`,
			},
			{
				desc: "path with slashes",
				raw:  `foo/bar/baz`,
				ref:  vars.Reference{Path: "foo/bar/baz", Fields: []string{}},
			},
			{
				desc: "path with slashes and field",
				raw:  `foo/bar/baz.yak`,
				ref:  vars.Reference{Path: "foo/bar/baz", Fields: []string{"yak"}},
			},
			{
				desc: "path with slashes and field with slash",
				raw:  `foo/bar.yak/baz`,
				ref:  vars.Reference{Path: "foo/bar", Fields: []string{"yak/baz"}},
			},
			{
				desc: "retains leading slash in path",
				raw:  `/foo/bar/baz`,
				ref:  vars.Reference{Path: "/foo/bar/baz", Fields: []string{}},
			},
			{
				desc: "removes leading path traversal",
				raw:  `".."/foo/bar/baz`,
				ref:  vars.Reference{Path: "foo/bar/baz", Fields: []string{}},
			},
			{
				desc: "errors if it's only path traversal elements",
				raw:  `..`,
				err:  `invalid var '..': empty field`,
			},
			{
				desc: "errors if it's only quoted path traversal elements",
				raw:  `".."`,
				err:  `invalid var '".."': empty field`,
			},
			{
				desc: "errors if it's only two path traversal elements",
				raw:  `../..`,
				err:  `invalid var '../..': empty field`,
			},
			{
				desc: "errors if it's only two quoted path traversal elements",
				raw:  `"../.."`,
				err:  `invalid var '"../.."': empty field`,
			},
			{
				desc: "removes multiple trailing path traversal",
				raw:  `"/foo/bar/baz/../.."`,
				ref:  vars.Reference{Path: "/foo", Fields: []string{}},
			},
			{
				desc: "removes multiple leading path traversal",
				raw:  `"../../.."/foo/bar/baz`,
				ref:  vars.Reference{Path: "foo/bar/baz", Fields: []string{}},
			},
			{
				desc: "resolves inner path traversal elements",
				raw:  `/foo/bar/".."/baz`,
				ref:  vars.Reference{Path: "/foo/baz", Fields: []string{}},
			},
			{
				desc: "resolves all path traversal elements",
				raw:  `"../""../""../.."/foo/bar/".."/"..""/.."/"../"/baz`,
				ref:  vars.Reference{Path: "baz", Fields: []string{}},
			},
			{
				desc: "resolves path traversal elements in fully quoted path",
				raw:  `"../../../../../../foo/bar/baz"`,
				ref:  vars.Reference{Path: "foo/bar/baz", Fields: []string{}},
			},
			{
				desc: "fully quoted reference is treated as path",
				raw:  `"src:foo.bar"`,
				ref:  vars.Reference{Path: "src:foo.bar", Fields: []string{}},
			},
			{
				desc: "source has whitespace removed",
				raw:  `"  src  ":foo`,
				ref:  vars.Reference{Source: "src", Path: "foo", Fields: []string{}},
			},
			{
				desc: "errors on unquoted leading path traversal",
				raw:  `../foo`,
				err:  `invalid var '../foo': empty field`,
			},
		} {

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
