package template_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/fly/template"
)

var _ = Describe("Template", func() {
	It("can template values into a byte slice", func() {
		byteSlice := []byte("{{key}}")
		variables := template.Variables{
			"key": "foo",
		}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"`)))
	})

	It("can template multiple values into a byte slice", func() {
		byteSlice := []byte("{{key}}={{value}}")
		variables := template.Variables{
			"key":   "foo",
			"value": "bar",
		}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"="bar"`)))
	})

	It("can template unicode values into a byte slice", func() {
		byteSlice := []byte("{{Ω}}")
		variables := template.Variables{
			"Ω": "☃",
		}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"☃"`)))
	})

	It("can template keys with dashes and underscores into a byte slice", func() {
		byteSlice := []byte("{{with-a-dash}} = {{with_an_underscore}}")
		variables := template.Variables{
			"with-a-dash":        "dash",
			"with_an_underscore": "underscore",
		}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"dash" = "underscore"`)))
	})

	It("can template the same value multiple times into a byte slice", func() {
		byteSlice := []byte("{{key}}={{key}}")
		variables := template.Variables{
			"key": "foo",
		}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"="foo"`)))
	})

	It("can template values with strange newlines", func() {
		byteSlice := []byte("{{key}}")
		variables := template.Variables{
			"key": "this\nhas\nmany\nlines",
		}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"this\nhas\nmany\nlines"`)))
	})

	It("raises an error for each variable that is undefined", func() {
		byteSlice := []byte("{{not-specified-one}}{{not-specified-two}}")
		variables := template.Variables{}
		errorMsg := `2 errors occurred:

* unbound variable in template: 'not-specified-one'
* unbound variable in template: 'not-specified-two'`

		_, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorMsg))
	})

	It("ignores an invalid input", func() {
		byteSlice := []byte("{{}")
		variables := template.Variables{}

		result, err := template.Evaluate(byteSlice, variables, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("{{}")))
	})
})
