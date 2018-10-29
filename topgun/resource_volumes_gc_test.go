package topgun_test

import (
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe(":life Garbage collecting resource cache volumes", func() {
	Describe("A resource that was removed from pipeline", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			Expect(volumesByResourceType("time")).To(HaveLen(1))

			By("updating pipeline and removing resource")
			fly("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() int {
				return len(volumesByResourceType("time"))
			}, 5*time.Minute, 10*time.Second).Should(BeZero())
		})
	})

	Describe("A resource that was updated", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			volumes := flyTable("volumes")
			originalResourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "time:") {
					originalResourceVolumeHandles = append(originalResourceVolumeHandles, volume["handle"])
				}
			}
			Expect(originalResourceVolumeHandles).To(HaveLen(1))

			By("updating pipeline and removing resource")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() []string {
				return volumesByResourceType("time")
			}, 5*time.Minute, 10*time.Second).ShouldNot(ContainElement(originalResourceVolumeHandles[0]))
		})
	})

	Describe("A resource in paused pipeline", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			volumes := flyTable("volumes")
			resourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "time:") {
					resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
				}
			}
			Expect(resourceVolumeHandles).To(HaveLen(1))

			By("pausing the pipeline")
			fly("pause-pipeline", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() int {
				return len(volumesByResourceType("time"))
			}, 5*time.Minute, 10*time.Second).Should(BeZero())
		})
	})

	Describe("a resource that has new versions", func() {
		var (
			gitRepoURI string
			gitRepo    GitRepo
		)

		BeforeEach(func() {
			if !strings.Contains(string(bosh("releases").Out.Contents()), "git-server") {
				Skip("git-server release not uploaded")
			}

			Deploy("deployments/concourse.yml", "-o", "operations/add-git-server.yml")

			gitRepoURI = fmt.Sprintf("git://%s/some-repo", JobInstance("git_server").IP)
			gitRepo = NewGitRepo(gitRepoURI)
		})

		AfterEach(func() {
			gitRepo.Cleanup()
		})

		It("has its old resource cache, old resource cache uses and old resource cache volumes cleared out", func() {
			By("creating an initial resource version")
			gitRepo.CommitAndPush()

			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-git-resource.yml", "-p", "volume-gc-test", "-v", "some-repo-uri="+gitRepoURI)

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource cache volumes")
			volumes := flyTable("volumes")
			originalResourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "ref:") {
					originalResourceVolumeHandles = append(originalResourceVolumeHandles, volume["handle"])
				}
			}
			Expect(originalResourceVolumeHandles).To(HaveLen(1))

			By("creating a new resource version")
			gitRepo.CommitAndPush()

			By("triggering the job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("eventually expiring the resource cache volume")
			Eventually(func() []string {
				volumes := flyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "ref:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return resourceVolumeHandles
			}, 10*time.Minute, 10*time.Second).ShouldNot(ContainElement(originalResourceVolumeHandles[0]))
		})
	})

	Describe("resource cache is not reaped when being used by a build", func() {
		var (
			gitRepoURI string
			gitRepo    GitRepo
		)

		BeforeEach(func() {
			if !strings.Contains(string(bosh("releases").Out.Contents()), "git-server") {
				Skip("git-server release not uploaded")
			}

			Skip("container gets GCed because the worker report interval is 30s, which is > the missing_since grace period (3 * atc GC interval, so 3s)")

			Deploy("deployments/concourse.yml", "-o", "operations/fast-gc.yml", "-o", "operations/add-git-server.yml")

			gitRepoURI = fmt.Sprintf("git://%s/some-repo", JobInstance("git_server").IP)
			gitRepo = NewGitRepo(gitRepoURI)
		})

		AfterEach(func() {
			gitRepo.Cleanup()
		})

		It("finds the resource cache volumes throughout duration of build", func() {
			By("creating an initial resource version")
			gitRepo.CommitAndPush()

			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-git-resource-and-wait.yml", "-p", "volume-gc-test", "-v", "some-repo-uri="+gitRepoURI)

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			watchSession := spawnFly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")
			Eventually(watchSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("getting the resource cache volumes")
			volumes := flyTable("volumes")
			originalResourceVolumeHandles := []string{}
			for _, volume := range volumes {
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "ref:") {
					originalResourceVolumeHandles = append(originalResourceVolumeHandles, volume["handle"])
				}
			}
			Expect(originalResourceVolumeHandles).To(HaveLen(1))

			By("creating a new resource version")
			gitRepo.CommitAndPush()

			By("detecting the new version")
			fly("check-resource", "-r", "volume-gc-test/some-repo")

			By("not expiring the resource cache volume for the ongoing build")
			Consistently(func() []string {
				volumes := flyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "ref:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return resourceVolumeHandles
			}).Should(ContainElement(originalResourceVolumeHandles[0]))

			By("hijacking the build to tell it to finish")
			hijackSession := spawnFly(
				"hijack",
				"-j", "volume-gc-test/simple-job",
				"-s", "wait",
				"touch", "/tmp/stop-waiting",
			)
			<-hijackSession.Exited
			Expect(hijackSession.ExitCode()).To(Equal(0))

			By("waiting for the build to exit")
			Eventually(watchSession, 1*time.Minute).Should(gbytes.Say("done"))
			<-watchSession.Exited
			Expect(watchSession.ExitCode()).To(Equal(0))

			By("eventually expiring the resource cache volume")
			Eventually(func() []string {
				volumes := flyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "ref:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return resourceVolumeHandles
			}).ShouldNot(ContainElement(originalResourceVolumeHandles[0]))
		})
	})
})
