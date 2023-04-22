package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource checking", func() {
	Context("with a global worker and a tagged", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml",
				"-o", "operations/add-other-worker.yml",
				"-o", "operations/tagged-worker.yml")
		})

		Describe("tagged resources", func() {
			BeforeEach(func() {
				By("setting a pipeline that has a tagged resource")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/tagged-resource.yml", "-p", "tagged-resource")

				By("unpausing the pipeline pipeline")
				Fly.Run("unpause-pipeline", "-p", "tagged-resource")
			})

			It("places and finds the checking container on the tagged worker", func() {
				By("running the check")
				Fly.Run("check-resource", "-r", "tagged-resource/some-resource")

				By("getting the worker name")
				workersTable := FlyTable("workers")
				var taggedWorkerName string
				for _, w := range workersTable {
					if w["tags"] == "tagged" {
						taggedWorkerName = w["name"]
					}
				}
				Expect(taggedWorkerName).ToNot(BeEmpty())

				By("checking that the container is on the tagged worker")
				containerTable := FlyTable("containers")
				Expect(containerTable).To(HaveLen(1))
				Expect(containerTable[0]["type"]).To(Equal("check"))
				Expect(containerTable[0]["worker"]).To(Equal(taggedWorkerName))

				By("running the check")
				Fly.Run("check-resource", "-r", "tagged-resource/some-resource")

				By("checking that the container is on the tagged worker")
				containerTable = FlyTable("containers")
				Expect(containerTable).To(HaveLen(1))
				Expect(containerTable[0]["type"]).To(Equal("check"))
				Expect(containerTable[0]["worker"]).To(Equal(taggedWorkerName))
			})
		})
	})

	Context("with a global worker and a team worker", func() {
		var teamWorkerName string
		var globalWorkerName string

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/add-other-worker.yml",
				"-o", "operations/other-worker-team.yml",
			)

			Eventually(func() []map[string]string {
				workersTable := FlyTable("workers")

				for _, w := range workersTable {
					if w["team"] == "main" {
						teamWorkerName = w["name"]
					} else {
						globalWorkerName = w["name"]
					}
				}

				return workersTable
			}).Should(HaveLen(2))

			Expect(teamWorkerName).ToNot(BeEmpty(), "team worker not found")
			Expect(globalWorkerName).ToNot(BeEmpty(), "global worker not found")
		})

		Context("when a team WITH its own worker checks first", func() {
			BeforeEach(func() {
				By("setting a pipeline that has a resource")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "get-resource")

				By("running the check")
				Fly.Run("check-resource", "-r", "get-resource/some-resource")
			})

			It("places the check container on the team worker", func() {
				containerTable := FlyTable("containers")
				Expect(containerTable).To(HaveLen(1))
				Expect(containerTable[0]["type"]).To(Equal("check"))
				Expect(containerTable[0]["worker"]).To(Equal(teamWorkerName))
			})

			Context("when another team WITHOUT its own worker sets the same resource", func() {
				BeforeEach(func() {
					By("creating another team")
					Fly.Run(
						"set-team",
						"--non-interactive",
						"--team-name", "some-team",
						"--local-user", "guest",
					)

					By("logging into other team")
					Fly.Run("login", "-n", "some-team", "-u", "guest", "-p", "guest")

					By("setting a pipeline that has a resource")
					Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

					By("unpausing the pipeline")
					Fly.Run("unpause-pipeline", "-p", "get-resource")
				})

				It("creates a new check container on the global worker", func() {
					By("running the check")
					Fly.Run("check-resource", "-r", "get-resource/some-resource")

					By("checking that the container is on the global worker")
					containerTable := FlyTable("containers")
					Expect(containerTable).To(HaveLen(1))
					Expect(containerTable[0]["type"]).To(Equal("check"))
					Expect(containerTable[0]["worker"]).To(Equal(globalWorkerName))
				})
			})
		})

		Context("when a team WITHOUT its own worker checks first", func() {
			BeforeEach(func() {
				By("creating another team")
				Fly.Run(
					"set-team",
					"--non-interactive",
					"--team-name", "some-team",
					"--local-user", "guest",
				)

				By("logging in to the other team")
				Fly.Run("login", "-n", "some-team", "-u", "guest", "-p", "guest")

				By("setting a pipeline that has a resource")
				Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "get-resource")

				By("running the check")
				Fly.Run("check-resource", "-r", "get-resource/some-resource")
			})

			It("places the check container on the global worker", func() {
				containerTable := FlyTable("containers")
				Expect(containerTable).To(HaveLen(1))
				Expect(containerTable[0]["type"]).To(Equal("check"))
				Expect(containerTable[0]["worker"]).To(Equal(globalWorkerName))
			})

			Context("when another team WITH its own worker sets the same resource", func() {
				var globalHandle string

				BeforeEach(func() {
					containerTable := FlyTable("containers")
					Expect(containerTable).To(HaveLen(1))
					Expect(containerTable[0]["type"]).To(Equal("check"))
					Expect(containerTable[0]["worker"]).To(Equal(globalWorkerName))

					globalHandle = containerTable[0]["handle"]

					By("logging in to the other team")
					Fly.Run("login", "-n", "main", "-u", "test", "-p", "test")

					By("setting a pipeline that has a resource")
					Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")
				})

				It("creates a new check container on the team worker", func() {
					By("unpausing the pipeline")
					Fly.Run("unpause-pipeline", "-p", "get-resource")

					By("running the check")
					Fly.Run("check-resource", "-r", "get-resource/some-resource")

					By("having created a new container on the team worker")
					containerTable := FlyTable("containers")
					Expect(containerTable).To(HaveLen(2))
					for _, c := range containerTable {
						if c["handle"] == globalHandle {
							// this container is still visible since it's on a global worker
							// and relates to the same resource config
							continue
						}

						Expect(c["type"]).To(Equal("check"))
						Expect(c["worker"]).To(Equal(teamWorkerName))
					}
				})
			})
		})
	})
})
