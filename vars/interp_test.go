package vars_test

import (
	"encoding/json"

	. "github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interpolation", func() {
	Describe("UnmarshalJSON", func() {
		Describe("Var", func() {
			for _, tt := range []struct {
				desc   string
				body   string
				result Var
				err    string
			}{
				{
					desc: "simple Var",
					body: `"((hello))"`,

					result: Var{Path: "hello", Fields: []string{}},
				},
				{
					desc: "complex Var",
					body: `"((source:\"hello.world\".path1.path2))"`,

					result: Var{
						Source: "source",
						Path:   "hello.world",
						Fields: []string{"path1", "path2"},
					},
				},
				{
					desc: "invalid Var",
					body: `"no var here"`,

					err: "assigned value 'no var here' is not a var reference",
				},
				{
					desc: "non-anchored Var",
					body: `"foo((bar))baz"`,

					err: `assigned value 'foo\(\(bar\)\)baz' is not a var reference`,
				},
			} {
				tt := tt

				It(tt.desc, func() {
					var dst Var
					err := json.Unmarshal([]byte(tt.body), &dst)
					if tt.err != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(MatchRegexp(tt.err))
					} else {
						Expect(err).ToNot(HaveOccurred())
						Expect(dst).To(Equal(tt.result))
					}
				})
			}
		})
	})

	Describe("Interpolate", func() {
		Describe("String", func() {
			for _, tt := range []struct {
				desc   string
				str    String
				vars   Variables
				result string
				err    string
			}{
				{
					desc: "no var",
					str:  "hello",

					result: "hello",
				},
				{
					desc: "anchored var",
					str:  "((hello))",
					vars: StaticVariables{"hello": "world"},

					result: "world",
				},
				{
					desc: "anchored var with primitive",
					str:  "((hello))",
					vars: StaticVariables{"hello": 123},

					result: "123",
				},
				{
					desc: "anchored var with non-primitive",
					str:  "((hello))",
					vars: StaticVariables{"hello": []interface{}{"bad"}},

					err: "cannot interpolate non-primitive value.*hello",
				},
				{
					desc: "coerces nil to \"null\"",
					str:  "((hello))",
					vars: StaticVariables{"hello": nil},

					result: "null",
				},
				{
					desc: "interspersed vars",
					str:  "abc-((hello))-ghi-((world))-((blah))",
					vars: StaticVariables{"hello": "def", "world": 123, "blah": true},

					result: "abc-def-ghi-123-true",
				},
				{
					desc: "can intersperse floats",
					str:  "abc-((hello))-ghi-((world))-((blah))",
					vars: StaticVariables{"hello": "def", "world": 123.456, "blah": true},

					result: "abc-def-ghi-123.456-true",
				},
				{
					desc: "non-string/non-number var",
					str:  String("((hello))-abc"),
					vars: StaticVariables{"hello": []string{"a", "b", "c"}},

					err: "cannot interpolate non-primitive value",
				},
				{
					desc: "missing vars",
					str:  "something-((hello))-else-((world))",
					vars: StaticVariables{},

					err: `undefined vars: hello\n.*` +
						`undefined vars: world`,
				},
			} {
				tt := tt

				It(tt.desc, func() {
					result, err := tt.str.Interpolate(NewResolver(tt.vars))
					if tt.err != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(MatchRegexp(tt.err))
					} else {
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(tt.result))
					}
				})
			}
		})

		Describe("Var", func() {
			for _, tt := range []struct {
				desc   string
				ref    Var
				vars   Variables
				opts   []JSONOpt
				result interface{}
				err    string
			}{
				{
					desc: "string var",
					ref:  Var{Path: "hello"},
					vars: StaticVariables{"hello": "world"},

					result: "world",
				},
				{
					desc: "list var",
					ref:  Var{Path: "hello"},
					vars: StaticVariables{"hello": []string{"abc", "def"}},

					result: []interface{}{"abc", "def"},
				},
				{
					desc: "use number",
					ref:  Var{Path: "hello"},
					vars: StaticVariables{"hello": 123},
					opts: []JSONOpt{UseNumber},

					result: json.Number("123"),
				},
				{
					desc: "default decoder",
					ref:  Var{Path: "hello"},
					vars: StaticVariables{"hello": 123},

					result: float64(123),
				},
				{
					desc: "missing var",
					ref:  Var{Path: "hello"},
					vars: StaticVariables{},

					err: `undefined vars: hello`,
				},
			} {
				tt := tt

				It(tt.desc, func() {
					var dst interface{}
					err := tt.ref.InterpolateInto(NewResolver(tt.vars), &dst, tt.opts...)
					if tt.err != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(MatchRegexp(tt.err))
					} else {
						Expect(err).ToNot(HaveOccurred())
						Expect(dst).To(Equal(tt.result))
					}
				})
			}
		})

		Describe("Any", func() {
			for _, tt := range []struct {
				desc   string
				any    Any
				vars   Variables
				result interface{}
				err    string
			}{
				{
					desc: "rooted var",
					any:  "((hello))",
					vars: StaticVariables{"hello": map[string]interface{}{"foo": "bar"}},

					result: map[string]interface{}{"foo": "bar"},
				},
				{
					desc: "var interspersed in string",
					any:  "((hello)) world ((number))",
					vars: StaticVariables{"hello": "sup", "number": 100},

					result: "sup world 100",
				},
				{
					desc: "traverses maps, interpolating keys and values",
					any: map[string]interface{}{
						"((hello))": map[string]interface{}{
							"foo": "((something))",
						},
					},
					vars: StaticVariables{
						"hello": "sup",
						"something": map[string]interface{}{
							"bar": "baz",
						},
					},

					result: map[string]interface{}{
						"sup": map[string]interface{}{
							"foo": map[string]interface{}{
								"bar": "baz",
							},
						},
					},
				},
				{
					desc: "key value must interpolate to a string",
					any: map[string]interface{}{
						"((k))": "val",
					},
					vars: StaticVariables{
						"k": []interface{}{"nope!"},
					},

					err: "cannot interpolate non-primitive value.*k",
				},
				{
					desc: "traverses lists, interpolating values",
					any: []interface{}{
						"((hello))",
						map[string]interface{}{
							"((k))": []interface{}{"((v))", "other"},
						},
					},
					vars: StaticVariables{
						"hello": "sup",
						"k":     "key",
						"v":     "val",
					},

					result: []interface{}{
						"sup",
						map[string]interface{}{
							"key": []interface{}{"val", "other"},
						},
					},
				},
			} {
				tt := tt

				It(tt.desc, func() {
					val, err := Interpolate(tt.any, NewResolver(tt.vars))
					if tt.err != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(MatchRegexp(tt.err))
					} else {
						Expect(err).ToNot(HaveOccurred())
						Expect(val).To(Equal(tt.result))
					}
				})
			}
		})
	})
})
