package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource caching", func() {
	It("takes params into account when determining a cache hit", func() {
		guid, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-with-params.yml",
			"-v", "unique_version="+guid.String(),
		)

		watch := flyHelper.TriggerJob(pipelineName, "without-params")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("fetching.*" + guid.String()))

		watch = flyHelper.TriggerJob(pipelineName, "without-params")
		<-watch.Exited
		Expect(watch).ToNot(gbytes.Say("fetching"))

		watch = flyHelper.TriggerJob(pipelineName, "with-params")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("fetching.*" + guid.String()))
	})
})
