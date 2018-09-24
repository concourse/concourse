package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Clear task cache", func() {
	It("clears the task's cache", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/clear-task-cache.yml",
		)

		watch := flyHelper.TriggerJob(pipelineName, "clear-task-cache")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("created-cache-file"))
		Expect(watch).NotTo(gbytes.Say("the-cached-file-already-exists"))

		watch = flyHelper.TriggerJob(pipelineName, "clear-task-cache")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("the-cached-file-already-exists"))
		Expect(watch).NotTo(gbytes.Say("created-cache-file"))

		flyHelper.ClearTaskCache(pipelineName+"/clear-task-cache", "some-task")

		watch = flyHelper.TriggerJob(pipelineName, "clear-task-cache")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("created-cache-file"))
		Expect(watch).NotTo(gbytes.Say("the-cached-file-already-exists"))
	})
})
