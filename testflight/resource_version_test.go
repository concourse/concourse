package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource version", func() {
	var hash string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	Describe("when the version is not pinned on the resource", func() {
		Describe("version: latest", func() {
			BeforeEach(func() {
				setAndUnpausePipeline("fixtures/resource-version-latest.yml", "-v", "hash="+hash)
			})

			It("only runs builds with latest version", func() {
				guid1 := newMockVersion("some-resource", "guid1")
				watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid1))

				_ = newMockVersion("some-resource", "guid2")
				_ = newMockVersion("some-resource", "guid3")
				guid4 := newMockVersion("some-resource", "guid4")

				watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid4))
			})
		})

		Describe("version: every", func() {
			BeforeEach(func() {
				setAndUnpausePipeline("fixtures/resource-version-every.yml", "-v", "hash="+hash)
			})

			It("runs builds with every version", func() {
				guid1 := newMockVersion("some-resource", "guid1")
				watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid1))

				guid2 := newMockVersion("some-resource", "guid2")
				guid3 := newMockVersion("some-resource", "guid3")
				guid4 := newMockVersion("some-resource", "guid4")

				watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid2))

				watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid3))

				watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid4))
			})
		})

		Describe("version: pinned", func() {
			BeforeEach(func() {
				setAndUnpausePipeline("fixtures/resource-version-every.yml", "-v", "hash="+hash)
			})

			It("only runs builds with the pinned version", func() {
				guid1 := newMockVersion("some-resource", "guid1")

				watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Eventually(watch).Should(gbytes.Say(guid1))

				_ = newMockVersion("some-resource", "guid2")
				guid3 := newMockVersion("some-resource", "guid3")
				_ = newMockVersion("some-resource", "guid4")

				setPipeline("fixtures/pinned-version.yml", "-v", "hash="+hash, "-v", "pinned_version="+guid3)

				watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(guid3))
			})
		})
	})

	Describe("when the version is pinned on the resource", func() {
		var olderGUID string
		var pinnedGUID string
		var versionConfig string

		BeforeEach(func() {
			versionConfig = "null"

			setAndUnpausePipeline(
				"fixtures/pinned-resource-simple-trigger.yml",
				"-y", "pinned_resource_version=null",
				"-y", "version_config="+versionConfig,
				"-v", "hash="+hash,
			)

			olderGUID = newMockVersion("some-resource", "older")
			pinnedGUID = newMockVersion("some-resource", "pinned")
			_ = newMockVersion("some-resource", "newer")
		})

		JustBeforeEach(func() {
			setPipeline(
				"fixtures/pinned-resource-simple-trigger.yml",
				"-y", `pinned_resource_version={"version":"`+pinnedGUID+`"}`,
				"-y", "version_config="+versionConfig,
				"-v", "hash="+hash,
			)
		})

		Describe("version: latest", func() {
			BeforeEach(func() {
				versionConfig = "latest"
			})

			It("only runs builds with pinned version", func() {
				watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(pinnedGUID))
			})
		})

		Describe("version: every", func() {
			BeforeEach(func() {
				versionConfig = "every"
			})

			It("only runs builds with pinned version", func() {
				watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(pinnedGUID))

				watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(pinnedGUID))
			})
		})

		Describe("version: pinned", func() {
			BeforeEach(func() {
				versionConfig = "version:" + olderGUID
			})

			It("only runs builds with the pinned version", func() {
				watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
				Expect(watch).To(gbytes.Say(pinnedGUID))
			})
		})
	})
})
