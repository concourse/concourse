package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource-types checks", func() {
	BeforeEach(func() {
		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-types.yml",
			"-v", "hash="+hash.String(),
		)
	})

	It("can check the resource-type", func() {
		watch := flyHelper.CheckResourceType("-r", pipelineName+"/custom-resource-type")
		Eventually(watch).Should(gbytes.Say("checked 'custom-resource-type'"))
		Eventually(watch).Should(gexec.Exit(0))
	})

	Context("when there is a new version", func() {
		var newVersion string

		BeforeEach(func() {
			u, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			newVersion = u.String()

			session := flyHelper.CheckResourceType(
				"-r", pipelineName+"/custom-resource-type",
				"-f", "version:"+newVersion,
			)
			<-session.Exited
			Expect(session.ExitCode()).To(Equal(0))
		})

		It("uses the updated resource type", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-imager")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("MIRRORED_VERSION=" + newVersion))
		})
	})

	It("reports that resource-type is not found if it doesn't exist", func() {
		watch := flyHelper.CheckResourceType("-r", pipelineName+"/nonexistent-resource-type")
		Eventually(watch.Err).Should(gbytes.Say("resource-type 'nonexistent-resource-type' not found"))
		Eventually(watch).Should(gexec.Exit(1))
	})

	It("fails when resource-type check fails", func() {
		watch := flyHelper.CheckResourceType("-r", pipelineName+"/failing-custom-resource-type")
		Eventually(watch.Err).Should(gbytes.Say("check failed"))
		Eventually(watch.Err).Should(gbytes.Say("im totally failing to check"))
		Eventually(watch).Should(gexec.Exit(1))
	})
})
