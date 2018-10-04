package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Task caching", func() {
	It("caches directories in the cache list without caching any non-cached directories", func() {
		setAndUnpausePipeline("fixtures/task-caches.yml")

		watch := fly("trigger-job", "-j", inPipeline("simple"), "-w")
		Expect(watch).To(gbytes.Say("not-cached-from-first-task"))
		Expect(watch).To(gbytes.Say("not-cached-from-second-task"))
		Expect(watch).To(gbytes.Say("blob-contents-from-second-task"))
		Expect(watch).To(gbytes.Say("second-task-cache-contents"))

		Expect(watch).NotTo(gbytes.Say("blob-contents-from-first-task"))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-first-task\s+not-cached-from-first-task`))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-second-task\s+not-cached-from-second-task`))
		Expect(watch).NotTo(gbytes.Say(`blob-contents-from-second-task\s+blob-contents-from-second-task`))
		Expect(watch).NotTo(gbytes.Say(`second-task-cache-contents\s+second-task-cache-contents`))

		watch = fly("trigger-job", "-j", inPipeline("simple"), "-w")
		Expect(watch).To(gbytes.Say("not-cached-from-first-task"))
		Expect(watch).To(gbytes.Say("not-cached-from-second-task"))
		Expect(watch).To(gbytes.Say(`blob-contents-from-second-task\s+blob-contents-from-second-task`))
		Expect(watch).To(gbytes.Say(`second-task-cache-contents\s+second-task-cache-contents`))

		Expect(watch).NotTo(gbytes.Say("blob-contents-from-first-task"))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-first-task\s+not-cached-from-first-task`))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-second-task\s+not-cached-from-second-task`))
	})
})
