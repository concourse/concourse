package templatehelpers_test

import (
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("YAML Template With Params", func() {

	Describe("validating", func() {
		var tmpdir string
		var goodPipeline templatehelpers.YamlTemplateWithParams
		var dupkeyPipeline templatehelpers.YamlTemplateWithParams

		BeforeEach(func() {
			var err error

			tmpdir, err = ioutil.TempDir("", "fly-test")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(
				filepath.Join(tmpdir, "good-pipeline.yml"),
				[]byte(`---
resource_types:
- name: foo
  type: registry-image
  source:
    repository: foo/foo
- name: bar
  type: registry-image
  source:
    repository: bar/bar
jobs:
- name: hello-world
  plan:
  - task: say-hello
    config:
      platform: linux
      image_resource:
        type: registry-image
        source: {repository: ubuntu}
      run:
        path: echo
        args: ["Hello, world!"]
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(
				filepath.Join(tmpdir, "dupkey-pipeline.yml"),
				[]byte(`---
resource_types:
- name: foo
  type: registry-image
  source:
    repository: foo/foo
- name: bar
  type: registry-image
  source:
    repository: bar/bar
jobs:
- name: hello-world
  plan:
  - task: say-hello
    config:
      platform: linux
      image_resource:
        type: registry-image
        source: {repository: ubuntu}
      run:
        path: echo
        args: ["Hello, world!"]
resource_types:
- name: baz
  type: registry-image
  source:
    repository: baz/baz
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())

			goodPipeline = templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "good-pipeline.yml")), nil, nil, nil)
			dupkeyPipeline = templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "dupkey-pipeline.yml")), nil, nil, nil)
		})

		AfterEach(func() {
			os.RemoveAll(tmpdir)
		})

		It("validates a good pipeline", func() {
			err := goodPipeline.Validate(false, false)
			Expect(err).To(BeNil())
		})
		It("validates a good pipeline with strict", func() {
			err := goodPipeline.Validate(true, false)
			Expect(err).To(BeNil())
		})
		It("validates a good pipeline with output", func() {
			err := goodPipeline.Validate(true, true)
			Expect(err).To(BeNil())
		})
		It("do not fail validating a pipeline with repeated resource types (probably should but for compat doesn't)", func() {
			err := dupkeyPipeline.Validate(false, false)
			Expect(err).To(BeNil())
		})
		It("fail validating a pipeline with repeated resource types with strict", func() {
			err := dupkeyPipeline.Validate(true, false)
			Expect(err).ToNot(BeNil())
		})
	})
})
