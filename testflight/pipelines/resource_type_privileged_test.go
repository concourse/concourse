package pipelines_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource type in a pipeline config", func() {
	var privileged string

	BeforeEach(func() {
		privileged = "nil"
	})

	JustBeforeEach(func() {
		unique, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-types-privileged.yml",
			"-y", "privileged="+privileged,
			"-v", "unique_config="+unique.String(),
		)
	})

	Context("when the resource type is privileged", func() {
		BeforeEach(func() {
			privileged = "true"
		})

		It("performs 'check', 'get', and 'put' with privileged containers", func() {
			By("running 'get' with a privileged container")
			watch := flyHelper.TriggerJob(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("privileged: true"))

			By("running the resource 'check' with a privileged container")
			versions, _, found, err := team.ResourceVersions(pipelineName, "my-resource", concourse.Page{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(versions[0].Version).To(Equal(atc.Version{
				"version":    "mock",
				"privileged": "true",
			}))

			By("running 'put' with a privileged container")
			watch = flyHelper.TriggerJob(pipelineName, "resource-putter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("pushing in a privileged container"))
		})
	})

	Context("when the custom resource type is not privileged", func() {
		BeforeEach(func() {
			privileged = "false"
		})

		It("performs 'check', 'get', and 'put' with unprivileged containers", func() {
			By("running 'get' with an unprivileged container")
			watch := flyHelper.TriggerJob(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("privileged: false"))

			By("running the resource 'check' with an unprivileged container")
			versions, _, found, err := team.ResourceVersions(pipelineName, "my-resource", concourse.Page{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(versions[0].Version).To(Equal(atc.Version{
				"version": "mock",
			}))

			By("running 'put' with an unprivileged container")
			watch = flyHelper.TriggerJob(pipelineName, "resource-putter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).ToNot(gbytes.Say("running in a privileged container"))
		})
	})
})
