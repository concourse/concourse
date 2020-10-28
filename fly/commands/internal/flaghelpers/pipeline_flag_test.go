package flaghelpers_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

var _ = Describe("PipelineFlag", func() {
	Describe("UnmarshalFlag", func() {

		var pipelineFlag *flaghelpers.PipelineFlag

		BeforeEach(func() {
			pipelineFlag = &flaghelpers.PipelineFlag{}
		})

		for _, tt := range []struct {
			desc         string
			flag         string
			name         string
			instanceVars atc.InstanceVars
			err          string
		}{
			{
				desc: "name",
				flag: "some-pipeline",
				name: "some-pipeline",
			},
			{
				desc:         "instance var",
				flag:         "some-pipeline/branch:master",
				name:         "some-pipeline",
				instanceVars: atc.InstanceVars{"branch": "master"},
			},
			{
				desc: "multiple instance vars",
				flag: `some-pipeline/branch:master,list:[1, "2"],other:{foo:bar: 123}`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"branch": "master",
					"list":   []interface{}{json.Number("1"), "2"},
					"other":  map[string]interface{}{"foo:bar": json.Number("123")},
				},
			},
			{
				desc: "quoted yaml brackets/braces with \"",
				flag: `some-pipeline/field1:{a: "{", b: "["},field2:hello`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"field1": map[string]interface{}{
						"a": "{",
						"b": "[",
					},
					"field2": "hello",
				},
			},
			{
				desc: "quoted yaml brackets/braces with '",
				flag: `some-pipeline/field1:{a: '{', b: '['},field2:hello`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"field1": map[string]interface{}{
						"a": "{",
						"b": "[",
					},
					"field2": "hello",
				},
			},
			{
				desc: "empty values",
				flag: `some-pipeline/field1:,field2:"",field3:null`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"field1": nil,
					"field2": "",
					"field3": nil,
				},
			},
			{
				desc: "yaml list",
				flag: `some-pipeline/field:[{a: '{', b: '['}, 1]`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"field": []interface{}{
						map[string]interface{}{
							"a": "{",
							"b": "[",
						},
						json.Number("1"),
					},
				},
			},
			{
				desc: "indexing by numerical field still uses map",
				flag: `some-pipeline/field.0:0,field.1:1`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"field": map[string]interface{}{
						"0": json.Number("0"),
						"1": json.Number("1"),
					},
				},
			},
			{
				desc: "whitespace trimmed from path/values",
				flag: `some-pipeline/branch: master, other: 123`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"branch": "master",
					"other":  json.Number("123"),
				},
			},
			{
				desc: "quoted fields can contain special characters",
				flag: `some-pipeline/"some.field:here":abc`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"some.field:here": "abc",
				},
			},
			{
				desc: "special characters in quoted yaml",
				flag: `some-pipeline/field1:'foo,bar',field2:"value1:value2"`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"field1": "foo,bar",
					"field2": "value1:value2",
				},
			},
			{
				desc: "supports dot notation",
				flag: `some-pipeline/"my.field".subkey1."subkey:2":"my-value","my.field".other:'other-value'`,
				name: "some-pipeline",
				instanceVars: atc.InstanceVars{
					"my.field": map[string]interface{}{
						"subkey1": map[string]interface{}{
							"subkey:2": "my-value",
						},
						"other": "other-value",
					},
				},
			},
			{
				desc: "errors if invalid ref is passed",
				flag: `some-pipeline/"my.field".:bad`,
				err:  `invalid var '"my.field".': empty field`,
			},
			{
				desc: "errors if invalid YAML is passed as the value",
				flag: `some-pipeline/hello:{bad: yaml`,
				err:  `invalid value for key 'hello': error converting YAML to JSON: yaml: line 1: did not find expected ',' or '}'`,
			},
		} {
			tt := tt
			It(tt.desc, func() {
				err := pipelineFlag.UnmarshalFlag(tt.flag)
				if tt.err == "" {
					Expect(err).ToNot(HaveOccurred())
					Expect(pipelineFlag.Name).To(Equal(tt.name))
					Expect(pipelineFlag.InstanceVars).To(Equal(tt.instanceVars))
				} else {
					Expect(err).To(MatchError(tt.err))
				}
			})
		}
	})
})
