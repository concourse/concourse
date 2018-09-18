package setpipelinehelpers_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/atc"
	. "github.com/concourse/fly/commands/internal/setpipelinehelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ATC Config", func() {
	Describe("Apply configuration interaction", func() {
		var atcConfig ATCConfig
		BeforeEach(func() {
			atcConfig = ATCConfig{
				SkipInteraction: true,
			}
		})

		Context("when the skip interaction flag has been set to true", func() {
			It("returns true", func() {
				Expect(atcConfig.ApplyConfigInteraction()).To(BeTrue())
			})
		})
	})

	Describe("validating", func() {
		var atcConfig ATCConfig
		var tmpdir string

		BeforeEach(func() {
			atcConfig = ATCConfig{}

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
		})

		AfterEach(func() {
			os.RemoveAll(tmpdir)
		})

		It("validates a good pipeline", func() {
			err := atcConfig.Validate(atc.PathFlag(filepath.Join(tmpdir, "good-pipeline.yml")), nil, nil, nil, false, false)
			Expect(err).To(BeNil())
		})
		It("validates a good pipeline with strict", func() {
			err := atcConfig.Validate(atc.PathFlag(filepath.Join(tmpdir, "good-pipeline.yml")), nil, nil, nil, true, false)
			Expect(err).To(BeNil())
		})
		It("validates a good pipeline with output", func() {
			err := atcConfig.Validate(atc.PathFlag(filepath.Join(tmpdir, "good-pipeline.yml")), nil, nil, nil, true, true)
			Expect(err).To(BeNil())
		})
		It("do not fail validating a pipeline with repeated resource types (probably should but for compat doesn't)", func() {
			err := atcConfig.Validate(atc.PathFlag(filepath.Join(tmpdir, "dupkey-pipeline.yml")), nil, nil, nil, false, false)
			Expect(err).To(BeNil())
		})
		It("fail validating a pipeline with repeated resource types with strict", func() {
			err := atcConfig.Validate(atc.PathFlag(filepath.Join(tmpdir, "dupkey-pipeline.yml")), nil, nil, nil, true, false)
			Expect(err).ToNot(BeNil())
		})
	})
})
