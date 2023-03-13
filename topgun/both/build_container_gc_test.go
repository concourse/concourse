package topgun_test

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Garbage collecting build containers", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml")
	})

	getContainers := func(condition, value string) []string {
		containers := FlyTable("containers")

		var handles []string
		for _, c := range containers {
			if c[condition] == value {
				handles = append(handles, c["handle"])
			}
		}

		return handles
	}

	Describe("A container that belonged to a build that succeeded", func() {
		Context("one-off builds", func() {
			It("is removed from the database and worker [#129725995]", func() {
				By("running a task with container having a rootfs, input, and output volume")
				Fly.Run("execute", "-c", "tasks/input-output.yml", "-i", "some-input=./tasks")

				By("collecting the build containers")
				buildContainerHandles := getContainers("build id", "1")

				By(fmt.Sprintf("eventually expiring the build containers: %v", buildContainerHandles))
				Eventually(func() int {
					var containerNum int
					err := Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": buildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 10*time.Minute, time.Second).Should(BeZero())

				By("having removed the containers from the worker")
				containers, err := WorkerGardenClient.Containers(nil)
				Expect(err).ToNot(HaveOccurred())

				existingHandles := []string{}
				for _, c := range containers {
					existingHandles = append(existingHandles, c.Handle())
				}

				for _, handle := range buildContainerHandles {
					Expect(existingHandles).ToNot(ContainElement(handle))
				}
			})
		})

		Context("pipeline builds", func() {
			It("is removed from the database and worker [#129725995]", func() {
				By("setting pipeline that creates containers for check, get, task, put")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put.yml", "-p", "build-container-gc")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "build-container-gc")

				By("triggering job")
				Fly.Run("trigger-job", "-w", "-j", "build-container-gc/simple-job")

				By("collecting the build containers")
				buildContainerHandles := getContainers("type", "task")

				By(fmt.Sprintf("eventually expiring the build containers: %v", buildContainerHandles))
				Eventually(func() int {
					var containerNum int
					err := Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": buildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 10*time.Minute, time.Second).Should(BeZero())

				By("having removed the containers from the worker")
				containers, err := WorkerGardenClient.Containers(nil)
				Expect(err).ToNot(HaveOccurred())

				existingHandles := []string{}
				for _, c := range containers {
					existingHandles = append(existingHandles, c.Handle())
				}

				for _, handle := range buildContainerHandles {
					Expect(existingHandles).ToNot(ContainElement(handle))
				}
			})
		})
	})

	Describe("A container that belonged to a build that fails", func() {
		Context("pipeline builds", func() {
			It("keeps in the database and worker [#129725995]", func() {
				By("setting pipeline that creates containers for check, get, task, put")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put-failing.yml", "-p", "build-container-gc")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "build-container-gc")

				By("triggering job")
				<-Fly.Start("trigger-job", "-w", "-j", "build-container-gc/simple-job").Exited

				By("collecting the build containers")
				buildContainerHandles := getContainers("type", "task")

				By(fmt.Sprintf("not expiring the build containers: %v", buildContainerHandles))
				Consistently(func() int {
					var containerNum int
					err := Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": buildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 2*time.Minute, time.Second).Should(Equal(len(buildContainerHandles)))

				By("not removing the containers from the worker")
				containers, err := WorkerGardenClient.Containers(nil)
				Expect(err).ToNot(HaveOccurred())

				existingHandles := []string{}
				for _, c := range containers {
					existingHandles = append(existingHandles, c.Handle())
				}

				for _, handle := range buildContainerHandles {
					Expect(existingHandles).To(ContainElement(handle))
				}
			})
		})

		Context("pipeline builds that fail subsequently", func() {
			It("keeps the latest build containers in the database and worker, removes old build containers from database and worker [#129725995]", func() {
				By("setting pipeline that creates containers for check, get, task, put")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put-failing.yml", "-p", "build-container-gc")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "build-container-gc")

				By("triggering first job")
				<-Fly.Start("trigger-job", "-w", "-j", "build-container-gc/simple-job").Exited

				By("collecting the first build containers")
				firstBuildContainerHandles := getContainers("type", "task")

				By("triggering second job")
				<-Fly.Start("trigger-job", "-w", "-j", "build-container-gc/simple-job").Exited

				By("collecting the second build containers")
				allBuildContainerHandles := getContainers("type", "task")

				var secondBuildContainerHandles []string
				for _, handle := range allBuildContainerHandles {
					alreadyExisted := false
					for _, preHandle := range firstBuildContainerHandles {
						if preHandle == handle {
							alreadyExisted = true
							break
						}
					}

					if !alreadyExisted {
						secondBuildContainerHandles = append(secondBuildContainerHandles, handle)
					}
				}

				By(fmt.Sprintf("eventually expiring the first build containers: %v", firstBuildContainerHandles))
				Eventually(func() int {
					var containerNum int
					err := Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": firstBuildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 10*time.Minute, time.Second).Should(BeZero())

				By("having removed the first build containers from the worker")
				containers, err := WorkerGardenClient.Containers(nil)
				Expect(err).ToNot(HaveOccurred())

				existingHandles := []string{}
				for _, c := range containers {
					existingHandles = append(existingHandles, c.Handle())
				}

				for _, handle := range firstBuildContainerHandles {
					Expect(existingHandles).NotTo(ContainElement(handle))
				}

				By(fmt.Sprintf("not expiring the second build containers: %v", secondBuildContainerHandles))
				Consistently(func() int {
					var containerNum int
					err := Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": secondBuildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 2*time.Minute, time.Second).Should(Equal(len(secondBuildContainerHandles)))

				By("not removing the containers from the worker")
				for _, handle := range secondBuildContainerHandles {
					Expect(existingHandles).To(ContainElement(handle))
				}
			})
		})

		Context("pipeline builds that is running and previous build failed", func() {
			It("keeps both the latest and previous build containers in the database and worker [#129725995]", func() {
				By("setting pipeline that creates containers for check, get, task, put")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put-failing.yml", "-p", "build-container-gc")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "build-container-gc")

				By("triggering first failing job")
				<-Fly.Start("trigger-job", "-w", "-j", "build-container-gc/simple-job").Exited

				By("collecting the first build containers")
				firstBuildContainerHandles := getContainers("type", "task")

				By("triggering second long running job")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put-waiting.yml", "-p", "build-container-gc")
				runningBuildSession := Fly.Start("trigger-job", "-w", "-j", "build-container-gc/simple-job")
				Eventually(runningBuildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

				By("collecting the second build containers")
				allBuildContainerHandles := getContainers("type", "task")

				var secondBuildContainerHandles []string
				for _, handle := range allBuildContainerHandles {
					alreadyExisted := false
					for _, preHandle := range firstBuildContainerHandles {
						if preHandle == handle {
							alreadyExisted = true
							break
						}
					}

					if !alreadyExisted {
						secondBuildContainerHandles = append(secondBuildContainerHandles, handle)
					}
				}

				By(fmt.Sprintf("not expiring the first build containers: %v", firstBuildContainerHandles))
				Consistently(func() int {
					var containerNum int
					err := Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": firstBuildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 2*time.Minute, time.Second).Should(Equal(len(firstBuildContainerHandles)))

				By("not removing the first build containers from the worker")
				containers, err := WorkerGardenClient.Containers(nil)
				Expect(err).ToNot(HaveOccurred())

				existingHandles := []string{}
				for _, c := range containers {
					existingHandles = append(existingHandles, c.Handle())
				}

				for _, handle := range firstBuildContainerHandles {
					Expect(existingHandles).To(ContainElement(handle))
				}

				By(fmt.Sprintf("not expiring the second build containers: %v", secondBuildContainerHandles))
				Consistently(func() int {
					var containerNum int
					err = Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"handle": secondBuildContainerHandles}).RunWith(DbConn).QueryRow().Scan(&containerNum)
					Expect(err).ToNot(HaveOccurred())

					return containerNum
				}, 2*time.Minute, time.Second).Should(Equal(len(secondBuildContainerHandles)))

				By("not removing the second build containers from the worker")
				for _, handle := range secondBuildContainerHandles {
					Expect(existingHandles).To(ContainElement(handle))
				}

				Fly.Run("abort-build", "-j", "build-container-gc/simple-job", "-b", "2")

				<-runningBuildSession.Exited
			})
		})
	})
})
