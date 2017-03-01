package pipelines_test

import (
	"os/exec"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a resouce with a tag", func() {
	BeforeEach(func() {
		if !hasTaggedWorkers() {
			Skip("this only runs when a worker with the 'tagged' tag is available")
		}
	})

	It("puts the resource check container on the tagged worker", func() {
		configurePipeline(
			"-c", "fixtures/tagged_resource.yml",
		)
		resourceString := pipelineName + "/" + "some-resource"

		fly := exec.Command(flyBin, "-t", targetedConcourse, "check-resource", "-r", resourceString)
		session := helpers.StartFly(fly)
		Eventually(session).Should(gexec.Exit(0))

		workersTable := flyTable("workers")
		taggedWorkerHandles := []string{}
		for _, w := range workersTable {
			if w["tags"] == "tagged" {
				taggedWorkerHandles = append(taggedWorkerHandles, w["name"])
			}
		}

		containerTable := flyTable("containers")
		currentPipelineContainers := []map[string]string{}
		for _, c := range containerTable {
			if c["pipeline"] == pipelineName {
				currentPipelineContainers = append(currentPipelineContainers, c)
			}
		}
		Expect(currentPipelineContainers).To(HaveLen(1))
		Expect(currentPipelineContainers[0]["type"]).To(Equal("check"))
		Expect(taggedWorkerHandles).To(ContainElement(currentPipelineContainers[0]["worker"]))
	})
})
