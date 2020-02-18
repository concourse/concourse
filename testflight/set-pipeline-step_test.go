package testflight_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"os"
	"path/filepath"
)

var _ = Describe("set-pipeline Step", func() {

	const pipeline_content = `---
resources:
- name: some-resource
  type: mock
  source:
    create_files:
      pipeline.yml: |
        ---
        jobs:
        - name: normal-job
          public: true
          plan:
          - task: a-task
            config:
              platform: linux
              image_resource:
                type: mock
                source: {mirror_self: true}
              run:
                path: echo
                args: ["hello world"]
      name.yml: |
        ---
        name: somebody

jobs:
- name: sp
  public: true
  plan:
    - get: some-resource
    - set_pipeline: %s
      file: some-resource/pipeline.yml
      var_files:
      - some-resource/name.yml
      vars:
        greetings: hello
`

	var fixture string

	BeforeEach(func() {
		var err error

		fixture = filepath.Join(tmp, "fixture")

		err = os.MkdirAll(fixture, 0755)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("set other pipeline", func() {
		var secondPipelineName string
		BeforeEach(func() {
			pipelineName = "first-sp"
			secondPipelineName = "second-sp"

			err := ioutil.WriteFile(
				filepath.Join(fixture, pipelineName+".yml"),
				[]byte(fmt.Sprintf(pipeline_content, secondPipelineName)),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			fly("set-pipeline", "-n", "-p", pipelineName, "-c", fixture+"/"+pipelineName+".yml")
			fly("unpause-pipeline", "-p", pipelineName)
		})

		AfterEach(func() {
			fly("destroy-pipeline", "-n", "-p", secondPipelineName)
		})

		It("set the other pipeline", func() {
			By("second pipeline should initially not exist")
			execS := spawnFly("get-pipeline", "-p", secondPipelineName)
			<-execS.Exited
			Expect(execS).To(gexec.Exit(1))
			Expect(execS.Err).To(gbytes.Say("pipeline not found"))

			By("set-pipeline step should succeed")
			execS = fly("trigger-job", "-w", "-j", pipelineName+"/sp")
			Expect(execS.Out).To(gbytes.Say("setting pipeline: second-sp"))
			Expect(execS.Out).To(gbytes.Say("done"))

			By("should trigger the second pipeline job successfully")
			execS = fly("trigger-job", "-w", "-j", secondPipelineName+"/normal-job")
			Expect(execS.Out).To(gbytes.Say("hello world"))
		})
	})
})
