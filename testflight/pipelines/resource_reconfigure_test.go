package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Reconfiguring a resource", func() {
	It("picks up the new configuration immediately", func() {
		guid1, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		guid2, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/reconfiguring.yml",
			"-v", "force_version="+guid1.String(),
		)

		watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).To(gbytes.Say(guid1.String()))

		flyHelper.ReconfigurePipeline(
			pipelineName,
			"-c", "fixtures/reconfiguring.yml",
			"-v", "force_version="+guid2.String(),
		)

		watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).To(gbytes.Say(guid2.String()))
	})
})
