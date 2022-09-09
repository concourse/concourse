package creds_test

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/dummy"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VariableLookupFromSecrets", func() {
	var (
		variables vars.Variables
	)

	BeforeEach(func() {
		secrets := dummy.NewSecretsFactory([]dummy.VarFlag{
			{
				Name: "a",
				Value: map[string]interface{}{
					"b": map[interface{}]interface{}{
						"c": "foo",
					},
				},
			},
		}).NewSecrets()
		variables = creds.NewVariables(secrets, "team", "pipeline", true)
	})

	Describe("Get", func() {
		It("traverses fields", func() {
			result, found, err := variables.Get(vars.Reference{Path: "a", Fields: []string{"b", "c"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(result).To(Equal("foo"))
		})

		Context("when a field is missing", func() {
			It("errors", func() {
				_, _, err := variables.Get(vars.Reference{Path: "a", Fields: []string{"b", "d"}})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
