package git_pipeline_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("We shouldn't reclone cached resources", func() {
	var originGitServer *gitserver.Server
	var cachedGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		cachedGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/caching.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
			"-v", "cached-git-server="+cachedGitServer.URI(),
		)
		originGitServer.Commit()
		cachedGitServer.Commit()
	})

	AfterEach(func() {
		originGitServer.Stop()
		cachedGitServer.Stop()
	})

	FIt("does not reclone on new commits", func() {
		// We expect to see cloning twice - once for each resource
		watch := flyWatch("some-passing-job", "1")
		Ω(watch).Should(gbytes.Say("Cloning into"))
		Ω(watch).Should(gbytes.Say("Cloning into"))
		Ω(watch).Should(gbytes.Say("succeeded"))
		Ω(watch).Should(gexec.Exit(0))

		originGitServer.Commit()

		// The second time we only expect to clone one resource
		watch = flyWatch("some-passing-job", "2")
		Ω(watch).Should(gbytes.Say("Cloning into"))
		Ω(watch).ShouldNot(gbytes.Say("Cloning into"))
		Ω(watch).Should(gbytes.Say("using version of resource found in cache"))
		Ω(watch).Should(gbytes.Say("succeeded"))
		Ω(watch).Should(gexec.Exit(0))
	})
})
