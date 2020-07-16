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
		warning     bool
	}

	for _, test := range []testCase{
		{
			description: "starts with a valid letter",
			identifier:  "something",
			warning:     false,
		},
		{
			description: "contains multilingual characters",
			identifier:  "ひらがな",
			warning:     false,
		},
		{
			description: "starts with a number",
			identifier:  "1something",
			warning:     true,
		},
		{
			description: "starts with hyphen",
			identifier:  "-something",
			warning:     true,
		},
		{
			description: "starts with period",
			identifier:  ".something",
			warning:     true,
		},
		{
			description: "starts with an uppercase letter",
			identifier:  "Something",
			warning:     true,
		},
		{
			description: "contains an underscore",
			identifier:  "some_thing",
			warning:     true,
		},
		{
			description: "contains an uppercase letter",
			identifier:  "someThing",
			warning:     true,
		},
	} {
		test := test

		Context("when an identifier "+test.description, func() {
			var it string
			if test.warning {
				it = "returns a warning"
			} else {
				it = "runs without warning"
			}
			It(it, func() {
				warning := atc.ValidateIdentifier(test.identifier)
				if test.warning {
					Expect(warning).NotTo(BeNil())
				} else {
					Expect(warning).To(BeNil())
				}
			})
		})
	}

	Describe("ValidateIdentifier with context", func() {
		Context("when an identifier is invalid", func() {
			It("returns an error with context", func() {
				warning := atc.ValidateIdentifier("_something", "pipeline")
				Expect(warning).NotTo(BeNil())
				Expect(warning.Message).To(ContainSubstring("'_something' is not a valid identifier"))
			})
		})
	})
})
