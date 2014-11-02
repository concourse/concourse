package git_pipeline_test

import (
	"time"

	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A job with a git resource", func() {
	It("triggers when it updates", func() {
		guid1 := gitServer.Commit()

		By("building the initial commit")
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid1))

		guid2 := gitServer.Commit()

		By("building another commit")
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid2))
	})

	It("performs output conditions correctly", func() {
		committedGuid := gitServer.Commit()

		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		By("executing the build")
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(committedGuid))

		By("performing on: [success] outputs on success")
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("performing on: [failure] outputs on failure")
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("not performing on: [success] outputs on failure")
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}, 10*time.Second, 1*time.Second).Should(BeEmpty())
	})
})
