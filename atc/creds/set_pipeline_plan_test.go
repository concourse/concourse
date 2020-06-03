package creds_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evaluate", func() {
	var plan creds.SetPipelinePlan

	BeforeEach(func() {
		variables := vars.StaticVariables{
			"pipeline-name": "pn-is",
			"filename":      "fn-is",
			"varfile":       "vf-is",
			"age":           "18",
		}
		plan = creds.NewSetPipelinePlan(variables, atc.SetPipelinePlan{
			Name:     "some-((pipeline-name))-ok",
			File:     "some-((filename))-ok",
			VarFiles: []string{"some-((varfile))-ok"},
			Vars:     map[string]interface{}{"age": "((age))"},
		})
	})

	Describe("Evaluate", func() {
		It("parses variables", func() {
			result, err := plan.Evaluate()
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(atc.SetPipelinePlan{
				Name:     "some-((pipeline-name))-ok", // Name should not be interpolated.
				File:     "some-fn-is-ok",
				VarFiles: []string{"some-vf-is-ok"},
				Vars:     map[string]interface{}{"age": "18"},
			}))
		})
	})
})
