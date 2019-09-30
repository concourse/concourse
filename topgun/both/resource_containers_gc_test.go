package topgun_test

import (
	"fmt"
	"time"

	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garbage collecting resource containers", func() {
	Describe("A container that is used by resource checking on freshly deployed worker", func() {
		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/worker-instances.yml",
				"-v", "worker_instances=2",
			)
		})

		It("is recreated in database and worker", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("checking resource")
			Fly.Run("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			containers := FlyTable("containers")
			var checkContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					checkContainerHandle = container["handle"]
					break
				}
			}
			Expect(checkContainerHandle).NotTo(BeEmpty())

			By(fmt.Sprintf("eventually expiring the resource config container: %s", checkContainerHandle))
			Eventually(func() bool {
				containers := FlyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" && container["handle"] == checkContainerHandle {
						return true
					}
				}
				return false
			}, 10*time.Minute, 10*time.Second).Should(BeFalse())

			By("checking resource again")
			Fly.Run("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			containers = FlyTable("containers")
			var newCheckContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					newCheckContainerHandle = container["handle"]
					break
				}
			}
			Expect(newCheckContainerHandle).NotTo(Equal(checkContainerHandle))
		})
	})

	Describe("container for resource that is removed from pipeline", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource config, resource config uses and container removed", func() {
			By("setting pipeline that creates resource config")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "resource-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "resource-gc-test")

			By("checking resource")
			Fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")

			By("getting the resource config")
			var resourceConfigsNum int
			err := Psql.Select("COUNT(id)").From("resource_configs").RunWith(DbConn).QueryRow().Scan(&resourceConfigsNum)
			Expect(err).ToNot(HaveOccurred())

			Expect(resourceConfigsNum).To(Equal(1))

			By("getting the resource config container")
			containers := FlyTable("containers")
			var checkContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					checkContainerHandle = container["handle"]
					break
				}
			}
			Expect(checkContainerHandle).NotTo(BeEmpty())

			By("updating pipeline and removing resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "resource-gc-test")

			By("eventually expiring the resource config")
			Eventually(func() int {
				var resourceConfigsNum int
				err := Psql.Select("COUNT(id)").From("resource_configs").RunWith(DbConn).QueryRow().Scan(&resourceConfigsNum)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigsNum
			}, 5*time.Minute, 10*time.Second).Should(Equal(0))

			By(fmt.Sprintf("eventually expiring the resource config container: %s", checkContainerHandle))
			Eventually(func() bool {
				containers := FlyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" && container["handle"] == checkContainerHandle {
						return true
					}
				}
				return false
			}, 5*time.Minute, 10*time.Second).Should(BeFalse())
		})
	})

	Describe("container for resource that is updated", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("has its resource config, resource config uses and container removed", func() {
			By("setting pipeline that creates resource config")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "resource-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "resource-gc-test")

			By("checking resource")
			Fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")

			By("getting the resource config")
			var originalResourceConfigID int
			err := Psql.Select("id").From("resource_configs").RunWith(DbConn).QueryRow().Scan(&originalResourceConfigID)
			Expect(err).ToNot(HaveOccurred())

			Expect(originalResourceConfigID).NotTo(BeZero())

			By("getting the resource config container")
			containers := FlyTable("containers")
			var originalCheckContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					originalCheckContainerHandle = container["handle"]
					break
				}
			}
			Expect(originalCheckContainerHandle).NotTo(BeEmpty())

			By("updating pipeline with new resource configuration")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "resource-gc-test")

			By("eventually expiring the resource config")
			Eventually(func() int {
				var resourceConfigsNum int
				err := Psql.Select("COUNT(id)").From("resource_configs").Where("id = $1", originalResourceConfigID).RunWith(DbConn).QueryRow().Scan(&resourceConfigsNum)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigsNum
			}, 5*time.Minute, 10*time.Second).Should(Equal(0))

			By(fmt.Sprintf("eventually expiring the resource config container: %s", originalCheckContainerHandle))
			Eventually(func() bool {
				containers := FlyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" && container["handle"] == originalCheckContainerHandle {
						return true
					}
				}
				return false
			}, 5*time.Minute, 10*time.Second).Should(BeFalse())
		})
	})

	Describe("container for resource checking", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml", "-o", "operations/fast-gc.yml")
		})

		It("is not immediately removed", func() {
			By("setting pipeline that creates resource config")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "resource-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "resource-gc-test")

			By("checking resource")
			Fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")

			Consistently(func() string {
				By("getting the resource config container")
				containers := FlyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" {
						return container["handle"]
					}
				}

				return ""
			}, 2*time.Minute).ShouldNot(BeEmpty())
		})

		Context("when two teams use identical configuration", func() {
			var teamName = "A-Team"

			It("doesn't create many containers for one resource check", func() {
				By("setting pipeline that creates resource config")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "resource-gc-test")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "resource-gc-test")

				By("checking resource")
				Fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")

				By("creating another team")
				Fly.Run("set-team", "--non-interactive", "--team-name", teamName, "--local-user", AtcUsername)

				Fly.Run("login", "-c", AtcExternalURL, "-n", teamName, "-u", AtcUsername, "-p", AtcPassword)

				By("setting pipeline that creates an identical resource config")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "resource-gc-test")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "resource-gc-test")

				By("checking resource excessively")
				for i := 0; i < 20; i++ {
					Fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")
				}

				otherTeamCheckCount := len(FlyTable("containers"))
				Expect(otherTeamCheckCount).To(Equal(1))

				By("checking resource excessively")
				Fly.Run("login", "-c", AtcExternalURL, "-n", "main", "-u", AtcUsername, "-p", AtcPassword)
				for i := 0; i < 20; i++ {
					Fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")
				}

				mainTeamCheckCount := len(FlyTable("containers"))
				Expect(mainTeamCheckCount).To(Equal(1))
			})
		})
	})
})
