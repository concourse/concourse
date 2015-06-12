package atc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
)

var _ = Describe("Plan", func() {
	Describe("PutPlan", func() {
		It("can convert itself to a GetPlan", func() {
			putPlan := atc.PutPlan{
				Type:     "resource-type",
				Name:     "resource-name",
				Resource: "resource-resource",
				Pipeline: "resource-pipeline",
				Source: atc.Source{
					"resource": "source",
				},
				Params: atc.Params{
					"resource": "params",
				},
				GetParams: atc.Params{
					"resource": "get-params",
				},
				Tags: []string{"tags"},
			}

			getPlan := atc.GetPlan{
				Type:     "resource-type",
				Name:     "resource-name",
				Resource: "resource-resource",
				Pipeline: "resource-pipeline",
				Source: atc.Source{
					"resource": "source",
				},
				Params: atc.Params{
					"resource": "get-params",
				},
				Tags: []string{"tags"},
			}

			Ω(putPlan.GetPlan()).Should(Equal(getPlan))
		})
	})
})
