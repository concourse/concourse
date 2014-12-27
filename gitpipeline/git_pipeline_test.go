package git_pipeline_test

import (
	"time"

	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A pipeline with git resources", func() {
	It("triggers when the repo changes", func() {
		By("building the initial commit")
		guid1 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid1))

		By("building another commit")
		guid2 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid2))
	})

	It("performs output conditions correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(committedGuid))

		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		By("performing on: [success] outputs on success")
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}, 30*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("performing on: [failure] outputs on failure")
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}, 30*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("not performing on: [success] outputs on failure")
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}, 10*time.Second, 1*time.Second).Should(BeEmpty())
	})
})
