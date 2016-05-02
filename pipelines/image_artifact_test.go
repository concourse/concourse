package pipelines_test

import (
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a task that produces outputs", func() {
	var guidServer *guidserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient) // TODO: remove?

		configurePipeline(
			"-c", "fixtures/image-artifact.yml",
		)
	})

	AfterEach(func() {
		guidServer.Stop()
	})

	It("uses the specified image artifact to run the task", func() {
		watch := flyWatch("artifact-test")
		Expect(watch).To(gbytes.Say("/bin/bash"))
	})
})
