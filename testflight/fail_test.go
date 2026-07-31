package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that always fails", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/fail.yml")
	})

	It("causes the build to fail", func(ctx SpecContext) {
		watch := spawnFly("trigger-job", "-j", inPipeline("failing-job"), "-w")
		Eventually(watch).Should(gexec.Exit(1))
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("failed"))
	}, DefaultSpecTimeout)
})
