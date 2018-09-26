package pipelines_test

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("serial groups", func() {
	Context("when no inputs are available for one resource", func() {
		BeforeEach(func() {
			hash, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/serial-groups.yml",
				"-v", "hash="+hash.String(),
			)
		})

		It("runs even when another job in the serial group has a pending build", func() {
			pendingBuild, err := team.CreateJobBuild(pipelineName, "some-pending-job")
			Expect(err).NotTo(HaveOccurred())

			watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))

			updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))
		})
	})

	Context("when inputs eventually become available for one resource", func() {
		BeforeEach(func() {
			hash, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			hash2, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/serial-groups-inputs-updated.yml",
				"-v", "hash-1="+hash.String(),
				"-v", "hash-2="+hash2.String(),
			)
		})

		It("is able to run second job with latest inputs", func() {
			pendingBuild, err := team.CreateJobBuild(pipelineName, "some-pending-job")
			Expect(err).NotTo(HaveOccurred())

			By("making a new version, kicking off some-passing-job, and pending some-pending-job")
			guid1, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			found, err := team.CheckResource(pipelineName, "some-resource", atc.Version{"version": guid1.String()})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say(guid1.String()))

			updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))

			By("making another new version, kicking off some-passing-job, and pending some-pending-job")
			guid2, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			found, err = team.CheckResource(pipelineName, "some-resource", atc.Version{"version": guid2.String()})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say(guid2.String()))

			updatedPendingBuild, found, err = client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))

			By("making a version for the other resource, kicking off some-pending-job which should run with newest resource versions")
			guid3, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			found, err = team.CheckResource(pipelineName, "other-resource", atc.Version{"version": guid3.String()})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			watch = flyHelper.Watch(pipelineName, "some-pending-job")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say(guid2.String()))
			Expect(watch).To(gbytes.Say(guid3.String()))

			getPendingBuildStats := func() string {
				updatedPendingBuild, found, err = client.Build(fmt.Sprint(pendingBuild.ID))
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				return updatedPendingBuild.Status
			}
			Eventually(getPendingBuildStats).Should(Equal(string(atc.StatusSucceeded)))
		})
	})
})
