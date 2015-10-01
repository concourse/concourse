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

	It("does not reclone on new commits", func() {
		// We expect to see cloning twice - once for each resource
		watch := flyWatch("some-passing-job", "1")
		Expect(watch).To(gbytes.Say("Cloning into"))
		Expect(watch).To(gbytes.Say("Cloning into"))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))

		originGitServer.Commit()

		// The second time we only expect to clone one resource
		watch = flyWatch("some-passing-job", "2")
		Expect(watch).To(gbytes.Say("Cloning into"))
		Expect(watch).NotTo(gbytes.Say("Cloning into"))
		Expect(watch).To(gbytes.Say("using version of resource found in cache"))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))
	})
})
