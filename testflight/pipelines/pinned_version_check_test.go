package pipelines_test

import (
	"fmt"

	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = FDescribe("A resource pinned with a version during initial set of the pipeline", func() {
	Context("when a resource is pinned in the pipeline config before the check is run", func() {
		BeforeEach(func() {
			hash, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/pinned-resource-simple-trigger.yml",
				"-v", "hash="+hash.String(),
				"-v", "pinned_resource_version=v1",
				"-v", "version_config=nil",
			)
		})

		It("should check from the version pinned", func() {
			watch := flyHelper.CheckResource("-r", fmt.Sprintf("%s/some-resource", pipelineName))
			<-watch.Exited
			Expect(watch).To(gexec.Exit(0))

			watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
			<-watch.Exited
			Expect(watch).To(gbytes.Say("v1"))
			Expect(watch.ExitCode()).To(Equal(0))
		})
	})
})
