package template_test

import (
	boshtemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc/template"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Template", func() {
	var (
		paramPayload  []byte
		configPayload []byte
		staticVars    boshtemplate.StaticVariables
	)

	BeforeEach(func() {
		paramPayload = []byte(`
secret:
  concourse_repo:
    private_key: some-private-key

env: some-env

env-tags: ["speedy"]
`)
	})

	JustBeforeEach(func() {
		err := yaml.Unmarshal(paramPayload, &staticVars)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Evaluate", func() {
		Context("when all of the variables are defined", func() {
			BeforeEach(func() {
				configPayload = []byte(`
resources:
- name: my-repo
  source:
    uri: git@github.com:concourse/concourse.git
    private_key: ((secret.concourse_repo.private_key))

- name: env-state
  source:
    bucket: ((env))-ci
    key: state

jobs:
- name: do-some-stuff
  plan:
  - get: my-repo
  - task: build-thing-on-env
    tags: ((env-tags))
`)
			})

			It("evaluates all params", func() {
				evaluatedContent1, err := template.NewTemplateResolver(configPayload, []boshtemplate.Variables{staticVars}).Resolve(false, true)
				Expect(err).NotTo(HaveOccurred())

				evaluatedContent2, err := template.NewTemplateResolver(configPayload, []boshtemplate.Variables{staticVars}).Resolve(true, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(evaluatedContent1).To(Equal(evaluatedContent2))
				Expect(evaluatedContent1).To(MatchYAML([]byte(`
resources:
- name: my-repo
  source:
    uri: git@github.com:concourse/concourse.git
    private_key: some-private-key

- name: env-state
  source:
    bucket: some-env-ci
    key: state

jobs:
- name: do-some-stuff
  plan:
  - get: my-repo
  - task: build-thing-on-env
    tags: ["speedy"]
`,
				)))
			})
		})

		Context("when not all of the variables are defined", func() {
			BeforeEach(func() {
				configPayload = []byte(`
resources:
- name: my-repo
  source:
    uri: git@github.com:concourse/concourse.git
    private_key: ((secret.concourse_repo.private_key))
- name: env-state
  source:
    bucket: ((bucket))
    key: ((state))
`)
			})

			It("evaluates only given params if expectAllKeys = false", func() {
				evaluatedContent, err := template.NewTemplateResolver(configPayload, []boshtemplate.Variables{staticVars}).Resolve(false, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(evaluatedContent).To(MatchYAML([]byte(`
resources:
- name: my-repo
  source:
    uri: git@github.com:concourse/concourse.git
    private_key: some-private-key
- name: env-state
  source:
    bucket: ((bucket))
    key: ((state))
`,
				)))
			})

			It("fails with an error if expectAllKeys = true", func() {
				_, err := template.NewTemplateResolver(configPayload, []boshtemplate.Variables{staticVars}).Resolve(true, true)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	It("can template values into a byte slice", func() {
		byteSlice := []byte("{{key}}")
		variables := boshtemplate.StaticVariables{
			"key": "foo",
		}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"`)))
	})

	It("can template multiple values into a byte slice", func() {
		byteSlice := []byte("{{key}}={{value}}")
		variables := boshtemplate.StaticVariables{
			"key":   "foo",
			"value": "bar",
		}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"="bar"`)))
	})

	It("can template unicode values into a byte slice", func() {
		byteSlice := []byte("{{Ω}}")
		variables := boshtemplate.StaticVariables{
			"Ω": "☃",
		}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"☃"`)))
	})

	It("can template keys with dashes and underscores into a byte slice", func() {
		byteSlice := []byte("{{with-a-dash}} = {{with_an_underscore}}")
		variables := boshtemplate.StaticVariables{
			"with-a-dash":        "dash",
			"with_an_underscore": "underscore",
		}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"dash" = "underscore"`)))
	})

	It("can template the same value multiple times into a byte slice", func() {
		byteSlice := []byte("{{key}}={{key}}")
		variables := boshtemplate.StaticVariables{
			"key": "foo",
		}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"="foo"`)))
	})

	It("can template values with strange newlines", func() {
		byteSlice := []byte("{{key}}")
		variables := boshtemplate.StaticVariables{
			"key": "this\nhas\nmany\nlines",
		}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"this\nhas\nmany\nlines"`)))
	})

	It("raises an error for each variable that is undefined", func() {
		byteSlice := []byte("{{not-specified-one}}{{not-specified-two}}")
		variables := boshtemplate.StaticVariables{}
		errorMsg := `2 errors occurred:
	* unbound variable in template: 'not-specified-one'
	* unbound variable in template: 'not-specified-two'

`

		_, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorMsg))
	})

	It("ignores an invalid input", func() {
		byteSlice := []byte("{{}")
		variables := boshtemplate.StaticVariables{}

		result, err := template.NewTemplateResolver(byteSlice, []boshtemplate.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("{{}")))
	})

})
