package pipelines_test

import (
	"github.com/concourse/concourse/atc"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource caching", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/caching.yml",
		)
	})

	It("does not fetch if there is nothing new", func() {
		someResourceV1, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		cachedResourceV1, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		found, err := team.CheckResource(pipelineName, "some-resource", atc.Version{"version": someResourceV1.String()})
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		found, err = team.CheckResource(pipelineName, "cached-resource", atc.Version{"version": cachedResourceV1.String()})
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("initially fetching twice")
		watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("fetching.*" + someResourceV1.String()))
		Expect(watch).To(gbytes.Say("fetching.*" + cachedResourceV1.String()))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))

		By("coming up with a new version for one resource")
		someResourceV2, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		found, err = team.CheckResource(pipelineName, "some-resource", atc.Version{"version": someResourceV2.String()})
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("hitting the cache for the original version and fetching the new one")
		watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("fetching.*" + someResourceV2.String()))
		Expect(watch).NotTo(gbytes.Say("fetching"))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))
	})
})
