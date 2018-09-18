package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("task caching", func() {
	var (
		originGitServer *gitserver.Server
	)

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("caches directories in the cache list without caching any non-cached directories", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/task-caches.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		// If we were to just use config in the pipeline, we would see the
		// echoing of the things we are looking for when it says "running ..."
		// So to prevent this we commit it to a resource and so then it just says
		// e.g. "running sh some-git-resource/first-script.sh"
		// and leaves the output of the fly buffer with just the contents that are
		// cat'ed out

		firstScript := `echo not-cached-from-first-task >> first-task-output/not-cached-from-first-task

    mkdir first-task-output/blobs
    echo blob-contents-from-first-task >> first-task-output/blobs/blob`
		originGitServer.CommitFileToBranch(firstScript, "first-script.sh", "master")

		secondScript := `cat ./first-task-output/not-cached-from-first-task

    echo not-cached-from-second-task >> first-task-output/not-cached-from-second-task
    cat ./first-task-output/not-cached-from-second-task

    echo blob-contents-from-second-task >> ./first-task-output/blobs/blob
    cat ./first-task-output/blobs/blob

    echo second-task-cache-contents >> ./second-task-cache/cache
    cat ./second-task-cache/cache`
		originGitServer.CommitFileToBranch(secondScript, "second-script.sh", "master")

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
