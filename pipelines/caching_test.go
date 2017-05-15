package pipelines_test

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
		originGitServer = gitserver.Start(client)
		cachedGitServer = gitserver.Start(client)

		flyHelper.ConfigurePipeline(
			pipelineName,
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
		By("initially cloning twice")
		watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Cloning into"))
		Expect(watch).To(gbytes.Say("Cloning into"))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))

		originGitServer.Commit()

		By("hitting the cache for the original version and fetching the new one")
		watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Cloning into"))
		Expect(watch).NotTo(gbytes.Say("Cloning into"))
		Expect(watch).To(gbytes.Say("using version of resource found in cache"))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))
	})
})
