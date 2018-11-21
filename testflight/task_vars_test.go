package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var _ = Describe("External Tasks", func() {
	var fixture string

	BeforeEach(func() {
		var err error

		fixture = filepath.Join(tmp, "fixture")

		err = os.MkdirAll(fixture, 0755)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when external task relies on template variables", func() {
		BeforeEach(func() {

			// we are testing an external task with two external variables - ((image_resource_type)) and ((echo_text))
			taskFileContents := `---
platform: linux

image_resource:
  type: ((image_resource_type))
  source: {mirror_self: true}

run:
  path: echo
  args: [((echo_text))]
`
			err := ioutil.WriteFile(
				filepath.Join(fixture, "task.yml"),
				[]byte(taskFileContents),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			taskUnwrapContents := `---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

inputs:
  - name: some-resource

outputs:
  - name: unwrapped-task-resource

run:
  path: sh
  args: ["-c", "cat some-resource/task.yml | sed -e 's/START_VAR/((/' | sed -e 's/END_VAR/))/' > unwrapped-task-resource/task.yml"]
`

			// since we are using create_files() in mock resource and we don't want pipeline resource to
			// contain unresolved variables in "task.yml":((task_content)), we will pre-process task
			// contents and temporarily replace "((" with "START_VAR" and "))" with "END_VAR"
			taskFileContents = strings.Replace(taskFileContents, "((", "START_VAR", -1)
			taskFileContents = strings.Replace(taskFileContents, "))", "END_VAR", -1)

			// then when we run the pipeline itself, it will contain an additional step called 'process-task-definition'
			// to do a backwards conversion to "((" and "))" using taskUnwrapContents
			fly("set-pipeline", "-n", "-p", pipelineName, "-c", "fixtures/task_vars.yml", "-v", "task_content="+taskFileContents+"", "-v", "task_unwrap_content="+taskUnwrapContents+"")

			fly("unpause-pipeline", "-p", pipelineName)
		})

		It("runs pipeline job with external task without error when vars are passed from the pipeline", func() {
			execS := fly("trigger-job", "-w", "-j", pipelineName+"/external-task-success")
			Expect(execS).To(gbytes.Say("Hello World"))
		})

		It("runs external task via fly execute without error when vars are passed from command line using -v", func() {
			execS := flyIn(fixture, "execute", "-c", "task.yml", "-v", "image_resource_type=mock", "-v", "echo_text=Hello World From Command Line")
			Expect(execS).To(gbytes.Say("Hello World From Command Line"))
		})

		It("runs external task via fly execute without error when vars are passed from command line using -l", func() {
			varsContents := `
image_resource_type: mock
echo_text: Hello World From Command Line
`
			err := ioutil.WriteFile(
				filepath.Join(fixture, "vars.yml"),
				[]byte(varsContents),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())
			execS := flyIn(fixture, "execute", "-c", "task.yml", "-l", "vars.yml")
			Expect(execS).To(gbytes.Say("Hello World From Command Line"))
		})

		It("fails pipeline job with external task if it has an uninterpolated variable", func() {
			execS := spawnFly("trigger-job", "-w", "-j", pipelineName+"/external-task-failure")
			<-execS.Exited
			Expect(execS).To(gexec.Exit(2))
			Expect(execS.Out).To(gbytes.Say("Expected to find variables: echo_text"))
		})

		It("should fail external task via fly execute if it has an uninterpolated variable (but it succeeds)", func() {
			// TODO: not sure how to change implementation to fail early on one-off tasks with uninterpolated variables via fly execute
			execS := flyIn(fixture, "execute", "-c", "task.yml", "-v", "image_resource_type=mock")
			Expect(execS).To(gbytes.Say("((echo_text))"))
		})

	})

})
