package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("image resource caching", func() {
	var version string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		version = u.String()

		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/image-resource-with-params.yml",
			"-v", "initial_version="+version,
			"-v", "hash="+hash.String(),
		)
	})

	It("gets the image resource from the cache based on given params", func() {
		By("triggering the resource get with params")
		watch := flyHelper.TriggerJob(pipelineName, "with-params")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).To(gbytes.Say(`fetching.*` + version))

		By("triggering the task with image resource with params")
		watch = flyHelper.TriggerJob(pipelineName, "image-resource-with-params")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).ToNot(gbytes.Say("fetching"))

		By("triggering the task with image resource without params")
		watch = flyHelper.TriggerJob(pipelineName, "image-resource-without-params")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).To(gbytes.Say(`fetching.*` + version))

		By("triggering the resource get without params")
		watch = flyHelper.TriggerJob(pipelineName, "without-params")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).ToNot(gbytes.Say(`fetching.*` + version))
	})
})
