package atc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
)

var _ = Describe("DependentGetPlan", func() {
	It("can convert itself to a GetPlan", func() {
		dependentGetPlan := atc.DependentGetPlan{
			Type:     "resource-type",
			Name:     "resource-name",
			Resource: "resource-resource",
			Source: atc.Source{
				"resource": "source",
			},
			Params: atc.Params{
				"resource": "params",
			},
			Tags: []string{"tags"},
		}

		getPlan := atc.GetPlan{
			Type:     "resource-type",
			Name:     "resource-name",
			Resource: "resource-resource",
			Source: atc.Source{
				"resource": "source",
			},
			Params: atc.Params{
				"resource": "params",
			},
			Tags: []string{"tags"},
		}

		Expect(dependentGetPlan.GetPlan()).To(Equal(getPlan))
	})
})
