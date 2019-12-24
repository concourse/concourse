package topgun_test

import (
	"strings"
	"time"

	. "github.com/concourse/concourse/topgun/common"
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
			Fly.Run("execute", "-c", "tasks/tiny.yml")

			By("verifying that the image cache sticks around")
			Consistently(func() []string {
				volumes := FlyTable("volumes")
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
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/build-image-gc-part-1.yml", "-p", "test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "test")

			By("triggering a build that waits")
			watchSession := Fly.Start("trigger-job", "-w", "-j", "test/some-job")
			//For the initializing block
			Eventually(watchSession).Should(gbytes.Say("echo 'waiting for /tmp/stop-waiting to exist'"))
			//For the output from the running step
			Eventually(watchSession).Should(gbytes.Say("waiting for /tmp/stop-waiting to exist"))

			By("getting the resource cache volumes")
			volumes := FlyTable("volumes")
			originalResourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "digest:") {
					originalResourceVolumeHandles = append(originalResourceVolumeHandles, volume["handle"])
				}
			}
			Expect(originalResourceVolumeHandles).To(HaveLen(1))

			By("setting a pipeline that uses image B")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/build-image-gc-part-2.yml", "-p", "test")

			By("triggering a build that succeeds")
			Fly.Run("trigger-job", "-w", "-j", "test/some-job")

			By("verifying that both image caches stick around")
			Consistently(func() []string {
				volumes := FlyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "digest:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}

				return resourceVolumeHandles
			}, time.Minute).Should(HaveLen(2))

			By("hijacking the previous build to tell it to finish")
			hijackSession := Fly.Start(
				"hijack",
				"-j", "test/some-job",
				"-b", "1",
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
				volumes := FlyTable("volumes")
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
