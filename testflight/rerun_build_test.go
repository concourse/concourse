package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
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
		var build *gexec.Session

		BeforeEach(func() {
			By("running on the initial version")
			fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:first-version")
			watch := waitForBuildAndWatch("some-passing-job")
			Eventually(watch).Should(gbytes.Say("first-version"))

			By("running again when there's a new version")
			fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:second-version")
			watch = waitForBuildAndWatch("some-passing-job", "2")
			Eventually(watch).Should(gbytes.Say("second-version"))

			By("confirming that there are no rerun builds")
			build = fly("builds", "-j", inPipeline("some-passing-job"))
			Expect(build).ToNot(gbytes.Say(`1\.1`))
		})

		Context("when creating a rerun of the first build", func() {
			BeforeEach(func() {
				fly("rerun-build", "-j", inPipeline("some-passing-job"), "-b", "1", "-w")
			})

			It("creats a rerun of the first build", func() {
				By("watching the job without specifying a build name")
				watch := waitForBuildAndWatch("some-passing-job")
				Eventually(watch).Should(gbytes.Say("second-version"))

				By("watching the job with rerun build's name")
				watch = waitForBuildAndWatch("some-passing-job", "1.1")
				Eventually(watch).Should(gbytes.Say("first-version"))

				By("confirming the rerun builds apears in build history")
				build = fly("builds", "-j", inPipeline("some-passing-job"))
				Expect(build).To(gbytes.Say(`1\.1`))
			})

			Context("when creating a rerun of the rerun build", func() {
				BeforeEach(func() {
					fly("rerun-build", "-j", inPipeline("some-passing-job"), "-b", "1.1", "-w")
				})

				It("creats a rerun of the first build", func() {
					By("watching the job with rerun build's name")
					watch := waitForBuildAndWatch("some-passing-job", "1.2")
					Eventually(watch).Should(gbytes.Say("first-version"))

					By("confirming the rerun builds apears in build history")
					build = fly("builds", "-j", inPipeline("some-passing-job"))
					Expect(build).To(gbytes.Say(`1\.2`))
				})
			})
		})
	})
})
