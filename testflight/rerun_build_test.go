package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Rerunning a build", func() {
	var hash string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/simple-trigger.yml", "-v", "hash="+hash)
	})

	Context("when there are two previous builds", func() {
		BeforeEach(func() {
			By("running on the initial version")
			fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:first-version")
			watch := waitForBuildAndWatch("some-passing-job")
			Eventually(watch).Should(gbytes.Say("first-version"))

			By("running again when there's a new version")
			fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:second-version")
			watch = waitForBuildAndWatch("some-passing-job", "2")
			Eventually(watch).Should(gbytes.Say("second-version"))
		})

		It("creates a rerun of the first build", func() {
			By("confirming that there are no rerun builds")
			build := fly("builds", "-j", inPipeline("some-passing-job"))
			Expect(build).ToNot(gbytes.Say(`1\.1`))

			fly("rerun-build", "-j", inPipeline("some-passing-job"), "-b", "1", "-w")
			watch := waitForBuildAndWatch("some-passing-job")
			Eventually(watch).Should(gbytes.Say("first-version"))

			By("running again to see if the rerun build appeared")
			build = fly("builds", "-j", inPipeline("some-passing-job"))
			Expect(build).To(gbytes.Say(`1\.1`))
		})

		Context("when there is a rerun build", func() {
			BeforeEach(func() {
				fly("rerun-build", "-j", inPipeline("some-passing-job"), "-b", "1", "-w")
				watch := waitForBuildAndWatch("some-passing-job")
				Eventually(watch).Should(gbytes.Say("first-version"))

				By("running again to see if the rerun build appeared")
				build := fly("builds", "-j", inPipeline("some-passing-job"))
				Expect(build).To(gbytes.Say(`1\.1`))
			})

			It("rerunning a rerun will create a rerun of the first build", func() {
				By("confirming that there is a rerun build")
				build := fly("builds", "-j", inPipeline("some-passing-job"))
				Expect(build).To(gbytes.Say(`1\.1`))

				fly("rerun-build", "-j", inPipeline("some-passing-job"), "-b", "1.1", "-w")
				watch := waitForBuildAndWatch("some-passing-job")
				Eventually(watch).Should(gbytes.Say("first-version"))

				By("running again to see if the rerun build appeared")
				build = fly("builds", "-j", inPipeline("some-passing-job"))
				Expect(build).To(gbytes.Say(`1\.2`))
			})
		})
	})
})
