package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Intercepting containers", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/wait-for-intercept.yml")
	})

	It("is limited to the team that owns the containers", func() {
		By("triggering the build")
		wait := spawnFly("trigger-job", "-w", "-j", inPipeline("wait"))
		Eventually(wait).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

		By("demonstrating we can hijack into all of the containers")
		handles := []string{}
		for _, row := range flyTable("containers") {
			if row["pipeline"] == pipelineName && row["job"] == "wait" {
				fly("intercept", "--handle", row["handle"], "hostname")
				handles = append(handles, row["handle"])
			}
		}

		By("demonstrating that the other team cannot intercept any of the containers")
		for _, handle := range handles {
			withFlyTarget(testflightGuestFlyTarget, func() {
				intercept := spawnFly("intercept", "--handle", handle, "hostname")
				<-intercept.Exited
				Expect(intercept.ExitCode()).ToNot(Equal(0))
				Expect(intercept.Err).To(gbytes.Say("no containers matched the given handle id"))
			})
		}

		By("stopping the build")
		fly("intercept", "-j", inPipeline("wait"), "-s", "wait-for-intercept", "touch", "/tmp/stop-waiting")

		<-wait.Exited
		Expect(wait).To(gbytes.Say("done"))
	})
})
