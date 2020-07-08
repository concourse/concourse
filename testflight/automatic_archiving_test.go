package testflight_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("archive-pipeline", func() {
	var (
		createdPipelineName string

		timeout  = 40 * time.Second
		interval = 5 * time.Second
	)

	BeforeEach(func() {
		createdPipelineName = randomPipelineName()
	})

	JustBeforeEach(func() {
		withFlyTarget(testflightFlyTarget, func() {
			setAndUnpausePipeline(
				"fixtures/archive-pipeline-1.yml",
				"-v", "pipeline_name="+createdPipelineName,
			)

			execS := fly("trigger-job", "-w", "-j", pipelineName+"/sp")
			Expect(execS.Out).To(gbytes.Say("setting pipeline: " + createdPipelineName))
			Expect(execS.Out).To(gbytes.Say("done"))
		})
	})

	Context("when the step is removed from the parent pipeline config", func() {
		It("archives the child pipeline", func() {
			execS := fly("get-pipeline", "-p", createdPipelineName)
			Expect(execS.Out).To(gbytes.Say("normal-job"))

			setPipeline("fixtures/archive-pipeline-2.yml")
			execS = fly("trigger-job", "-w", "-j", pipelineName+"/sp")
			Expect(execS.Out).To(gbytes.Say("succeeded"))

			Eventually(func() *gbytes.Buffer {
				return flyUnsafe("get-pipeline", "-p", createdPipelineName).Err
			}, timeout, interval).Should(gbytes.Say("pipeline not found"), "the pipeline should be archived")
		})
	})

	Context("when the parent pipeline is deleted", func() {
		JustBeforeEach(func() {
			fly("destroy-pipeline", "-n", "-p", createdPipelineName)
		})

		It("archives the child pipeline", func() {
			Eventually(func() *gbytes.Buffer {
				return flyUnsafe("get-pipeline", "-p", createdPipelineName).Err
			}, timeout, interval).Should(gbytes.Say("pipeline not found"), "the pipeline should be archived")
		})
	})

	Context("when the job is removed from the parent pipeline config", func() {
		AfterEach(func() {
			fly("destroy-pipeline", "-n", "-p", createdPipelineName)
		})

		It("archives the child pipeline", func() {
			execS := fly("get-pipeline", "-p", createdPipelineName)
			Expect(execS.Out).To(gbytes.Say("normal-job"))

			setPipeline("fixtures/archive-pipeline-3.yml")

			Eventually(func() *gbytes.Buffer {
				return flyUnsafe("get-pipeline", "-p", createdPipelineName).Err
			}, timeout, interval).Should(gbytes.Say("pipeline not found"), "the pipeline should be archived")
		})
	})
})
