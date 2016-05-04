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
		
		workers, err := client.ListWorkers()
		Expect(err).NotTo(HaveOccurred())

		var hasTaggedWorker bool
	dance:
		for _, worker := range workers {
			for _, tag := range worker.Tags {
				if tag == "tagged" {
					hasTaggedWorker = true
					break dance
				}
			}
		}

		if !hasTaggedWorker {
			Skip("this only runs when a worker with the 'tagged' tag is available")
		}
	})

	It("uses the specified image artifact to run the task", func() {
		watch := flyWatch("artifact-test")
		Expect(watch).To(gbytes.Say("/bin/bash"))
	})
})
