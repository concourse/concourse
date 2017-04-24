package topgun_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A one-off build using an image_resource", func() {
	BeforeEach(func() {
		Deploy("deployments/single-vm-fast-gc.yml")
	})

	It("does not garbage-collect the image immediately", func() {
		By("running a task with an image_resource")
		fly("execute", "-c", "tasks/tiny.yml")

		By("verifying that the image cache sticks around")
		Consistently(func() []string {
			volumes := flyTable("volumes")
			resourceVolumeHandles := []string{}
			for _, volume := range volumes {
				// there is going to be one for image resource
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "digest:") {
					resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
				}
			}

			return resourceVolumeHandles
		}, time.Minute).Should(HaveLen(1))
	})
})
