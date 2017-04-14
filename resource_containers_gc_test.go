package topgun_test

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe(":life Garbage collecting resource containers", func() {
	Describe("A container that is used by resource checking on freshly deployed worker", func() {
		BeforeEach(func() {
			Deploy("deployments/two-forwarded-workers.yml")
		})

		It("is recreated in database and worker [#129726933]", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("checking resource")
			fly("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			containers := flyTable("containers")
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
				containers := flyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" && container["handle"] == checkContainerHandle {
						return true
					}
				}
				return false
			}, 10*time.Minute, 10*time.Second).Should(BeFalse())

			By("checking resource again")
			fly("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			containers = flyTable("containers")
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
		var dbConn *sql.DB

		BeforeEach(func() {
			var err error
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
			Expect(err).ToNot(HaveOccurred())

			Deploy("deployments/single-vm.yml")
		})

		It("has its resource config, resource config uses and container removed", func() {
			By("setting pipeline that creates resource config")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "resource-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "resource-gc-test")

			By("checking resource")
			fly("check-resource", "-r", "resource-gc-test/tick-tock")

			By("getting the resource config")
			var resourceConfigsNum int
			err := psql.Select("COUNT(id)").From("resource_configs").RunWith(dbConn).QueryRow().Scan(&resourceConfigsNum)
			Expect(err).ToNot(HaveOccurred())

			Expect(resourceConfigsNum).To(Equal(1))

			By("getting the resource config container")
			containers := flyTable("containers")
			var checkContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					checkContainerHandle = container["handle"]
					break
				}
			}
			Expect(checkContainerHandle).NotTo(BeEmpty())

			By("updating pipeline and removing resource")
			fly("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "resource-gc-test")

			By("eventually expiring the resource config")
			Eventually(func() int {
				var resourceConfigsNum int
				err := psql.Select("COUNT(id)").From("resource_configs").RunWith(dbConn).QueryRow().Scan(&resourceConfigsNum)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigsNum
			}, 5*time.Minute, 10*time.Second).Should(Equal(0))

			By(fmt.Sprintf("eventually expiring the resource config container: %s", checkContainerHandle))
			Eventually(func() bool {
				containers := flyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" && container["handle"] == checkContainerHandle {
						return true
					}
				}
				return false
			}, 5*time.Minute, 10*time.Second).Should(BeFalse())
		})
	})

	Describe("container for resource when pipeline is paused", func() {
		var dbConn *sql.DB

		BeforeEach(func() {
			var err error
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
			Expect(err).ToNot(HaveOccurred())

			Deploy("deployments/single-vm.yml")
		})

		It("has its resource config, resource config uses and container removed", func() {

			By("setting pipeline that creates resource config")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "resource-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "resource-gc-test")

			By("checking resource")
			fly("check-resource", "-r", "resource-gc-test/tick-tock")

			By("getting the resource config")
			var resourceConfigsNum int
			err := psql.Select("COUNT(id)").From("resource_configs").RunWith(dbConn).QueryRow().Scan(&resourceConfigsNum)
			Expect(err).ToNot(HaveOccurred())

			Expect(resourceConfigsNum).To(Equal(1))

			By("getting the resource config container")
			containers := flyTable("containers")
			var checkContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					checkContainerHandle = container["handle"]
					break
				}
			}
			Expect(checkContainerHandle).NotTo(BeEmpty())

			By("pausing the pipeline")
			fly("pause-pipeline", "-p", "resource-gc-test")

			By("eventually expiring the resource config")
			Eventually(func() int {
				var resourceConfigsNum int
				err := psql.Select("COUNT(id)").From("resource_configs").RunWith(dbConn).QueryRow().Scan(&resourceConfigsNum)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigsNum
			}, 5*time.Minute, 10*time.Second).Should(Equal(0))

			By(fmt.Sprintf("eventually expiring the resource config container: %s", checkContainerHandle))
			Eventually(func() bool {
				containers := flyTable("containers")
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
		var dbConn *sql.DB

		BeforeEach(func() {
			var err error
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
			Expect(err).ToNot(HaveOccurred())

			Deploy("deployments/single-vm.yml")
		})

		It("has its resource config, resource config uses and container removed", func() {

			By("setting pipeline that creates resource config")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "resource-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "resource-gc-test")

			By("checking resource")
			fly("check-resource", "-r", "resource-gc-test/tick-tock")

			By("getting the resource config")
			var originalResourceConfigID int
			err := psql.Select("id").From("resource_configs").RunWith(dbConn).QueryRow().Scan(&originalResourceConfigID)
			Expect(err).ToNot(HaveOccurred())

			Expect(originalResourceConfigID).NotTo(BeZero())

			By("getting the resource config container")
			containers := flyTable("containers")
			var originalCheckContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					originalCheckContainerHandle = container["handle"]
					break
				}
			}
			Expect(originalCheckContainerHandle).NotTo(BeEmpty())

			By("updating pipeline with new resource configuration")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "resource-gc-test")

			By("eventually expiring the resource config")
			Eventually(func() int {
				var resourceConfigsNum int
				err := psql.Select("COUNT(id)").From("resource_configs").RunWith(dbConn).QueryRow().Scan(&resourceConfigsNum)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigsNum
			}, 5*time.Minute, 10*time.Second).Should(Equal(0))

			By(fmt.Sprintf("eventually expiring the resource config container: %s", originalCheckContainerHandle))
			Eventually(func() bool {
				containers := flyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" && container["handle"] == originalCheckContainerHandle {
						return true
					}
				}
				return false
			}, 5*time.Minute, 10*time.Second).Should(BeFalse())
		})
	})
})
