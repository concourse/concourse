package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Task caching", func() {
	It("caches directories in the cache list without caching any non-cached directories", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/task-caches.yml",
		)

		watch := flyHelper.TriggerJob(pipelineName, "simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("not-cached-from-first-task"))
		Expect(watch).To(gbytes.Say("not-cached-from-second-task"))
		Expect(watch).To(gbytes.Say("blob-contents-from-second-task"))
		Expect(watch).To(gbytes.Say("second-task-cache-contents"))

		Expect(watch).NotTo(gbytes.Say("blob-contents-from-first-task"))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-first-task\s+not-cached-from-first-task`))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-second-task\s+not-cached-from-second-task`))
		Expect(watch).NotTo(gbytes.Say(`blob-contents-from-second-task\s+blob-contents-from-second-task`))
		Expect(watch).NotTo(gbytes.Say(`second-task-cache-contents\s+second-task-cache-contents`))

		watch = flyHelper.TriggerJob(pipelineName, "simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("not-cached-from-first-task"))
		Expect(watch).To(gbytes.Say("not-cached-from-second-task"))
		Expect(watch).To(gbytes.Say(`blob-contents-from-second-task\s+blob-contents-from-second-task`))
		Expect(watch).To(gbytes.Say(`second-task-cache-contents\s+second-task-cache-contents`))

		Expect(watch).NotTo(gbytes.Say("blob-contents-from-first-task"))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-first-task\s+not-cached-from-first-task`))
		Expect(watch).NotTo(gbytes.Say(`not-cached-from-second-task\s+not-cached-from-second-task`))
	})
})
