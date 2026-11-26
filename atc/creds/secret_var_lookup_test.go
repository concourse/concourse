package creds_test

import (
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/atc/creds/dummy"
	"github.com/concourse/concourse/v8/vars"

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
				Value: map[string]any{
					"b": map[any]any{
						"c": "foo",
					},
				},
			},
		}).NewSecrets()
		variables = creds.NewVariables(secrets, creds.SecretLookupParams{Team: "team", Pipeline: "pipeline"}, true)
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

var _ = Describe("SecretLookupWithPrefix", func() {
	It("adds the prefix", func() {
		sl := creds.NewSecretLookupWithPrefix("some/prefix/")
		p, err := sl.VariableToSecretPath("my/secret")
		Expect(err).ToNot(HaveOccurred())
		Expect(p).To(Equal("some/prefix/my/secret"))
	})

	It("errors if the secret escapes the prefix", func() {
		sl := creds.NewSecretLookupWithPrefix("some/prefix/")
		p, err := sl.VariableToSecretPath("../../my/secret")
		Expect(err).To(MatchError("'../../my/secret' tries to escape the lookup path"))
		Expect(p).To(BeEmpty())
	})
})

var _ = Describe("SecretLookupWithTemplate", func() {
	It("renders the template", func() {
		t, err := creds.BuildSecretTemplate("name", "{{.Team}}/{{.Pipeline}}/{{.Secret}}")
		Expect(err).ToNot(HaveOccurred())

		sl := creds.NewSecretLookupWithTemplate(t, "some-team", "some-pipeline")
		p, err := sl.VariableToSecretPath("my/secret")
		Expect(err).ToNot(HaveOccurred())
		Expect(p).To(Equal("some-team/some-pipeline/my/secret"))
	})

	It("errors if the secret escapes the template", func() {
		t, err := creds.BuildSecretTemplate("name", "{{.Team}}/{{.Pipeline}}/{{.Secret}}")
		Expect(err).ToNot(HaveOccurred())

		sl := creds.NewSecretLookupWithTemplate(t, "some-team", "some-pipeline")
		p, err := sl.VariableToSecretPath("../../my/secret")
		Expect(err).To(MatchError("'../../my/secret' tries to escape the lookup path"))
		Expect(p).To(BeEmpty())
	})
})
