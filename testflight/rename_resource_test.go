package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a resource", func() {

	It("preserves version history", func() {
		setAndUnpausePipeline("fixtures/resource-with-versions.yml")

		guid1 := newMockVersion("some-resource", "guid1")
		guid2 := newMockVersion("some-resource", "guid2")
		guid3 := newMockVersion("some-resource", "guid3")

		fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		build := fly("builds", "-p", pipelineName)
		Expect(build).To(gbytes.Say(pipelineName + "/some-passing-job"))

		setPipeline("fixtures/rename-resource.yml")

		fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		build = fly("builds", "-p", pipelineName)
		Expect(build).To(gbytes.Say(pipelineName + "/some-passing-job"))

		resourceVersions := fly("resource-versions", "-r", inPipeline("some-new-resource"))
		Expect(resourceVersions).To(gbytes.Say(guid3))
		Expect(resourceVersions).To(gbytes.Say(guid2))
		Expect(resourceVersions).To(gbytes.Say(guid1))
	})
})
