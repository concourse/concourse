package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a resource with nested configuration", func() {
	var gitServer *gitserver.Server

	BeforeEach(func() {
		gitServer = gitserver.Start(client)
		gitServer.Commit()

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/config-test.yml",
			"-v", "git-server="+gitServer.URI(),
		)
	})

	AfterEach(func() {
		gitServer.Stop()
	})

	It("works", func() {
		watch := flyHelper.TriggerJob(pipelineName, "config-test")
		<-watch.Exited

		Expect(watch).To(gexec.Exit(0))
	})
})
