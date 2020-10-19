package atc_test

import (
	"net/url"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
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
					"field.1": map[string]interface{}{
						"subfield:1": 1,
						"subfield 2": []interface{}{"1", 2, map[string]interface{}{"k": "v"}},
					},
					"other": "field",
				}},
				out: `some-pipeline/"field.1"."subfield 2":["1",2,{"k":"v"}],"field.1"."subfield:1":1,other:field`,
			},
			{
				desc: "instance vars sorted alphabetically",
				ref: atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{
					"b": map[string]interface{}{
						"foo": 1,
						"bar": []interface{}{"1", 2},
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
				}},
				out: `some-pipeline/colon:"a:b",comma:"a,b",space:"a b"`,
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
		} {
			tt := tt
			It(tt.desc, func() {
				Expect(tt.ref.String()).To(Equal(tt.out))
			})
		}
	})

	Describe("WebQueryParams", func() {
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
				out:  url.Values{"var.hello": []string{`"world"`}, "var.num": []string{`123`}},
			},
			{
				desc: "nested",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{"hello": map[string]interface{}{"foo": 123, "bar": false}}},
				out:  url.Values{"var.hello.foo": []string{`123`}, "var.hello.bar": []string{`false`}},
			},
			{
				desc: "quoted",
				ref:  atc.PipelineRef{InstanceVars: atc.InstanceVars{"hello.1": map[string]interface{}{"foo:bar": "baz"}}},
				out:  url.Values{`var."hello.1"."foo:bar"`: []string{`"baz"`}},
			},
		} {
			tt := tt
			It(tt.desc, func() {
				Expect(tt.ref.WebQueryParams()).To(Equal(tt.out))
			})
		}
	})
})
