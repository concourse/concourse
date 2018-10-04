package testflight_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A resource check which times out", func() {
	var checkDelay time.Duration

	BeforeEach(func() {
		checkDelay = 0
	})

	JustBeforeEach(func() {
		setAndUnpausePipeline(
			"fixtures/resource-check-timeouts.yml",
			"-v", "check_delay="+checkDelay.String(),
		)
	})

	Context("when check script times out", func() {
		BeforeEach(func() {
			checkDelay = time.Minute
		})

		It("prints an error and cancels the check", func() {
			checkS := spawnFly("check-resource", "-r", inPipeline("my-resource"))
			<-checkS.Exited
			Expect(checkS).To(gexec.Exit(1))
			Expect(checkS.Err).To(gbytes.Say("Timed out after 10s while checking for new versions - perhaps increase your resource check timeout?"))
		})
	})

	Context("when check script finishes before timeout", func() {
		BeforeEach(func() {
			checkDelay = time.Second
		})

		It("succeeds", func() {
			fly("check-resource", "-r", inPipeline("my-resource"))
		})
	})
})
