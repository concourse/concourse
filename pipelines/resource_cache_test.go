package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("[#139960779] resource caching", func() {
	var (
		originGitServer *gitserver.Server
	)

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("gets the resource from the cache based on given params", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-with-params.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		originGitServer.Commit()

		watch := flyHelper.TriggerJob(pipelineName, "without-params")
		<-watch.Exited

		watch = flyHelper.TriggerJob(pipelineName, "without-params")
		<-watch.Exited
		Expect(watch).ToNot(gbytes.Say("Cloning"))

		watch = flyHelper.TriggerJob(pipelineName, "with-params")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Cloning"))
	})
})
