package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task using an image within the plan", func() {
	BeforeEach(func() {
		Deploy(
			"deployments/concourse.yml",
			"-o", "operations/add-other-worker.yml",
			"-o", "operations/other-worker-tagged.yml",
		)

		Fly.Run("set-pipeline", "-n", "-p", "image-artifact", "-c", "pipelines/image-artifact.yml")
		Fly.Run("unpause-pipeline", "-p", "image-artifact")
	})

	Describe("running the job", func() {
		var jobName string
		var jobSession *gexec.Session

		BeforeEach(func() {
			jobName = ""
		})

		JustBeforeEach(func() {
			jobSession = Fly.Start("trigger-job", "-w", "-j", "image-artifact/"+jobName)
			<-jobSession.Exited
		})

		Context("when the artifact is found on the worker", func() {
			BeforeEach(func() {
				jobName = "artifact-test-found-locally"
			})

			It("uses the specified image artifact to run the task", func() {
				// Simultaneously test that it's using the image artifact instead of the
				// image resource, and also that the files are mounted with the right
				// permissions for a non-privileged task. If it's using the image
				// resource, bash won't be installed and .bashrc won't exist. If the
				// file permissions are set up for a privileged task, the contents of
				// /root won't be accessible to the task's "fake root" user
				Expect(jobSession).To(gbytes.Say(".bashrc"))
			})
		})

		Context("when the artifact is found on another worker", func() {
			BeforeEach(func() {
				jobName = "artifact-test-stream-in"
			})

			It("uses the specified image artifact to run the task", func() {
				Expect(jobSession).To(gbytes.Say(".bashrc"))
			})
		})
	})
})
