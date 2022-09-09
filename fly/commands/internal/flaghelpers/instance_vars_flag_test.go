package flaghelpers_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

var _ = Describe("InstanceVarsFlag", func() {
	Describe("UnmarshalFlag", func() {

		var instanceVarsFlag *flaghelpers.InstanceVarsFlag

		BeforeEach(func() {
			instanceVarsFlag = &flaghelpers.InstanceVarsFlag{}
		})

		for _, tt := range []struct {
			desc         string
			flag         string
			instanceVars atc.InstanceVars
			err          string
		}{
			{
				desc:         "instance var",
				flag:         "branch:master",
				instanceVars: atc.InstanceVars{"branch": "master"},
			},
			{
				desc: "multiple instance vars",
				flag: `branch:master,list:[1, "2"],other:{foo:bar: 123}`,
				instanceVars: atc.InstanceVars{
					"branch": "master",
					"list":   []interface{}{json.Number("1"), "2"},
					"other":  map[string]interface{}{"foo:bar": json.Number("123")},
				},
			},
			{
				desc: "quoted yaml brackets/braces with \"",
				flag: `field1:{a: "{", b: "["},field2:hello`,
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
				flag: `field1:{a: '{', b: '['},field2:hello`,
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
				flag: `field1:,field2:"",field3:null`,
				instanceVars: atc.InstanceVars{
					"field1": nil,
					"field2": "",
					"field3": nil,
				},
			},
			{
				desc: "yaml list",
				flag: `field:[{a: '{', b: '['}, 1]`,
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
				flag: `field.0:0,field.1:1`,
				instanceVars: atc.InstanceVars{
					"field": map[string]interface{}{
						"0": json.Number("0"),
						"1": json.Number("1"),
					},
				},
			},
			{
				desc: "whitespace trimmed from path/values",
				flag: `branch: master, other: 123`,
				instanceVars: atc.InstanceVars{
					"branch": "master",
					"other":  json.Number("123"),
				},
			},
			{
				desc: "quoted fields can contain special characters",
				flag: `"some.field:here":abc`,
				instanceVars: atc.InstanceVars{
					"some.field:here": "abc",
				},
			},
			{
				desc: "special characters in quoted yaml",
				flag: `field1:'foo,bar',field2:"value1:value2"`,
				instanceVars: atc.InstanceVars{
					"field1": "foo,bar",
					"field2": "value1:value2",
				},
			},
			{
				desc: "supports dot notation",
				flag: `"my.field".subkey1."subkey:2":"my-value","my.field".other:'other-value'`,
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
				flag: `"my.field".:bad`,
				err:  `invalid var '"my.field".': empty field`,
			},
			{
				desc: "errors if invalid YAML is passed as the value",
				flag: `hello:{bad: yaml`,
				err:  `invalid value for key 'hello': error converting YAML to JSON: yaml: line 1: did not find expected ',' or '}'`,
			},
		} {
			tt := tt
			It(tt.desc, func() {
				err := instanceVarsFlag.UnmarshalFlag(tt.flag)
				if tt.err == "" {
					Expect(err).ToNot(HaveOccurred())
					Expect(instanceVarsFlag.InstanceVars).To(Equal(tt.instanceVars))
				} else {
					Expect(err).To(MatchError(tt.err))
				}
			})
		}
	})
})
