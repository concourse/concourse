package atc_test

import (
	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateIdentifier", func() {
	type testCase struct {
		description string
		identifier  string
		errors      bool
	}

	for _, test := range []testCase{
		{
			description: "starts with a valid letter",
			identifier:  "something",
			errors:      false,
		},
		{
			description: "contains multilingual characters",
			identifier:  "ひらがな",
			errors:      false,
		},
		{
			description: "starts with a number",
			identifier:  "1something",
			errors:      true,
		},
		{
			description: "starts with hyphen",
			identifier:  "-something",
			errors:      true,
		},
		{
			description: "starts with period",
			identifier:  ".something",
			errors:      true,
		},
		{
			description: "starts with an uppercase letter",
			identifier:  "Something",
			errors:      true,
		},
		{
			description: "contains an underscore",
			identifier:  "some_thing",
			errors:      true,
		},
		{
			description: "contains an uppercase letter",
			identifier:  "someThing",
			errors:      true,
		},
	} {
		test := test

		Context("when an identifier "+test.description, func() {
			var it string
			if test.errors {
				it = "returns an error"
			} else {
				it = "runs without error"
			}
			It(it, func() {
				err := atc.ValidateIdentifier(test.identifier)
				if test.errors {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
	}

	Describe("ValidateIdentifier with context", func() {
		Context("when an identifier is invalid", func() {
			It("returns an error with context", func() {
				err := atc.ValidateIdentifier("_something", "pipeline")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("'_something' is not a valid identifier"))
			})
		})

	})
})
