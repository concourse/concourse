package template_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/fly/template"
)

var _ = Describe("Variables", func() {
	Describe("merging two sets of variables together", func() {
		It("merges the two sets into one", func() {
			a := template.Variables{
				"a": "foo",
			}
			b := template.Variables{
				"b": "bar",
			}

			result := a.Merge(b)

			Ω(result).Should(Equal(template.Variables{
				"a": "foo",
				"b": "bar",
			}))
		})

		It("does not affect the original sets", func() {
			a := template.Variables{
				"a": "foo",
			}
			b := template.Variables{
				"b": "bar",
			}

			a.Merge(b)

			Ω(a).Should(Equal(template.Variables{
				"a": "foo",
			}))
		})

		It("overwrites the LHS with the RHS", func() {
			a := template.Variables{
				"a": "foo",
				"b": "old",
			}
			b := template.Variables{
				"b": "new",
			}

			result := a.Merge(b)

			Ω(result).Should(Equal(template.Variables{
				"a": "foo",
				"b": "new",
			}))
		})
	})

	Describe("loading variables from a slice of key value pairs", func() {
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

	Describe("loading variables from a file", func() {
		It("can load them from a file", func() {
			variables, err := template.LoadVariablesFromFile("fixtures/vars.yml")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(variables).Should(Equal(template.Variables{
				"hello": "world",
			}))
		})

		It("returns an error if the file does not exist", func() {
			_, err := template.LoadVariablesFromFile("fixtures/missing.yml")
			Ω(err).Should(HaveOccurred())
		})

		It("returns an error if the file is in an invalid format", func() {
			_, err := template.LoadVariablesFromFile("fixtures/invalid_vars.yml")
			Ω(err).Should(HaveOccurred())
		})
	})
})
