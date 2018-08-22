package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("clear task cache", func() {
	var (
		originGitServer *gitserver.Server
	)

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("clears the task's cache", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/clear-task-cache.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		// Put script in .sh file so that we can easily select what we want to
		// cat to fly's output

		script := `
    if [ ! -f some-git-resource/cache/cached-file ]; then
      echo the-cached-file-already-exists >> some-git-resource/cache/cached-file
      echo 'created-cache-file'
    else
      cat some-git-resource/cache/cached-file
    fi`
		originGitServer.CommitFileToBranch(script, "script.sh", "master")

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
