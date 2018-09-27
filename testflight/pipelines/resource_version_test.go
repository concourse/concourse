package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource version", func() {
	Describe("when the version is not pinned on the resource", func() {
		Describe("version: latest", func() {
			It("only runs builds with latest version", func() {
				hash, err := uuid.NewV4()
				Expect(err).ToNot(HaveOccurred())

				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-version-latest.yml",
					"-v", "hash="+hash.String(),
				)

				guid1 := newMockVersion("some-resource", "guid1")
				watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid1))

				_ = newMockVersion("some-resource", "guid2")
				_ = newMockVersion("some-resource", "guid3")
				guid4 := newMockVersion("some-resource", "guid4")

				watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid4))

				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "3")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})

		Describe("version: every", func() {
			It("runs builds with every version", func() {
				hash, err := uuid.NewV4()
				Expect(err).ToNot(HaveOccurred())

				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-version-every.yml",
					"-v", "hash="+hash.String(),
				)

				guid1 := newMockVersion("some-resource", "guid1")
				watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid1))

				guid2 := newMockVersion("some-resource", "guid2")
				guid3 := newMockVersion("some-resource", "guid3")
				guid4 := newMockVersion("some-resource", "guid4")

				watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid2))

				watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid3))

				watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid4))
			})
		})

		Describe("version: pinned", func() {
			It("only runs builds with the pinned version", func() {
				hash, err := uuid.NewV4()
				Expect(err).ToNot(HaveOccurred())

				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-version-latest.yml",
					"-v", "hash="+hash.String(),
				)

				guid1 := newMockVersion("some-resource", "guid1")

				watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
				Eventually(watch).Should(gbytes.Say(guid1))

				_ = newMockVersion("some-resource", "guid2")
				guid3 := newMockVersion("some-resource", "guid3")
				_ = newMockVersion("some-resource", "guid4")

				flyHelper.ReconfigurePipeline(
					pipelineName,
					"-c", "fixtures/pinned-version.yml",
					"-v", "pinned_version="+guid3,
					"-v", "hash="+hash.String(),
				)

				watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(guid3))

				Consistently(func() bool {
					_, found, err := team.JobBuild(pipelineName, "some-passing-job", "3")
					Expect(err).NotTo(HaveOccurred())
					return found
				}).Should(BeFalse())
			})
		})
	})

	Describe("when the version is pinned on the resource", func() {
		var olderGuid string
		var pinnedGuid string
		var versionConfig string
		var hash *uuid.UUID

		BeforeEach(func() {
			versionConfig = "nil"

			var err error
			hash, err = uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/pinned-resource-simple-trigger.yml",
				"-v", "pinned_resource_version=bogus",
				"-y", "version_config="+versionConfig,
				"-v", "hash="+hash.String(),
			)

			olderGuid = newMockVersion("some-resource", "older")
			pinnedGuid = newMockVersion("some-resource", "pinned")
			_ = newMockVersion("some-resource", "newer")
		})

		JustBeforeEach(func() {
			flyHelper.ReconfigurePipeline(
				pipelineName,
				"-c", "fixtures/pinned-resource-simple-trigger.yml",
				"-v", "pinned_resource_version="+pinnedGuid,
				"-y", "version_config="+versionConfig,
				"-v", "hash="+hash.String(),
			)
		})

		Describe("version: latest", func() {
			BeforeEach(func() {
				versionConfig = "latest"
			})

			It("only runs builds with pinned version", func() {
				watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(pinnedGuid))
			})
		})

		Describe("version: every", func() {
			BeforeEach(func() {
				versionConfig = "every"
			})

			It("only runs builds with pinned version", func() {
				watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(pinnedGuid))

				watch = flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(pinnedGuid))
			})
		})

		Describe("version: pinned", func() {
			BeforeEach(func() {
				versionConfig = "version:" + olderGuid
			})

			It("only runs builds with the pinned version", func() {
				watch := flyHelper.TriggerJob(pipelineName, "some-passing-job")
				<-watch.Exited
				Expect(watch.ExitCode()).To(Equal(0))
				Expect(watch).To(gbytes.Say(pinnedGuid))
			})
		})
	})
})
