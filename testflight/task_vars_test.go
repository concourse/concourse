package testflight_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
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
		var taskFileContents string

		BeforeEach(func() {
			// we are testing an external task with two external variables - ((image_resource_type)) and ((echo_text))
			taskFileContents = `---
platform: linux

image_resource:
  type: ((image_resource_type))
  source: {mirror_self: true}

run:
  path: echo
  args: [((echo_text))]
`
		})

		JustBeforeEach(func() {

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

		Context("when required vars are passed from the pipeline", func() {
			It("successfully runs pipeline job with external task", func() {
				execS := fly("trigger-job", "-w", "-j", pipelineName+"/external-task-success")
				Expect(execS).To(gbytes.Say("Hello World"))
			})
		})

		Context("when not all required vars are passed from the pipeline", func() {
			It("fails pipeline job with external task due to an uninterpolated variable", func() {
				execS := spawnFly("trigger-job", "-w", "-j", pipelineName+"/external-task-failure")
				<-execS.Exited
				Expect(execS).To(gexec.Exit(2))
				Expect(execS.Out).To(gbytes.Say("undefined vars: echo_text"))
			})
		})

		Context("when required vars are passed from from command line using -v", func() {
			It("successfully runs external task via fly execute", func() {
				execS := flyIn(fixture, "execute", "-c", "task.yml", "-v", "image_resource_type=mock", "-v", "echo_text=Hello World From Command Line")
				Expect(execS).To(gbytes.Say("Hello World From Command Line"))
			})
		})

		Context("when required vars are passed from from command line using -v", func() {
			It("successfully runs external task via fly execute", func() {
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
		})

		Context("when not all required vars are passed from from command line", func() {
			It("fails external task via fly execute due to an uninterpolated variable", func() {
				execS := spawnFlyIn(fixture, "execute", "-c", "task.yml", "-v", "image_resource_type=mock")
				<-execS.Exited
				Expect(execS).To(gexec.Exit(2))
				Expect(execS.Out).To(gbytes.Say("undefined vars: echo_text"))
			})
		})

		Context("when vars are from load_var", func() {
			It("successfully runs pipeline job with external task", func() {
				execS := fly("trigger-job", "-w", "-j", pipelineName+"/external-task-vars-from-load-var")
				Expect(execS).To(gbytes.Say("bar"))
			})
		})

		Context("when task vars are not used, task should get vars from var_sources", func() {
			BeforeEach(func() {
				taskFileContents = `---
platform: linux

image_resource:
  type: ((image_resource_type))
  source: {mirror_self: true}

run:
  path: echo
  args: [((vs:echo_text))]
`
			})
			It("successfully runs pipeline job with external task", func() {
				execS := fly("trigger-job", "-w", "-j", pipelineName+"/task-var-is-defined-but-task-also-needs-vars-from-var-sources")
				Expect(execS).To(gbytes.Say("text-from-var-source"))
			})
		})
	})

})
