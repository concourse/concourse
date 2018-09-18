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

			Expect(result).To(Equal(template.Variables{
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

			Expect(a).To(Equal(template.Variables{
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

			Expect(result).To(Equal(template.Variables{
				"a": "foo",
				"b": "new",
			}))

		})
	})
})
