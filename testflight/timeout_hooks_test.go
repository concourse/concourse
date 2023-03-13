package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline containing a job with a timeout and hooks", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/timeout_hooks.yml")
	})

	It("runs the failure and ensure hooks", func() {
		watch := spawnFly("trigger-job", "-j", inPipeline("duration-fail-job"), "-w")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("duration fail job on failure"))
		Expect(watch).To(gbytes.Say("duration fail job ensure"))
		Expect(watch).To(gexec.Exit(1))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("duration fail job on success"))
	})
})
