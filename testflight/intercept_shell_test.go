package testflight_test

import (
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("fly intercept default shell", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/wait-for-intercept.yml")
	})

	const testBash = `[ -n "$BASH_VERSION" ] && echo "yes bash" || echo "no bash"` + "\n"

	Context("when the container has bash", func() {
		It("defaults to bash", func() {
			By("triggering the build")
			wait := spawnFly("trigger-job", "-w", "-j", inPipeline("wait"))
			Eventually(wait).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("intercepting the container")
			pr, pw := io.Pipe()
			session := spawnFlyOpts(withStdin(pr))("intercept", "-j", inPipeline("wait"), "-s", "wait-for-intercept")

			By("checking shell is bash")
			pw.Write([]byte(testBash))
			Eventually(session, 10*time.Second).Should(gbytes.Say("yes bash"))

			session.Kill()
		})
	})

	Context("when the container does not have bash", func() {
		It("falls back to sh", func() {
			By("triggering the build")
			wait := spawnFly("trigger-job", "-w", "-j", inPipeline("wait"))
			Eventually(wait).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("removing bash")
			fly("intercept", "-j", pipelineName+"/wait", "-s", "wait-for-intercept", "--", "rm", "-f", "/bin/bash")

			By("intercepting the container")
			pr, pw := io.Pipe()
			session := spawnFlyOpts(withStdin(pr))("intercept", "-j", inPipeline("wait"), "-s", "wait-for-intercept")
			Eventually(session.Err).Should(gbytes.Say(`Couldn't find "bash".*retrying with "sh"`))

			By("checking shell is not bash")
			pw.Write([]byte(testBash))
			Eventually(session, 10*time.Second).Should(gbytes.Say("no bash"))

			session.Kill()
		})
	})
})
