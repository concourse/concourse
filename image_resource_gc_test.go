package topgun_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A build using an image_resource", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/fast-gc.yml")
	})

	Describe("one-off builds", func() {
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

	Describe("pipeline builds", func() {
		It("keeps images for the latest build", func() {
			By("setting a pipeline that uses image A")
			fly("set-pipeline", "-n", "-c", "pipelines/build-image-gc-part-1.yml", "-p", "test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "test")

			By("triggering a build that waits")
			watchSession := spawnFly("trigger-job", "-w", "-j", "test/some-job")
			Eventually(watchSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("getting the resource cache volumes")
			volumes := flyTable("volumes")
			originalResourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "digest:") {
					originalResourceVolumeHandles = append(originalResourceVolumeHandles, volume["handle"])
				}
			}
			Expect(originalResourceVolumeHandles).To(HaveLen(1))

			By("setting a pipeline that uses image B")
			fly("set-pipeline", "-n", "-c", "pipelines/build-image-gc-part-2.yml", "-p", "test")

			By("triggering a build that succeeds")
			fly("trigger-job", "-w", "-j", "test/some-job")

			By("verifying that both image caches stick around")
			Consistently(func() []string {
				volumes := flyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "digest:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}

				return resourceVolumeHandles
			}, time.Minute).Should(HaveLen(2))

			By("hijacking the previous build to tell it to finish")
			hijackSession := spawnFly(
				"hijack",
				"-j", "test/some-job",
				"-s", "wait",
				"touch", "/tmp/stop-waiting",
			)
			<-hijackSession.Exited
			Expect(hijackSession.ExitCode()).To(Equal(0))

			By("waiting for the build to exit")
			Eventually(watchSession, 1*time.Minute).Should(gbytes.Say("done"))
			<-watchSession.Exited
			Expect(watchSession.ExitCode()).To(Equal(0))

			By("eventually expiring the previous build's resource cache volume")
			var remainingHandles []string
			Eventually(func() []string {
				volumes := flyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "digest:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}

				remainingHandles = resourceVolumeHandles

				return resourceVolumeHandles
			}, 10*time.Minute, 10*time.Second).Should(HaveLen(1))

			By("keeping the new image")
			Expect(remainingHandles).ToNot(Equal(originalResourceVolumeHandles))
		})
	})
})
