package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Clear task cache", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/clear-task-cache.yml")
	})

	It("clears the task's cache", func() {
		By("warming the cache")
		watch := fly("trigger-job", "-j", inPipeline("clear-task-cache"), "-w")
		Expect(watch).To(gbytes.Say("created-cache-file"))
		Expect(watch).NotTo(gbytes.Say("the-cached-file-already-exists"))

		By("observing the cache")
		watch = fly("trigger-job", "-j", inPipeline("clear-task-cache"), "-w")
		Expect(watch).To(gbytes.Say("the-cached-file-already-exists"))
		Expect(watch).NotTo(gbytes.Say("created-cache-file"))

		By("clearing the task cache")
		fly("clear-task-cache", "-n", "-j", inPipeline("clear-task-cache"), "-s", "some-task")

		By("warming the cache again")
		watch = fly("trigger-job", "-j", inPipeline("clear-task-cache"), "-w")
		Expect(watch).To(gbytes.Say("created-cache-file"))
		Expect(watch).NotTo(gbytes.Say("the-cached-file-already-exists"))
	})
})
