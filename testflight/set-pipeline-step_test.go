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
	const (
		pipelineName       = "first-sp"
		secondPipelineName = "second-sp"
	)

	const pipelineContent = `---
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
      team: ((team_name))
      file: some-resource/pipeline.yml
      var_files:
      - some-resource/name.yml
      vars:
        greetings: hello`

	var (
		fixture          string
		currentTeamName  string
		currentFlyTarget string
	)

	BeforeEach(func() {
		fixture = filepath.Join(tmp, "fixture")

		err := os.MkdirAll(fixture, 0755)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := ioutil.WriteFile(
			filepath.Join(fixture, pipelineName+".yml"),
			[]byte(fmt.Sprintf(pipelineContent, secondPipelineName)),
			0755,
		)
		Expect(err).NotTo(HaveOccurred())

		withFlyTarget(currentFlyTarget, func() {
			fly(
				"set-pipeline", "-n",
				"-p", pipelineName,
				"-c", fixture+"/"+pipelineName+".yml",
				"-v", "team_name="+currentTeamName,
			)
			fly("unpause-pipeline", "-p", pipelineName)
		})
	})

	AfterEach(func() {
		withFlyTarget(currentFlyTarget, func() {
			fly("destroy-pipeline", "-n", "-p", pipelineName)
		})
	})

	Context("when setting the current team's pipeline", func() {
		BeforeEach(func() {
			currentFlyTarget = testflightFlyTarget
			currentTeamName = ""
		})

		AfterEach(func() {
			fly("destroy-pipeline", "-n", "-p", secondPipelineName)
		})

		It("sets the other pipeline", func() {
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

	Context("when setting another team's pipeline from the main team", func() {
		BeforeEach(func() {
			currentFlyTarget = adminFlyTarget
			currentTeamName = teamName
		})

		It("sets the other pipeline", func() {
			By("second pipeline should initially not exist")
			withFlyTarget(testflightFlyTarget, func() {
				execS := spawnFly("get-pipeline", "-p", secondPipelineName)
				<-execS.Exited
				Expect(execS).To(gexec.Exit(1))
				Expect(execS.Err).To(gbytes.Say("pipeline not found"))
			})

			By("set-pipeline step should succeed")
			withFlyTarget(adminFlyTarget, func() {
				execS := fly("trigger-job", "-w", "-j", pipelineName+"/sp")
				Expect(execS.Out).To(gbytes.Say("setting pipeline: second-sp"))
				Expect(execS.Out).To(gbytes.Say("done"))
			})

			By("should trigger the second pipeline job successfully")
			withFlyTarget(testflightFlyTarget, func() {
				execS := fly("trigger-job", "-w", "-j", secondPipelineName+"/normal-job")
				Expect(execS.Out).To(gbytes.Say("hello world"))
			})
		})

		AfterEach(func() {
			withFlyTarget(testflightFlyTarget, func() {
				fly("destroy-pipeline", "-n", "-p", secondPipelineName)
			})
		})
	})

	Context("when setting the main team's pipeline from a normal team", func() {
		BeforeEach(func() {
			currentFlyTarget = testflightFlyTarget
			currentTeamName = "main"
		})

		It("fails to set the other pipeline", func() {
			By("second pipeline should initially not exist")
			withFlyTarget(adminFlyTarget, func() {
				execS := spawnFly("get-pipeline", "-p", secondPipelineName)
				<-execS.Exited
				Expect(execS).To(gexec.Exit(1))
				Expect(execS.Err).To(gbytes.Say("pipeline not found"))
			})

			By("set-pipeline step should fail")
			withFlyTarget(testflightFlyTarget, func() {
				execS := spawnFly("trigger-job", "-w", "-j", pipelineName+"/sp")
				<-execS.Exited
				Expect(execS).To(gexec.Exit(2))
				Expect(execS.Out).To(gbytes.Say("only main team can set another team's pipeline"))
				Expect(execS.Out).To(gbytes.Say("errored"))
			})

			By("second pipeline should still not exist")
			withFlyTarget(adminFlyTarget, func() {
				execS := spawnFly("get-pipeline", "-p", secondPipelineName)
				<-execS.Exited
				Expect(execS).To(gexec.Exit(1))
				Expect(execS.Err).To(gbytes.Say("pipeline not found"))
			})
		})
	})
})
