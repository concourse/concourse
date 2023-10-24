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
			By("containerd runtime it should failed on establish network connection")
			Expect(watch).To(gbytes.Say("timed out"))
			Expect(watch.ExitCode()).ToNot(Equal(0))
		} else {
			By("guardian runtime it should success establish network connection")
			Expect(watch).To(gbytes.Say("saved"))
			Expect(watch.ExitCode()).To(Equal(0))
		}
	})
})
