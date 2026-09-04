package creds_test

import (
	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/vars"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evaluate", func() {
	var plan creds.LoadVarPlan

	BeforeEach(func() {
		variables := vars.StaticVariables{
			"var-name":   "vn-is",
			"filename":   "fn-is",
			"var-format": "json",
		}
		plan = creds.NewLoadVarPlan(variables, atc.LoadVarPlan{
			Name:   "some-((var-name))-ok",
			File:   "some-((filename))-ok",
			Format: "((var-format))",
		})
	})

	Describe("Evaluate", func() {
		It("parses variables", func() {
			result, err := plan.Evaluate()
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(atc.LoadVarPlan{
				Name:   "some-((var-name))-ok", // Name should not be interpolated.
				File:   "some-fn-is-ok",
				Format: "json",
			}))
		})
	})
})
