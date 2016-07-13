package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a task using an image within the plan", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/image-artifact.yml",
		)

		if !hasTaggedWorkers() {
			Skip("this only runs when a worker with the 'tagged' tag is available")
		}
	})

	Context("when the artifact is found on the worker", func() {
		jobFixture := "artifact-test-found-locally"

		It("uses the specified image artifact to run the task", func() {
			watch := flyWatch(jobFixture)
			// Simultaneously test that it's using the image artifact instead of the
			// image resource, and also that the files are mounted with the right
			// permissions for a non-privileged task. If it's using the image
			// resource, bash won't be installed and .bashrc won't exist. If the
			// file permissions are set up for a privileged task, the contents of
			// /root won't be accessible to the task's "fake root" user
			Expect(watch).To(gbytes.Say(".bashrc"))
		})
	})

	Context("when the artifact is found on another worker", func() {
		jobFixture := "artifact-test-stream-in"

		It("uses the specified image artifact to run the task", func() {
			watch := flyWatch(jobFixture)
			// See comment in previous test
			Expect(watch).To(gbytes.Say(".bashrc"))
		})
	})
})
