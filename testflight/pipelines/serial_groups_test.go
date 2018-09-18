package pipelines_test

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("serial groups", func() {
	var originGitServer *gitserver.Server

	Context("when no inputs are available for one resource", func() {
		BeforeEach(func() {
			originGitServer = gitserver.Start(client)

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/serial-groups.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		AfterEach(func() {
			originGitServer.Stop()
		})

		It("runs even when another job in the serial group has a pending build", func() {
			pendingBuild, err := team.CreateJobBuild(pipelineName, "some-pending-job")
			Expect(err).NotTo(HaveOccurred())

			guid1 := originGitServer.Commit()
			watch := flyHelper.Watch(pipelineName, "some-passing-job")

			Eventually(watch).Should(gbytes.Say(guid1))

			updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))
		})
	})

	Context("when inputs eventually become available for one resource", func() {
		BeforeEach(func() {
			originGitServer = gitserver.Start(client)

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/serial-groups-inputs-updated.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		AfterEach(func() {
			originGitServer.Stop()
		})

		It("is able to run second job with latest inputs", func() {
			pendingBuild, err := team.CreateJobBuild(pipelineName, "some-pending-job")
			Expect(err).NotTo(HaveOccurred())

			By("making a commit master, kicking off some-passing-job, and pending some-pending-job")
			guid1 := originGitServer.Commit()
			watch := flyHelper.Watch(pipelineName, "some-passing-job")
			Eventually(watch).Should(gbytes.Say(guid1))

			updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))

			By("making a commit to master again, kicking off some-passing-job, and pending some-pending-job")
			guid2 := originGitServer.Commit()
			watch = flyHelper.Watch(pipelineName, "some-passing-job", "2")
			Eventually(watch).Should(gbytes.Say(guid2))

			updatedPendingBuild, found, err = client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))

			By("making a commit to other-branch, kicking off some-pending-job which should run with newest resource versions")
			guid3 := originGitServer.CommitOnBranch("other-branch")
			watch = flyHelper.Watch(pipelineName, "some-pending-job")
			Eventually(watch).Should(gbytes.Say(guid3))

			Eventually(watch).Should(gbytes.Say(guid2))

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
