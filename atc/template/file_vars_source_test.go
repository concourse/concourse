package template_test

import (
	. "github.com/concourse/atc/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileVarsSource", func() {
	var (
		source           *FileVarsSource
		evaluatedContent []byte
		config           []byte
	)

	BeforeEach(func() {
		source = &FileVarsSource{
			ParamsContent: []byte(`
secret:
  concourse_repo:
    private_key: some-private-key

env: some-env

env-tags: ["speedy"]
`,
			),
		}
	})

	JustBeforeEach(func() {
		var err error
		evaluatedContent, err = source.Evaluate(config)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Evaluate", func() {
		Context("when all of the variables are defined", func() {
			BeforeEach(func() {
				config = []byte(`
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

			It("evaluates params", func() {
				Expect(evaluatedContent).To(MatchYAML([]byte(`
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
				config = []byte(`
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

			It("evaluates the params with the given secrets", func() {
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
		})
	})
})
