package flaghelpers_test

import (
	"encoding/json"

	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

var _ = Describe("YAMLVariablePair", func() {
	Describe("UnmarshalFlag", func() {
		var variable *flaghelpers.YAMLVariablePairFlag

		BeforeEach(func() {
			variable = &flaghelpers.YAMLVariablePairFlag{}
		})

		for _, tt := range []struct {
			desc string
			flag string
			ref  vars.Reference
			val  interface{}
			err  string
		}{
			{
				desc: "basic",
				flag: "key=value",
				ref:  vars.Reference{Path: "key", Fields: []string{}},
				val:  "value",
			},
			{
				desc: "unmarshals arbitrary yaml",
				flag: `key={hello:world: [a, b, c]}`,
				ref:  vars.Reference{Path: `key`, Fields: []string{}},
				val:  map[string]interface{}{"hello:world": []interface{}{"a", "b", "c"}},
			},
			{
				desc: "unmarshals numbers as json.Number",
				flag: `key={int: 1, float: 1.23}`,
				ref:  vars.Reference{Path: `key`, Fields: []string{}},
				val:  map[string]interface{}{"int": json.Number("1"), "float": json.Number("1.23")},
			},
			{
				desc: "errors if yaml is malformed",
				flag: `key={a: b`,
				err:  `error converting YAML to JSON: yaml: line 1: did not find expected ',' or '}'`,
			},
		} {
			tt := tt
			It(tt.desc, func() {
				err := variable.UnmarshalFlag(tt.flag)
				if tt.err == "" {
					Expect(err).ToNot(HaveOccurred())
					Expect(variable.Ref).To(Equal(tt.ref))
					Expect(variable.Value).To(Equal(tt.val))
				} else {
					Expect(err).To(MatchError(tt.err))
				}
			})
		}
	})
})
