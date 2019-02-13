package topgun_test

import (
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("GlobalStepDefault test", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/add-global-step-timeout.yml")
	})

	Context("global step timeout is set to 5ms", func() {
		It("task step without timeout modifier times out after 5ms", func() {
			By("setting the pipeline that has a build")
			fly.Run("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "hijacked-containers-test")

			By("triggering the build")
			fly.Run("unpause-pipeline", "-p", "hijacked-containers-test")
			buildSession := fly.Start("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
			Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

		})
	})
})
