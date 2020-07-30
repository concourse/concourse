package testflight_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("set-pipeline Step", func() {
	var (
		createdPipelineName string
		currentTeamName     string
		currentFlyTarget    string
	)

	BeforeEach(func() {
		createdPipelineName = randomPipelineName()
	})

	JustBeforeEach(func() {
		withFlyTarget(currentFlyTarget, func() {
			setAndUnpausePipeline(
				"fixtures/set-pipeline.yml",
				"-v", "team_name="+currentTeamName,
				"-v", "pipeline_name="+createdPipelineName,
			)
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
			fly("destroy-pipeline", "-n", "-p", createdPipelineName)
		})

		It("sets the other pipeline", func() {
			By("second pipeline should initially not exist")
			execS := spawnFly("get-pipeline", "-p", createdPipelineName)
			<-execS.Exited
			Expect(execS).To(gexec.Exit(1))
			Expect(execS.Err).To(gbytes.Say("pipeline not found"))

			By("set-pipeline step should succeed")
			execS = fly("trigger-job", "-w", "-j", pipelineName+"/sp")
			Expect(execS.Out).To(gbytes.Say("setting pipeline: " + createdPipelineName))
			Expect(execS.Out).To(gbytes.Say("done"))

			By("should trigger the second pipeline job successfully")
			execS = fly("trigger-job", "-w", "-j", createdPipelineName+"/normal-job")
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
				execS := spawnFly("get-pipeline", "-p", createdPipelineName)
				<-execS.Exited
				Expect(execS).To(gexec.Exit(1))
				Expect(execS.Err).To(gbytes.Say("pipeline not found"))
			})

			By("set-pipeline step should succeed")
			withFlyTarget(adminFlyTarget, func() {
				execS := fly("trigger-job", "-w", "-j", pipelineName+"/sp")
				Expect(execS.Out).To(gbytes.Say("setting pipeline: " + createdPipelineName))
				Expect(execS.Out).To(gbytes.Say("done"))
			})

			By("should trigger the second pipeline job successfully")
			withFlyTarget(testflightFlyTarget, func() {
				execS := fly("trigger-job", "-w", "-j", createdPipelineName+"/normal-job")
				Expect(execS.Out).To(gbytes.Say("hello world"))
			})
		})

		AfterEach(func() {
			withFlyTarget(testflightFlyTarget, func() {
				fly("destroy-pipeline", "-n", "-p", createdPipelineName)
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
				execS := spawnFly("get-pipeline", "-p", createdPipelineName)
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
				execS := spawnFly("get-pipeline", "-p", createdPipelineName)
				<-execS.Exited
				Expect(execS).To(gexec.Exit(1))
				Expect(execS.Err).To(gbytes.Say("pipeline not found"))
			})
		})
	})

	Context("set self", func() {
		BeforeEach(func() {
			currentFlyTarget = testflightFlyTarget
			currentTeamName = ""
			createdPipelineName = "self"
		})

		It("set the other pipeline", func() {
			By("set-pipeline step should succeed")
			execS := fly("trigger-job", "-w", "-j", pipelineName+"/sp")
			Expect(execS.Out).To(gbytes.Say(fmt.Sprintf("setting pipeline: %s", pipelineName)))
			Expect(execS.Out).To(gbytes.Say("done"))

			By("should trigger the pipeline job successfully")
			execS = fly("trigger-job", "-w", "-j", pipelineName+"/normal-job")
			Expect(execS.Out).To(gbytes.Say("hello world"))
		})
	})
})
