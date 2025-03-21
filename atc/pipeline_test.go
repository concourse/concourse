package atc_test

import (
	"net/url"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineRef", func() {
	Describe("String", func() {
		for _, tt := range []struct {
			desc string
			ref  atc.PipelineRef
			out  string
		}{
			{
				desc: "simple",
				ref:  atc.PipelineRef{Name: "some-pipeline"},
				out:  "some-pipeline",
			},
			{
				desc: "with instance vars",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"field.1": map[string]any{
						"subfield:1": 1,
						"subfield 2": []any{"1", 2, map[string]any{"k": "v"}},
					},
					"other": "field",
				}},
				out: `some-pipeline/"field.1"."subfield 2":["1",2,{"k":"v"}],"field.1"."subfield:1":1,other:field`,
			},
			{
				desc: "instance vars sorted alphabetically",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"b": map[string]any{
						"foo": 1,
						"bar": []any{"1", 2},
					},
					"a": "hello.world",
				}},
				out: `some-pipeline/a:hello.world,b.bar:["1",2],b.foo:1`,
			},
			{
				desc: "quotes string values that contain special characters",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"colon": "a:b",
					"comma": "a,b",
					"space": "a b",
					"slash": "a/b",
				}},
				out: `some-pipeline/colon:"a:b",comma:"a,b",slash:"a/b",space:"a b"`,
			},
			{
				desc: "quotes string values that match special YAML values",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"int":        "123",
					"float":      "4e+6",
					"bool":       "true",
					"weird_bool": "yes",
					"empty":      "",
				}},
				out: `some-pipeline/bool:"true",empty:"",float:"4e+6",int:"123",weird_bool:"yes"`,
			},
			{
				desc: "doesn't quote non-string primitives",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"int":   123,
					"float": 123.456,
					"bool":  true,
					"nil":   nil,
				}},
				out: `some-pipeline/bool:true,float:123.456,int:123,nil:null`,
			},
			{
				desc: "empty instance vars",
				ref:  atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{}},
				out:  "some-pipeline",
			},
			{
				desc: "nil instance vars",
				ref:  atc.PipelineRef{Name: "some-pipeline", InstanceVars: nil},
				out:  "some-pipeline",
			},
			{
				desc: "extremely large numbers",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"large_int":   9223372036854775807,     // max int64
					"large_float": 1.7976931348623157e+308, // max float64
				}},
				out: `some-pipeline/large_float:1.7976931348623157e+308,large_int:9223372036854775807`,
			},
			{
				desc: "empty string values",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"empty": "",
					"blank": "   ",
				}},
				out: `some-pipeline/blank:"   ",empty:""`,
			},
			{
				desc: "strings that could be misinterpreted as JSON",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"json_looking":  "{\"key\":\"value\"}",
					"array_looking": "[1,2,3]",
				}},
				out: `some-pipeline/array_looking:"[1,2,3]",json_looking:"{\"key\":\"value\"}"`,
			},
			{
				desc: "special unicode characters",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"unicode": "こんにちは世界",
					"emoji":   "🚀 🔥 👍",
				}},
				// No fixed output - we'll check contents dynamically
				out: "",
			},
		} {
			tt := tt
			It(tt.desc, func() {
				if tt.desc == "special unicode characters" {
					result := tt.ref.String()
					Expect(result).To(HavePrefix("some-pipeline/"))

					// Check that the key names are present without specifying exact format
					Expect(result).To(ContainSubstring("emoji:"))
					Expect(result).To(ContainSubstring("unicode:"))

					// Check that the values are present
					Expect(result).To(ContainSubstring("🚀 🔥 👍"))
					Expect(result).To(ContainSubstring("こんにちは世界"))
				} else {
					Expect(tt.ref.String()).To(Equal(tt.out))
				}
			})
		}
	})

	Describe("QueryParams", func() {
		for _, tt := range []struct {
			desc string
			ref  atc.PipelineRef
			out  url.Values
		}{
			{
				desc: "empty",
				ref:  atc.PipelineRef{InstanceVars: nil},
				out:  nil,
			},
			{
				desc: "simple",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{"hello": "world", "num": 123}},
				out:  url.Values{"vars.hello": []string{`"world"`}, "vars.num": []string{`123`}},
			},
			{
				desc: "nested",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{"hello": map[string]any{"foo": 123, "bar": false}}},
				out:  url.Values{"vars.hello.foo": []string{`123`}, "vars.hello.bar": []string{`false`}},
			},
			{
				desc: "quoted",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{"hello.1": map[string]any{"foo:bar": "baz"}}},
				out:  url.Values{`vars."hello.1"."foo:bar"`: []string{`"baz"`}},
			},
			{
				desc: "empty map instance vars",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{}},
				out:  nil,
			},
			{
				desc: "special unicode characters",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{"emoji": "🚀", "unicode": "世界"}},
				out:  url.Values{"vars.emoji": []string{`"🚀"`}, "vars.unicode": []string{`"世界"`}},
			},
			{
				desc: "deeply nested complex structure",
				ref: atc.PipelineRef{InstanceVars: atc.InstanceVars{
					"complex": map[string]any{
						"nested": map[string]any{
							"array": []any{1, "two", map[string]any{"inner": true}},
							"deep": map[string]any{
								"deeper": map[string]any{
									"deepest": "value",
								},
							},
						},
					},
				}},
				out: url.Values{
					"vars.complex.nested.array":               []string{`[1,"two",{"inner":true}]`},
					"vars.complex.nested.deep.deeper.deepest": []string{`"value"`},
				},
			},
		} {
			tt := tt
			It(tt.desc, func() {
				Expect(tt.ref.QueryParams()).To(Equal(tt.out))
			})
		}
	})

	Describe("InstanceVarsFromQueryParams", func() {
		for _, tt := range []struct {
			desc  string
			query url.Values
			out   atc.InstanceVars
			err   string
		}{
			{
				desc:  "empty",
				query: url.Values{},
				out:   nil,
			},
			{
				desc: "simple",
				query: url.Values{
					"vars.hello": {`"world"`},
					"vars.foo":   {`"bar"`},
				},
				out: atc.InstanceVars{
					"hello": "world",
					"foo":   "bar",
				},
			},
			{
				desc: "complex refs",
				query: url.Values{
					`vars."a.b".c."d:e"`: {`"f"`},
				},
				out: atc.InstanceVars{
					"a.b": map[string]any{
						"c": map[string]any{
							"d:e": "f",
						},
					},
				},
			},
			{
				desc: "val is JSON",
				query: url.Values{
					`vars.foo"`: {`["a",{"b":123}]`},
				},
				out: atc.InstanceVars{
					"foo": []any{"a", map[string]any{"b": 123.0}},
				},
			},
			{
				desc: "root-level vars",
				query: url.Values{
					`vars`: {`{"foo":["a",{"b":123}]}`},
				},
				out: atc.InstanceVars{
					"foo": []any{"a", map[string]any{"b": 123.0}},
				},
			},
			{
				desc: "root-level vars with other subvars",
				query: url.Values{
					`vars`:     {`{"foo":["a",{"b":123}]}`},
					`vars.bar`: {`"baz"`},
				},
				out: atc.InstanceVars{
					"foo": []any{"a", map[string]any{"b": 123.0}},
					"bar": "baz",
				},
			},
			{
				desc: "ignores non-var params",
				query: url.Values{
					`vars.foo`: {`123`},
					`varsfoo`:  {`whatever`},
					`ignore"`:  {`blah`},
				},
				out: atc.InstanceVars{
					"foo": 123.0,
				},
			},
			{
				desc: "errors on invalid ref",
				query: url.Values{
					`vars.foo.`: {`123`},
				},
				err: "invalid var",
			},
			{
				desc: "errors when invalid JSON",
				query: url.Values{
					`vars.foo`: {`"123`},
				},
				err: "unexpected end of JSON input",
			},
			{
				desc:  "nil query params",
				query: nil,
				out:   nil,
			},
			{
				desc: "malformed reference",
				query: url.Values{
					`vars.foo..bar`: {`123`},
				},
				err: "invalid var",
			},
			{
				desc: "invalid JSON with detailed error message",
				query: url.Values{
					`vars.foo`: {`{"unclosed": "object"`},
				},
				err: "unexpected end of JSON input",
			},
			{
				desc: "unicode characters and emojis",
				query: url.Values{
					`vars.unicode`: {`"こんにちは世界"`},
					`vars.emoji`:   {`"🚀 🔥 👍"`},
				},
				out: atc.InstanceVars{
					"unicode": "こんにちは世界",
					"emoji":   "🚀 🔥 👍",
				},
			},
			{
				desc: "complex nested structure",
				query: url.Values{
					`vars.complex.nested.array`:               {`[1,"two",{"inner":true}]`},
					`vars.complex.nested.deep.deeper.deepest`: {`"value"`},
				},
				out: atc.InstanceVars{
					"complex": map[string]any{
						"nested": map[string]any{
							"array": []any{1.0, "two", map[string]any{"inner": true}},
							"deep": map[string]any{
								"deeper": map[string]any{
									"deepest": "value",
								},
							},
						},
					},
				},
			},
			{
				desc: "overlapping vars with both root and specific fields",
				query: url.Values{
					`vars`:                {`{"a":{"nested":{"value":1}},"b":"original"}`},
					`vars.a.nested.value`: {`2`},
					`vars.b`:              {`"overridden"`},
					`vars.c`:              {`"new"`},
				},
				out: atc.InstanceVars{
					"a": map[string]any{
						"nested": map[string]any{
							"value": 2.0,
						},
					},
					"b": "overridden",
					"c": "new",
				},
			},
		} {
			tt := tt
			It(tt.desc, func() {
				vars, err := atc.InstanceVarsFromQueryParams(tt.query)
				if tt.err != "" {
					Expect(err).To(MatchError(ContainSubstring(tt.err)))
				} else {
					Expect(err).ToNot(HaveOccurred())
					Expect(vars).To(Equal(tt.out))
				}
			})
		}
	})
})
