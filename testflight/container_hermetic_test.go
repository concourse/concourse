package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a task that has hermetic set to true", func() {
	It("runs the build", func() {

		setAndUnpausePipeline("fixtures/container_hermetic.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-hermetic-job"), "-w")
		<-watch.Exited

		if config.Runtime == "containerd" {
			By("containerd runtime it should fail to establish a network connection")
			Expect(watch).To(gbytes.Say("1 packets transmitted, 0 packets received, 100% packet loss"))
			Expect(watch.ExitCode()).ToNot(Equal(0))
		} else {
			By("guardian runtime it should succeed in establishing network connection")
			Expect(watch).To(gbytes.Say("1 packets transmitted, 1 packets received, 0% packet loss"))
			Expect(watch.ExitCode()).To(Equal(0))
		}
	})
})
