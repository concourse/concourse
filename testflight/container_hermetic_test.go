package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that has hermetic set to true", func() {
	It("runs the build", func() {

		setAndUnpausePipeline("fixtures/container_hermetic.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-hermetic-job"), "-w")
		<-watch.Exited

		if config.Runtime == "containerd" {
			By("containerd runtime it should failed on establish network connection")
			Expect(watch).To(gbytes.Say("failure resolving"))
			Expect(watch).To(gexec.Exit(100)) // can't apt update
		} else {
			By("guardian runtime it should success on curl")
			Expect(watch).To(gbytes.Say("200"))
		}
	})
})
