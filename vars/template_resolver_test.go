package vars_test

import (
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Template", func() {
	var (
		paramPayload  []byte
		configPayload []byte
		staticVars    vars.StaticVariables
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
				evaluatedContent1, err := vars.NewTemplateResolver(configPayload, []vars.Variables{staticVars}).Resolve(false, true)
				Expect(err).NotTo(HaveOccurred())

				evaluatedContent2, err := vars.NewTemplateResolver(configPayload, []vars.Variables{staticVars}).Resolve(true, true)
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
				evaluatedContent, err := vars.NewTemplateResolver(configPayload, []vars.Variables{staticVars}).Resolve(false, true)
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
				_, err := vars.NewTemplateResolver(configPayload, []vars.Variables{staticVars}).Resolve(true, true)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when multiple variable sources are given", func() {

			var staticVars2 vars.StaticVariables

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

				paramPayload2 := []byte(`
secret:
  concourse_repo:
    private_key: some-private-key-override

env: some-env-override
`)
				err := yaml.Unmarshal(paramPayload2, &staticVars2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("evaluates params using param sources in the given order", func() {
				// forward order
				evaluatedContent1, err := vars.NewTemplateResolver(configPayload, []vars.Variables{staticVars, staticVars2}).Resolve(false, true)
				Expect(err).NotTo(HaveOccurred())
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

				// reverse order
				evaluatedContent2, err := vars.NewTemplateResolver(configPayload, []vars.Variables{staticVars2, staticVars}).Resolve(false, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(evaluatedContent2).To(MatchYAML([]byte(`
resources:
- name: my-repo
  source:
    uri: git@github.com:concourse/concourse.git
    private_key: some-private-key-override

- name: env-state
  source:
    bucket: some-env-override-ci
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
	})

	It("can template values into a byte slice", func() {
		byteSlice := []byte("{{key}}")
		variables := vars.StaticVariables{
			"key": "foo",
		}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"`)))
	})

	It("can template multiple values into a byte slice", func() {
		byteSlice := []byte("{{key}}={{value}}")
		variables := vars.StaticVariables{
			"key":   "foo",
			"value": "bar",
		}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"="bar"`)))
	})

	It("can template unicode values into a byte slice", func() {
		byteSlice := []byte("{{Ω}}")
		variables := vars.StaticVariables{
			"Ω": "☃",
		}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"☃"`)))
	})

	It("can template keys with dashes and underscores into a byte slice", func() {
		byteSlice := []byte("{{with-a-dash}} = {{with_an_underscore}}")
		variables := vars.StaticVariables{
			"with-a-dash":        "dash",
			"with_an_underscore": "underscore",
		}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"dash" = "underscore"`)))
	})

	It("can template the same value multiple times into a byte slice", func() {
		byteSlice := []byte("{{key}}={{key}}")
		variables := vars.StaticVariables{
			"key": "foo",
		}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"foo"="foo"`)))
	})

	It("can template values with strange newlines", func() {
		byteSlice := []byte("{{key}}")
		variables := vars.StaticVariables{
			"key": "this\nhas\nmany\nlines",
		}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`"this\nhas\nmany\nlines"`)))
	})

	It("raises an error for each variable that is undefined", func() {
		byteSlice := []byte("{{not-specified-one}}{{not-specified-two}}")
		variables := vars.StaticVariables{}
		errorMsg := `2 errors occurred:
	* unbound variable in template: 'not-specified-one'
	* unbound variable in template: 'not-specified-two'

`

		_, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(errorMsg))
	})

	It("ignores an invalid input", func() {
		byteSlice := []byte("{{}")
		variables := vars.StaticVariables{}

		result, err := vars.NewTemplateResolver(byteSlice, []vars.Variables{variables}).ResolveDeprecated(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("{{}")))
	})

})
