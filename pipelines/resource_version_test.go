package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource version", func() {
	var (
		guidServer      *guidserver.Server
		originGitServer *gitserver.Server
	)

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)
	})

	AfterEach(func() {
		guidServer.Stop()
		originGitServer.Stop()
	})

	Describe("version: latest", func() {
		It("only runs builds with latest version", func() {
			configurePipeline(
				"-c", "fixtures/simple-trigger.yml",
				"-v", "testflight-helper-image="+guidServerRootfs,
				"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
				"-v", "origin-git-server="+originGitServer.URI(),
			)

			guid1 := originGitServer.Commit()
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

			pausePipeline()

			guid2 := originGitServer.Commit()
			guid3 := originGitServer.Commit()
			guid4 := originGitServer.Commit()

			unpausePipeline()

			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid4))

			Expect(guidServer.ReportingGuids()).NotTo(ContainElement(guid2))
			Expect(guidServer.ReportingGuids()).NotTo(ContainElement(guid3))
		})
	})

	Describe("version: every", func() {
		It("runs builds with every version", func() {
			configurePipeline(
				"-c", "fixtures/resource-version-every.yml",
				"-v", "testflight-helper-image="+guidServerRootfs,
				"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
				"-v", "origin-git-server="+originGitServer.URI(),
			)

			guid1 := originGitServer.Commit()
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

			pausePipeline()

			guid2 := originGitServer.Commit()
			guid3 := originGitServer.Commit()
			guid4 := originGitServer.Commit()

			unpausePipeline()

			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid2))
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid3))
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid4))
		})
	})

	Describe("version: pinned", func() {
		It("only runs builds with the pinned version", func() {
			guid1 := originGitServer.Commit()

			configurePipeline(
				"-c", "fixtures/resource-version-every.yml",
				"-v", "testflight-helper-image="+guidServerRootfs,
				"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
				"-v", "origin-git-server="+originGitServer.URI(),
			)

			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

			pausePipeline()

			guid2 := originGitServer.Commit()
			guid3 := originGitServer.Commit()
			rev := originGitServer.RevParse("master")
			guid4 := originGitServer.Commit()

			reconfigurePipeline(
				"-c", "fixtures/pinned-version.yml",
				"-v", "testflight-helper-image="+guidServerRootfs,
				"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
				"-v", "origin-git-server="+originGitServer.URI(),
				"-v", "git-resource-version="+rev,
			)

			unpausePipeline()

			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid3))

			Expect(guidServer.ReportingGuids()).NotTo(ContainElement(guid2))
			Expect(guidServer.ReportingGuids()).NotTo(ContainElement(guid4))
		})
	})
})
