package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a resource", func() {

	It("preserved data", func() {
		guid, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/resource-with-params.yml",
			"-v", "unique_version="+guid.String(),
		)

		fly("trigger-job", "-j", inPipeline("without-params"), "-w")
		build := fly("builds", "-p", pipelineName)
		Expect(build).To(gbytes.Say(pipelineName + "/without-params"))

		setPipeline(
			"fixtures/rename-resource-with-params.yml",
			"-v", "unique_version="+guid.String(),
		)

		fly("trigger-job", "-j", inPipeline("without-params"), "-w")
		build = fly("builds", "-p", pipelineName)
		Expect(build).To(gbytes.Say(pipelineName + "/without-params"))
	})
})
