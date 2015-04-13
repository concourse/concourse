package template_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/fly/template"
)

var _ = Describe("Variables", func() {
	It("converts an array of key=value strings to Variables", func() {
		variables := template.Variables{
			"key":   "foo",
			"value": "bar",
		}

		input := []string{
			"key=foo",
			"value=bar",
		}

		loadedVariables, err := template.LoadVariables(input)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(loadedVariables).Should(Equal(variables))
	})

	It("allows values to have an = sign in them", func() {
		variables := template.Variables{
			"key": "foo=bar",
		}

		input := []string{
			"key=foo=bar",
		}

		loadedVariables, err := template.LoadVariables(input)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(loadedVariables).Should(Equal(variables))
	})

	It("allows unicode values", func() {
		variables := template.Variables{
			"Ω": "☃",
		}

		input := []string{
			"Ω=☃",
		}

		loadedVariables, err := template.LoadVariables(input)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(loadedVariables).Should(Equal(variables))
	})

	It("errors if the input is invalid", func() {
		input := []string{
			"key",
		}

		_, err := template.LoadVariables(input)
		Ω(err).Should(HaveOccurred())
		Ω(err).Should(MatchError("input has incorrect format (should be key=value): 'key'"))
	})
})
