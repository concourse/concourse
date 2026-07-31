package testflight_test

import (
	"os"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("serial groups", func() {
	var hash string

	BeforeEach(func() {
		u, err := uuid.NewRandom()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	Context("when no inputs are available for one resource", func() {
		var pendingS *gexec.Session

		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/serial-groups.yml", "-v", "hash="+hash)

			pendingS = spawnFly("trigger-job", "-j", inPipeline("some-pending-job"), "-w")
		})

		AfterEach(func() {
			pendingS.Signal(os.Interrupt)
			Eventually(pendingS).Should(gexec.Exit())
		})

		It("runs even when another job in the serial group has a pending build", func(ctx SpecContext) {
			fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			Consistently(pendingS, time.Second).ShouldNot(gexec.Exit())
		}, DefaultSpecTimeout)
	})

	Context("when inputs eventually become available for one resource", func() {
		var hash2 string

		BeforeEach(func() {
			u, err := uuid.NewRandom()
			Expect(err).ToNot(HaveOccurred())

			hash2 = u.String()
		})

		BeforeEach(func() {
			setAndUnpausePipeline(
				"fixtures/serial-groups-inputs-updated.yml",
				"-v", "hash-1="+hash,
				"-v", "hash-2="+hash2,
			)
		})

		It("is able to run second job with latest inputs", func(ctx SpecContext) {
			By("starting a pending build")
			pendingS := spawnFly("trigger-job", "-j", inPipeline("some-pending-job"), "-w")

			By("making a new version")
			guid1 := newMockVersion("some-resource", "some-1")

			By("kicking off some-passing-job")
			watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			Expect(watch).To(gbytes.Say(guid1))

			By("making sure some-pending-job is still pending")
			Expect(pendingS).ToNot(gexec.Exit())

			By("making another new version")
			guid2 := newMockVersion("some-resource", "some-2")

			By("kicking off some-passing-job")
			watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			Expect(watch).To(gbytes.Say(guid2))

			By("making sure some-pending-job is still pending")
			Expect(pendingS).ToNot(gexec.Exit())

			By("making a version for the other resource")
			guid3 := newMockVersion("other-resource", "other-1")

			By("waiting for some-pending-job to finally run")
			Eventually(pendingS).Should(gexec.Exit(0))
			Expect(pendingS).To(gbytes.Say(guid2))
			Expect(pendingS).To(gbytes.Say(guid3))
		}, DefaultSpecTimeout)
	})

	Context("when a resource has check_every never configured", func() {
		It("checks the resource and runs the job", func(ctx SpecContext) {
			setAndUnpausePipeline("fixtures/serial-groups.yml", "-v", "hash="+hash)
			By("kicking off serial-true-job")
			sess := fly("trigger-job", "-j", inPipeline("serial-true-job"), "-w")
			Eventually(sess).To(gexec.Exit(0))
		}, DefaultSpecTimeout)
	})
})
