package git_pipeline_test

import (
	"time"

	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A job with a git resource", func() {
	It("builds a repo's initial and later commits", func() {
		guid1 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid1))

		guid2 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 2*time.Minute, 10*time.Second).Should(ContainElement(guid2))
	})

	It("performs success outputs when the build succeeds, and failure outputs when the build fails", func() {
		committedGuid := gitServer.Commit()

		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		// synchronize on the build triggering
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(committedGuid))

		// should have eventually promoted
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		// should have promoted to failure branch because of on: [failure]
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		// should *not* have promoted to no-update branch
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}, 10*time.Second, 1*time.Second).Should(BeEmpty())
	})
})
