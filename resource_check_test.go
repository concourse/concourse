package topgun_test

import (
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource checking", func() {
	BeforeEach(func() {
		Deploy("deployments/two-workers-one-tagged.yml")
	})

	Context("with tags on the resource", func() {
		BeforeEach(func() {
			By("setting a pipeline that has a tagged resource")
			fly("set-pipeline", "-n", "-c", "pipelines/tagged-resource.yml", "-p", "tagged-resource")

			By("unpausing the pipeline pipeline")
			fly("unpause-pipeline", "-p", "tagged-resource")
		})

		It("places the checking container on the tagged worker", func() {
			By("running the check")
			fly("check-resource", "-r", "tagged-resource/some-resource")

			By("getting the worker name")
			workersTable := flyTable("workers")
			var taggedWorkerName string
			for _, w := range workersTable {
				if w["tags"] == "tagged" {
					taggedWorkerName = w["name"]
				}
			}
			Expect(taggedWorkerName).ToNot(BeEmpty())

			By("checking that the container is on the tagged worker")
			containerTable := flyTable("containers")
			Expect(containerTable).To(HaveLen(1))
			Expect(containerTable[0]["type"]).To(Equal("check"))
			Expect(containerTable[0]["worker"]).To(Equal(taggedWorkerName))
		})
	})
})
