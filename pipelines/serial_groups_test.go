package pipelines_test

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("serial groups", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	Context("when no inputs are available for one resource", func() {
		BeforeEach(func() {
			guidServer = guidserver.Start(client)
			originGitServer = gitserver.Start(client)

			configurePipeline(
				"-c", "fixtures/serial-groups.yml",
				"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		AfterEach(func() {
			guidServer.Stop()
			originGitServer.Stop()
		})

		It("runs even when another job in the serial group has a pending build", func() {
			pendingBuild, err := client.CreateJobBuild(pipelineName, "some-pending-job")
			Expect(err).NotTo(HaveOccurred())

			guid1 := originGitServer.Commit()
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

			updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))
		})
	})

	Context("when inputs eventually become available for one resource", func() {
		BeforeEach(func() {
			guidServer = guidserver.Start(client)
			originGitServer = gitserver.Start(client)

			configurePipeline(
				"-c", "fixtures/serial-groups-inputs-updated.yml",
				"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		AfterEach(func() {
			guidServer.Stop()
			originGitServer.Stop()
		})

		It("is able to run second job with latest inputs", func() {
			pendingBuild, err := client.CreateJobBuild(pipelineName, "some-pending-job")
			Expect(err).NotTo(HaveOccurred())
			By("making a commit master, kicking off some-passing-job, and pending some-pending-job")
			guid1 := originGitServer.Commit()
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

			updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))

			By("making a commit to master again, kicking off some-passing-job, and pending some-pending-job")
			guid2 := originGitServer.Commit()
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid2))

			updatedPendingBuild, found, err = client.Build(fmt.Sprint(pendingBuild.ID))
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))

			By("making a commit to other-branch, kicking off some-pending-job which should run with newest resource versions")
			guid3 := originGitServer.CommitOnBranch("other-branch")
			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid3))

			Eventually(guidServer.ReportingGuids).Should(ContainElement(guid2))

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
