package topgun_test

import (
	"strings"
	"time"

	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Garbage collecting resource cache volumes", func() {
	Describe("A resource that was removed from pipeline", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			Fly.Run("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			Expect(VolumesByResourceType("time")).To(HaveLen(1))

			By("updating pipeline and removing resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() int {
				return len(VolumesByResourceType("time"))
			}, 5*time.Minute, 10*time.Second).Should(BeZero())
		})
	})

	Describe("A resource that was updated", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			Fly.Run("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			volumes := FlyTable("volumes")
			originalResourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "time:") {
					originalResourceVolumeHandles = append(originalResourceVolumeHandles, volume["handle"])
				}
			}
			Expect(originalResourceVolumeHandles).To(HaveLen(1))

			By("updating pipeline and removing resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() []string {
				return VolumesByResourceType("time")
			}, 5*time.Minute, 10*time.Second).ShouldNot(ContainElement(originalResourceVolumeHandles[0]))
		})
	})

	Describe("A resource in paused pipeline", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			Fly.Run("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			volumes := FlyTable("volumes")
			resourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "time:") {
					resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
				}
			}
			Expect(resourceVolumeHandles).To(HaveLen(1))

			By("pausing the pipeline")
			Fly.Run("pause-pipeline", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() int {
				return len(VolumesByResourceType("time"))
			}, 5*time.Minute, 10*time.Second).Should(BeZero())
		})
	})

	Describe("a resource that has new versions", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its old resource cache, old resource cache uses and old resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			Fly.Run("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("locating the cache")
			volumes := FlyTable("volumes")
			var firstCacheHandle string
			for _, volume := range volumes {
				if volume["type"] == "resource" && volume["identifier"] == "version:first-version" {
					firstCacheHandle = volume["handle"]
				}
			}
			Expect(firstCacheHandle).ToNot(BeEmpty(), "should have found a resource cache volume")

			By("creating a new resource version")
			Fly.Run("check-resource", "-r", "volume-gc-test/some-resource", "-f", "version:second-version")

			By("triggering the job")
			Fly.Run("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("eventually expiring the resource cache volume")
			Eventually(func() []string {
				volumes := FlyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "version:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return resourceVolumeHandles
			}, 10*time.Minute, 10*time.Second).ShouldNot(ContainElement(firstCacheHandle))
		})
	})

	Describe("resource cache is not reaped when being used by a build", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml", "-o", "operations/fast-gc.yml")
		})

		It("finds the resource cache volumes throughout duration of build", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource-and-wait.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			watchSession := Fly.Start("trigger-job", "-w", "-j", "volume-gc-test/simple-job")
			Eventually(watchSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("locating the cache")
			volumes := FlyTable("volumes")
			var firstCacheHandle string
			for _, volume := range volumes {
				if volume["type"] == "resource" && volume["identifier"] == "version:first-version" {
					firstCacheHandle = volume["handle"]
				}
			}
			Expect(firstCacheHandle).ToNot(BeEmpty(), "should have found a resource cache volume")

			By("creating a new resource version")
			Fly.Run("check-resource", "-r", "volume-gc-test/some-resource", "-f", "version:second-version")

			By("not expiring the resource cache volume for the ongoing build")
			Consistently(func() []string {
				volumes := FlyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && volume["identifier"] == "version:first-version" {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return resourceVolumeHandles
			}).Should(ConsistOf([]string{firstCacheHandle}))

			By("hijacking the build to tell it to finish")
			Fly.Run(
				"hijack",
				"-j", "volume-gc-test/simple-job",
				"-s", "wait",
				"touch", "/tmp/stop-waiting",
			)

			By("waiting for the build to exit")
			Eventually(watchSession, 1*time.Minute).Should(gbytes.Say("done"))
			<-watchSession.Exited
			Expect(watchSession.ExitCode()).To(Equal(0))

			By("eventually expiring the resource cache volume")
			Eventually(func() []string {
				volumes := FlyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && volume["identifier"] == "version:first-version" {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return resourceVolumeHandles
			}).ShouldNot(ContainElement(firstCacheHandle))
		})
	})
})
