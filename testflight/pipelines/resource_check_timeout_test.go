package pipelines_test

import (
	"fmt"

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
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-check-timeouts.yml",
			"-v", "check_delay="+checkDelay.String(),
		)
	})

	Context("when check script times out", func() {
		BeforeEach(func() {
			checkDelay = time.Minute
		})

		It("prints an error and cancels the job", func() {
			watch := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
			<-watch.Exited
			Expect(watch).To(gexec.Exit(1))
			Expect(watch.Err).To(gbytes.Say("Timed out after 10s while checking for new versions - perhaps increase your resource check timeout?"))

			time.Sleep(40 * time.Second)

			hijack := flyHelper.Hijack("-c", fmt.Sprintf("%s/my-resource", pipelineName), "cat", "/tmp/some-random-file")
			<-hijack.Exited
			Expect(hijack).NotTo(gbytes.Say("should not get here"))
		})
	})

	Context("when check script finishes before timeout", func() {
		BeforeEach(func() {
			checkDelay = time.Second
		})

		It("succeeds if the check script returns before the timeout", func() {
			watch := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
			<-watch.Exited
			Expect(watch).To(gexec.Exit(0))

			hijack := flyHelper.Hijack("-c", fmt.Sprintf("%s/my-resource", pipelineName), "cat", "/tmp/some-random-file")
			<-hijack.Exited
		})
	})
})
