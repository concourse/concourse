package atc_test

import (
	. "github.com/concourse/atc"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = Describe("LoadConfig", func() {
	It("works", func() {
		payload := `---
resources:
- name: some-resource
  type: some-type
  check_every: 10s
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    config:
      run:
        path: ls
      params:
        FOO: true
        BAR: 1
        BAZ: 1.9`

		var c Config
		err := yaml.Unmarshal([]byte(payload), &c)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Jobs[0].Plan[1].TaskConfig.Run.Path).To(Equal("ls"))
	})

	It("converts to yaml and back", func() {
		t := LoadTaskConfig{
			TaskConfig: &TaskConfig{
				Platform: "linux",
			},
		}

		marshalled, err := yaml.Marshal(&t)
		Expect(err).NotTo(HaveOccurred())

		var result LoadTaskConfig
		Expect(yaml.Unmarshal(marshalled, &result)).To(Succeed())
		Expect(result).To(Equal(t))
	})

	It("converts to json and back", func() {
		t := LoadTaskConfig{
			TaskConfig: &TaskConfig{
				Platform: "linux",
			},
		}

		marshalled, err := json.Marshal(&t)
		Expect(err).NotTo(HaveOccurred())

		var result LoadTaskConfig
		Expect(json.Unmarshal(marshalled, &result)).To(Succeed())
		Expect(result).To(Equal(t))
	})

	Describe("LoadTaskConfig", func() {
		Context("when the load key is filled", func() {
			It("populates the LoadConfig", func() {
				yml := `load: foo/bar`

				var t LoadTaskConfig
				Expect(yaml.Unmarshal([]byte(yml), &t)).To(Succeed())

				Expect(t.TaskConfig).To(BeNil())
				Expect(t.LoadConfig).NotTo(BeNil())
				Expect(t.LoadConfig.Path).To(Equal("foo/bar"))
			})
		})

		Context("when it parses directly into a task config", func() {
			It("populates the TaskConfig", func() {
				yml := `platform: linux
image_resource:
  type: docker-image
  source:
    repository: alpine
run:
  path: /bin/bash`

				var t LoadTaskConfig
				Expect(yaml.Unmarshal([]byte(yml), &t)).To(Succeed())

				Expect(t.LoadConfig).To(BeNil())
				Expect(t.TaskConfig).NotTo(BeNil())
				Expect(t.TaskConfig.Platform).To(Equal("linux"))
				Expect(t.TaskConfig.Run.Path).To(Equal("/bin/bash"))

			})
		})

		Context("when the config does not satisfy load or task config", func() {
			It("errors", func() {
				yml := `[]`

				var t LoadTaskConfig
				Expect(yaml.Unmarshal([]byte(yml), &t)).NotTo(Succeed())
			})
		})
	})
})
