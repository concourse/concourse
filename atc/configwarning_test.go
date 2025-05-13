package atc_test

import (
	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateIdentifier", func() {
	type testCase struct {
		description string
		identifier  string
		context     []string
		errorMsg    string
	}

	for _, test := range []testCase{
		{
			description: "starts with a valid letter",
			identifier:  "something",
			errorMsg:    "",
		},
		{
			description: "contains multilingual characters",
			identifier:  "ひらがな",
			errorMsg:    "",
		},
		{
			description: "contains an underscore",
			identifier:  "some_thing",
			errorMsg:    "",
		},
		{
			description: "starts with a number",
			identifier:  "1something",
			errorMsg:    "",
		},
		{
			description: "starts with a number",
			identifier:  "1_min",
			errorMsg:    "",
		},
		{
			description: "starts with hyphen",
			identifier:  "-something",
			errorMsg:    ": '-something' is not a valid identifier: must start with a lowercase letter or a number",
		},
		{
			description: "starts with period",
			identifier:  ".something",
			errorMsg:    ": '.something' is not a valid identifier: must start with a lowercase letter or a number",
		},
		{
			description: "starts with an uppercase letter",
			identifier:  "Something",
			errorMsg:    ": 'Something' is not a valid identifier: must start with a lowercase letter or a number",
		},
		{
			description: "contains a space",
			identifier:  "some thing",
			errorMsg:    ": 'some thing' is not a valid identifier: illegal character ' '",
		},
		{
			description: "contains an uppercase letter",
			identifier:  "someThing",
			errorMsg:    ": 'someThing' is not a valid identifier: illegal character 'T'",
		},
		{
			description: "is an empty string",
			identifier:  "",
			errorMsg:    ": identifier cannot be an empty string",
		},
		{
			description: "is a var from across step in task",
			context:     []string{".across", ".task(running-((.:name)))"},
			identifier:  "((.:name))",
			errorMsg:    "",
		},
		{
			description: "is a var from across step in set_pipeline",
			context:     []string{".across", ".set_pipeline(((.:name)))"},
			identifier:  "running-((.:name))",
			errorMsg:    "",
		},
	} {
		test := test

		Context("when an identifier "+test.description, func() {

			It("validate identifier", func() {
				configError := atc.ValidateIdentifier(test.identifier, test.context...)
				if test.errorMsg != "" {
					Expect(configError).NotTo(BeNil())
					Expect(configError.Message).To(Equal(test.errorMsg))
				} else {
					Expect(configError).To(BeNil())
				}
			})
		})
	}

	Describe("ValidateIdentifier with context", func() {
		Context("when an identifier is invalid", func() {
			It("returns an error with context", func() {
				configError := atc.ValidateIdentifier("_something", "pipeline")
				Expect(configError).NotTo(BeNil())
				Expect(configError.Error()).To(ContainSubstring("pipeline: '_something' is not a valid identifier: must start with a lowercase letter or a number"))
			})
		})
	})
})
