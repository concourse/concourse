package pipelines_test

import (
	"fmt"
	"strings"

	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource version", func() {
	var (
		originGitServer *gitserver.Server
	)

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	Describe("when the version is not pinned on the resource", func() {
		Describe("version: latest", func() {
			It("only runs builds with latest version", func() {
				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/simple-trigger.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
				)

				guid1 := originGitServer.Commit()
				watch := flyHelper.Watch(pipelineName, "some-passing-job")
				Eventually(watch).Should(gbytes.Say(guid1))

				flyHelper.PausePipeline(pipelineName)

				originGitServer.Commit()
				originGitServer.Commit()
				guid4 := originGitServer.Commit()

				flyHelper.UnpausePipeline(pipelineName)

				watch = flyHelper.Watch(pipelineName, "some-passing-job", "2")
				Eventually(watch).Should(gbytes.Say(guid4))
				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "3")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})

		Describe("version: every", func() {
			It("runs builds with every version", func() {
				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-version-every.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
				)

				guid1 := originGitServer.Commit()
				watch := flyHelper.Watch(pipelineName, "some-passing-job")
				Eventually(watch).Should(gbytes.Say(guid1))

				flyHelper.PausePipeline(pipelineName)

				guid2 := originGitServer.Commit()
				guid3 := originGitServer.Commit()
				guid4 := originGitServer.Commit()

				flyHelper.UnpausePipeline(pipelineName)

				watch = flyHelper.Watch(pipelineName, "some-passing-job", "2")
				Eventually(watch).Should(gbytes.Say(guid2))
				watch = flyHelper.Watch(pipelineName, "some-passing-job", "3")
				Eventually(watch).Should(gbytes.Say(guid3))
				watch = flyHelper.Watch(pipelineName, "some-passing-job", "4")
				Eventually(watch).Should(gbytes.Say(guid4))
			})
		})

		Describe("version: pinned", func() {
			It("only runs builds with the pinned version", func() {
				guid1 := originGitServer.Commit()

				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-version-every.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
				)

				watch := flyHelper.Watch(pipelineName, "some-passing-job")
				Eventually(watch).Should(gbytes.Say(guid1))

				flyHelper.PausePipeline(pipelineName)

				originGitServer.Commit()
				guid3 := originGitServer.Commit()
				rev := originGitServer.RevParse("master")
				originGitServer.Commit()

				flyHelper.ReconfigurePipeline(
					pipelineName,
					"-c", "fixtures/pinned-version.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
					"-v", "git-resource-version="+rev,
				)

				flyHelper.UnpausePipeline(pipelineName)

				watch = flyHelper.Watch(pipelineName, "some-passing-job", "2")
				Eventually(watch).Should(gbytes.Say(guid3))
				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "3")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})
	})

	Describe("when the version is pinned on the resource", func() {
		Describe("version: latest", func() {
			It("only runs builds with pinned version", func() {
				originGitServer.Commit()
				rev := originGitServer.RevParse("master")
				guid2 := originGitServer.Commit()
				rev2 := strings.TrimSpace(originGitServer.RevParse("master"))
				originGitServer.Commit()

				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/pinned-resource-simple-trigger.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
					"-v", "pinned-resource-version="+rev2,
				)
				flyHelper.UnpausePipeline(pipelineName)

				checkResource := flyHelper.CheckResource("-r", fmt.Sprintf("%s/some-git-resource", pipelineName), "-f", fmt.Sprintf("ref:%s", rev))
				<-checkResource.Exited
				Expect(checkResource.ExitCode()).To(Equal(0))
				Expect(checkResource).To(gbytes.Say("checked 'some-git-resource'"))

				watch := flyHelper.Watch(pipelineName, "some-passing-job", "1")
				Eventually(watch).Should(gbytes.Say(guid2))
				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "2")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})

		Describe("version: every", func() {
			It("runs builds with pinned version", func() {
				originGitServer.Commit()
				rev := strings.TrimSpace(originGitServer.RevParse("master"))
				guid2 := originGitServer.Commit()
				rev2 := strings.TrimSpace(originGitServer.RevParse("master"))
				originGitServer.Commit()

				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/pinned-resource-version-every.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
					"-v", "pinned-resource-version="+rev2,
				)

				flyHelper.UnpausePipeline(pipelineName)

				checkResource := flyHelper.CheckResource("-r", fmt.Sprintf("%s/some-git-resource", pipelineName), "-f", fmt.Sprintf("ref:%s", rev))
				<-checkResource.Exited
				Expect(checkResource.ExitCode()).To(Equal(0))
				Expect(checkResource).To(gbytes.Say("checked 'some-git-resource'"))

				watch := flyHelper.Watch(pipelineName, "some-passing-job", "1")
				Eventually(watch).Should(gbytes.Say(guid2))
				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "2")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})

		Describe("version: pinned", func() {
			It("only runs builds with the pinned version", func() {
				originGitServer.Commit()
				rev := strings.TrimSpace(originGitServer.RevParse("master"))
				guid2 := originGitServer.Commit()
				rev2 := strings.TrimSpace(originGitServer.RevParse("master"))
				originGitServer.Commit()
				rev3 := strings.TrimSpace(originGitServer.RevParse("master"))
				originGitServer.Commit()

				flyHelper.ReconfigurePipeline(
					pipelineName,
					"-c", "fixtures/pinned-resource-pinned-version.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
					"-v", "pinned-resource-version="+rev2,
					"-v", "git-resource-version="+rev3,
				)

				flyHelper.UnpausePipeline(pipelineName)

				checkResource := flyHelper.CheckResource("-r", fmt.Sprintf("%s/some-git-resource", pipelineName), "-f", fmt.Sprintf("ref:%s", rev))
				<-checkResource.Exited
				Expect(checkResource.ExitCode()).To(Equal(0))
				Expect(checkResource).To(gbytes.Say("checked 'some-git-resource'"))

				watch := flyHelper.Watch(pipelineName, "some-passing-job", "1")
				Eventually(watch).Should(gbytes.Say(guid2))
				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "2")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})
	})
})
