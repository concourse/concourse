package validatepipelinehelpers_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/fly/commands/internal/validatepipelinehelpers"

	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validate Pipeline", func() {

	Describe("validating", func() {
		var tmpdir string
		var goodPipeline templatehelpers.YamlTemplateWithParams
		var dupkeyPipeline templatehelpers.YamlTemplateWithParams
		var goodAcrossPipeline templatehelpers.YamlTemplateWithParams

		BeforeEach(func() {
			var err error

			tmpdir, err = ioutil.TempDir("", "validate-test")
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

			err = ioutil.WriteFile(
				filepath.Join(tmpdir, "good-across-pipeline.yml"),
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
    across:
    - var: foo_version
      values: ["2.4", "2.5"]
    config:
      platform: linux
      image_resource:
        type: registry-image
        source: {repository: ubuntu, tag: "foo-((foo))"}
      run:
        path: echo
        args: ["Hello, world!"]
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())

			goodPipeline = templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "good-pipeline.yml")), nil, nil, nil, nil)
			dupkeyPipeline = templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "dupkey-pipeline.yml")), nil, nil, nil, nil)
			goodAcrossPipeline = templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "good-across-pipeline.yml")), nil, nil, nil, nil)
		})

		AfterEach(func() {
			os.RemoveAll(tmpdir)
		})

		It("validates a good pipeline", func() {
			err := validatepipelinehelpers.Validate(goodPipeline, false, false, false)
			Expect(err).To(BeNil())
		})
		It("validates a good pipeline with strict", func() {
			err := validatepipelinehelpers.Validate(goodPipeline, true, false, false)
			Expect(err).To(BeNil())
		})
		It("validates a good pipeline with output", func() {
			err := validatepipelinehelpers.Validate(goodPipeline, true, true, false)
			Expect(err).To(BeNil())
		})
		It("do not fail validating a pipeline with repeated resource types (probably should but for compat doesn't)", func() {
			err := validatepipelinehelpers.Validate(dupkeyPipeline, false, false, false)
			Expect(err).To(BeNil())
		})
		It("fail validating a pipeline with repeated resource types with strict", func() {
			err := validatepipelinehelpers.Validate(dupkeyPipeline, true, false, false)
			Expect(err).ToNot(BeNil())
		})
		It("fail validating a pipeline using experimental `across` without the command flag enabling it", func() {
			err := validatepipelinehelpers.Validate(goodAcrossPipeline, false, false, false)
			Expect(err).ToNot(BeNil())
		})
		It("validates a pipeline using experimental `across` when the command flag enabling it is present", func() {
			err := validatepipelinehelpers.Validate(goodAcrossPipeline, false, false, true)
			Expect(err).To(BeNil())
		})
	})
})
