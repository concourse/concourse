package testflight_test

import (
	"time"

	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource container GC", func() {
	var checkContainerHandle string

	BeforeEach(func() {
		uuid, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/resource-gc-get-and-task.yml",
			"-v", "unique_version="+uuid.String(),
		)

		fly("check-resource", "-r", inPipeline("simple-resource"))

		for _, container := range flyTable("containers") {
			if container["type"] == "check" && container["pipeline"] == pipelineName && container["name"] == "simple-resource" {
				checkContainerHandle = container["handle"]
			}
		}
	})

	Describe("removing the resource", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/resource-gc-removed.yml")
		})

		It("eventually removes the check container", func() {
			Eventually(func() []string {
				handles := []string{}
				for _, container := range flyTable("containers") {
					if container["type"] == "check" && container["pipeline"] == pipelineName && container["name"] == "simple-resource" {
						handles = append(handles, container["handle"])
					}
				}

				return handles
			}, 5*time.Minute, 10*time.Second).ShouldNot(ContainElement(checkContainerHandle))
		})
	})

	Describe("changing the resource config", func() {
		BeforeEach(func() {
			uuid, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			setAndUnpausePipeline(
				"fixtures/resource-gc-get-and-task.yml",
				"-v", "unique_version="+uuid.String(),
			)

			fly("check-resource", "-r", inPipeline("simple-resource"))
		})

		It("eventually removes the check container", func() {
			Eventually(func() []string {
				handles := []string{}
				for _, container := range flyTable("containers") {
					if container["type"] == "check" && container["pipeline"] == pipelineName && container["name"] == "simple-resource" {
						handles = append(handles, container["handle"])
					}
				}

				return handles
			}, 5*time.Minute, 10*time.Second).ShouldNot(ContainElement(checkContainerHandle))
		})
	})
})
