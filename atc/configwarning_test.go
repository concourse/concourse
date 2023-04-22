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
		warningMsg  string
		warning     bool
		errorMsg    string
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
			description: "contains an underscore",
			identifier:  "some_thing",
			warning:     false,
		},
		{
			description: "starts with a number",
			identifier:  "1something",
			warningMsg:  "must start with a lowercase letter",
			warning:     true,
		},
		{
			description: "starts with hyphen",
			identifier:  "-something",
			warningMsg:  "must start with a lowercase letter",
			warning:     true,
		},
		{
			description: "starts with period",
			identifier:  ".something",
			warningMsg:  "must start with a lowercase letter",
			warning:     true,
		},
		{
			description: "starts with an uppercase letter",
			identifier:  "Something",
			warningMsg:  "must start with a lowercase letter",
			warning:     true,
		},
		{
			description: "contains a space",
			identifier:  "some thing",
			warningMsg:  "illegal character ' '",
			warning:     true,
		},
		{
			description: "contains an uppercase letter",
			identifier:  "someThing",
			warningMsg:  "illegal character 'T'",
			warning:     true,
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
			warning:     false,
		},
		{
			description: "is a var from across step in set_pipeline",
			context:     []string{".across", ".set_pipeline(((.:name)))"},
			identifier:  "running-((.:name))",
			warning:     false,
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
				warning, err := atc.ValidateIdentifier(test.identifier, test.context...)
				if test.warning {
					Expect(warning).NotTo(BeNil())
					Expect(warning.Message).To(ContainSubstring(test.warningMsg))
				} else {
					Expect(warning).To(BeNil())
					if test.errorMsg != "" {
						Expect(err).To(MatchError(test.errorMsg))
					}
				}
			})
		})
	}

	Describe("ValidateIdentifier with context", func() {
		Context("when an identifier is invalid", func() {
			It("returns an error with context", func() {
				warning, _ := atc.ValidateIdentifier("_something", "pipeline")
				Expect(warning).NotTo(BeNil())
				Expect(warning.Message).To(ContainSubstring("'_something' is not a valid identifier"))
			})
		})
	})
})
