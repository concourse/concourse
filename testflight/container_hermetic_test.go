package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that has hermetic set to true", func() {
	It("runs the build", func(ctx SpecContext) {

		setAndUnpausePipeline("fixtures/container_hermetic.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-hermetic-job"), "-w")
		Eventually(watch).Should(gexec.Exit())

		if config.Runtime == "containerd" {
			By("containerd runtime it should fail to establish a network connection")
			Expect(watch).To(gbytes.Say("1 packets transmitted, 0 packets received, 100% packet loss"))
			Expect(watch).To(gexec.Exit(1))
		} else {
			By("guardian runtime it should succeed in establishing network connection")
			Expect(watch).To(gbytes.Say("1 packets transmitted, 1 packets received, 0% packet loss"))
			Expect(watch).To(gexec.Exit(0))
		}
	}, DefaultSpecTimeout)
})
