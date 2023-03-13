package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with an input with trigger: true", func() {
	var hash string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/simple-trigger.yml", "-v", "hash="+hash)
	})

	It("triggers when the resource changes", func() {
		By("running on the initial version")
		fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:first-version")
		watch := waitForBuildAndWatch("some-passing-job")
		Eventually(watch).Should(gbytes.Say("first-version"))

		By("running again when there's a new version")
		fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:second-version")
		watch = waitForBuildAndWatch("some-passing-job", "2")
		Eventually(watch).Should(gbytes.Say("second-version"))
	})
})
