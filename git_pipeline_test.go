package testflight_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
)

type GitPipelineTemplate struct {
	GitServers struct {
		Origin   string
		Success  string
		Failure  string
		NoUpdate string
	}

	GuidServerCurlCommand string

	TestflightHelperImage string
}

var _ = Describe("A job with a git resource", func() {
	var (
		gitServer *gitserver.Server

		successGitServer  *gitserver.Server
		failureGitServer  *gitserver.Server
		noUpdateGitServer *gitserver.Server
	)

	BeforeEach(func() {
		guidserver.Start(helperRootfs, gardenClient)

		gitServer = gitserver.Start(helperRootfs, gardenClient)
		gitServer.Commit()

		successGitServer = gitserver.Start(helperRootfs, gardenClient)
		failureGitServer = gitserver.Start(helperRootfs, gardenClient)
		noUpdateGitServer = gitserver.Start(helperRootfs, gardenClient)

		templateData := GitPipelineTemplate{
			TestflightHelperImage: helperRootfs,
			GuidServerCurlCommand: guidserver.CurlCommand(),
		}

		templateData.GitServers.Origin = gitServer.URI()
		templateData.GitServers.Success = successGitServer.URI()
		templateData.GitServers.Failure = failureGitServer.URI()
		templateData.GitServers.NoUpdate = noUpdateGitServer.URI()

		writeATCPipeline("git.yml", templateData)

		atcProcess = ifrit.Envoke(atcRunner)
		Consistently(atcProcess.Wait(), 1*time.Second).ShouldNot(Receive())
	})

	AfterEach(func() {
		gitServer.Stop()
		successGitServer.Stop()
		failureGitServer.Stop()
		noUpdateGitServer.Stop()

		guidserver.Stop(gardenClient)
	})

	It("builds a repo's initial and later commits", func() {
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(HaveLen(1))
		Ω(guidserver.ReportingGuids()).Should(Equal(gitServer.CommittedGuids()))

		gitServer.Commit()

		Eventually(guidserver.ReportingGuids, 2*time.Minute, 10*time.Second).Should(HaveLen(2))
		Ω(guidserver.ReportingGuids()).Should(Equal(gitServer.CommittedGuids()))
	})

	It("performs success outputs when the build succeeds, and failure outputs when the build fails", func() {
		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		// synchronize on the build triggering
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(HaveLen(1))

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
