package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource config versions", func() {
	BeforeEach(func() {
		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-type-versions.yml",
			"-v", "hash="+hash.String(),
		)
	})

	// This test is for a case where the build inputs and outputs will not be invalidated if the resource config id field on the resource
	// gets updated due to a new version of the custom resource type that it is using.
	Describe("build inputs and outputs are not affected by a change in resource config id", func() {
		It("will run both jobs only once even with a new custom resource type version", func() {
			By("Waiting for a new build when the pipeline is created")
			watch := flyHelper.Watch(pipelineName, "initial-job")
			<-watch.Exited
			Expect(watch).To(gbytes.Say("succeeded"))
			Expect(watch).To(gexec.Exit(0))

			By("Checking the a new version of the custom resource type")
			u, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			newVersion := u.String()

			session := flyHelper.CheckResourceType(
				"-r", pipelineName+"/custom-resource-type",
				"-f", "version:"+newVersion,
			)
			<-session.Exited
			Expect(session.ExitCode()).To(Equal(0))

			By("Triggering a job using the custom type")
			watch = flyHelper.TriggerJob(pipelineName, "passed-job")
			<-watch.Exited
			Expect(watch).To(gbytes.Say("succeeded"))
			Expect(watch).To(gexec.Exit(0))

			By("Using the version  of 'some-resource' consumed upstream")
			builds := flyHelper.Builds(pipelineName, "initial-job")
			Expect(builds).To(HaveLen(1))
		})
	})
})
