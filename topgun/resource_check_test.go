package topgun_test

import (
	"bytes"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource checking", func() {
	Context("with tags on the resource", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml", "-o", "operations/tagged-worker.yml")
			_ = waitForRunningWorker()

			By("setting a pipeline that has a tagged resource")
			fly.Run("set-pipeline", "-n", "-c", "pipelines/tagged-resource.yml", "-p", "tagged-resource")

			By("unpausing the pipeline pipeline")
			fly.Run("unpause-pipeline", "-p", "tagged-resource")
		})

		It("places the checking container on the tagged worker", func() {
			By("running the check")
			fly.Run("check-resource", "-r", "tagged-resource/some-resource")

			By("getting the worker name")
			workersTable := flyTable("workers")
			var taggedWorkerName string
			for _, w := range workersTable {
				if w["tags"] == "tagged" {
					taggedWorkerName = w["name"]
				}
			}
			Expect(taggedWorkerName).ToNot(BeEmpty())

			By("checking that the container is on the tagged worker")
			containerTable := flyTable("containers")
			Expect(containerTable).To(HaveLen(1))
			Expect(containerTable[0]["type"]).To(Equal("check"))
			Expect(containerTable[0]["worker"]).To(Equal(taggedWorkerName))
		})
	})

	Context("with a team worker and a global worker and there is a check container on the team worker", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml",
				"-o", "operations/add-other-worker.yml",
				"-o", "operations/other-worker-team.yml")

			By("setting a pipeline that has a resource")
			fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

			By("unpausing the pipeline")
			fly.Run("unpause-pipeline", "-p", "get-resource")

			By("running the check")
			fly.Run("check-resource", "-r", "get-resource/some-resource")
		})

		It("finds the check container on the team worker", func() {
			By("getting the worker name")
			workersTable := flyTable("workers")
			var teamWorkerName string
			for _, w := range workersTable {
				if w["team"] == "main" {
					teamWorkerName = w["name"]
				}
			}
			Expect(teamWorkerName).ToNot(BeEmpty())

			By("checking that the container is on the team worker")
			containerTable := flyTable("containers")
			Expect(containerTable).To(HaveLen(1))
			Expect(containerTable[0]["type"]).To(Equal("check"))
			Expect(containerTable[0]["worker"]).To(Equal(teamWorkerName))
		})

		Context("when another team sets the same resource", func() {
			BeforeEach(func() {
				By("logging back in to main team")
				fly.Login(atcUsername, atcPassword, atcExternalURL)

				By("creating another team")
				setTeamSession := fly.SpawnInteractive(
					bytes.NewBufferString("y\n"),
					"set-team",
					"--team-name", "team-b",
					"--local-user", "guest",
				)

				<-setTeamSession.Exited
				Expect(setTeamSession.ExitCode()).To(Equal(0))

				By("logging into other team")
				fly.Run("login", "-n", "team-b", "-u", "guest", "-p", "guest")

				By("setting a pipeline that has a resource")
				fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

				By("unpausing the pipeline")
				fly.Run("unpause-pipeline", "-p", "get-resource")
			})

			It("places the checking container on the global worker", func() {
				By("running the check")
				fly.Run("check-resource", "-r", "get-resource/some-resource")

				By("getting the worker name")
				workersTable := flyTable("workers")
				var globalWorkerName []string
				for _, w := range workersTable {
					globalWorkerName = append(globalWorkerName, w["name"])
				}
				Expect(globalWorkerName).ToNot(BeEmpty())
				Expect(globalWorkerName).To(HaveLen(1))

				By("checking that the container is on the global worker")
				containerTable := flyTable("containers")
				Expect(containerTable).To(HaveLen(1))
				Expect(containerTable[0]["type"]).To(Equal("check"))
				Expect(containerTable[0]["worker"]).To(Equal(globalWorkerName[0]))
			})
		})
	})

	Context("when there is one team worker and one global worker and there is a check container on the global worker", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml",
				"-o", "operations/add-other-worker.yml",
				"-o", "operations/other-worker-team.yml")

			By("creating another team")
			setTeamSession := fly.SpawnInteractive(
				bytes.NewBufferString("y\n"),
				"set-team",
				"--team-name", "team-b",
				"--local-user", "guest",
			)

			<-setTeamSession.Exited
			Expect(setTeamSession.ExitCode()).To(Equal(0))

			By("logging into other team")
			fly.Run("login", "-n", "team-b", "-u", "guest", "-p", "guest")

			By("setting a pipeline that has a resource")
			fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

			By("unpausing the pipeline")
			fly.Run("unpause-pipeline", "-p", "get-resource")

			By("running the check")
			fly.Run("check-resource", "-r", "get-resource/some-resource")

			By("getting the worker name")
			workersTable := flyTable("workers")
			var globalWorkerName []string
			for _, w := range workersTable {
				globalWorkerName = append(globalWorkerName, w["name"])
			}
			Expect(globalWorkerName).ToNot(BeEmpty())
			Expect(globalWorkerName).To(HaveLen(1))

			By("checking that the container is on the global worker")
			containerTable := flyTable("containers")
			Expect(containerTable).To(HaveLen(1))
			Expect(containerTable[0]["type"]).To(Equal("check"))
			Expect(containerTable[0]["worker"]).To(Equal(globalWorkerName[0]))
		})

		It("creates a new check container on the team worker", func() {
			By("logging into main team")
			fly.Run("login", "-n", "main", "-u", "test", "-p", "test")

			By("setting a pipeline that has a resource")
			fly.Run("set-pipeline", "-n", "-c", "pipelines/get-resource.yml", "-p", "get-resource")

			By("unpausing the pipeline")
			fly.Run("unpause-pipeline", "-p", "get-resource")

			By("running the check")
			fly.Run("check-resource", "-r", "get-resource/some-resource")

			By("getting the worker name")
			workersTable := flyTable("workers")
			var teamWorkerName string
			for _, w := range workersTable {
				if w["team"] == "main" {
					teamWorkerName = w["name"]
				}
			}
			Expect(teamWorkerName).ToNot(BeEmpty())

			By("checking that the container is on the team worker")
			containerTable := flyTable("containers")
			Expect(containerTable).To(HaveLen(2))

			type container struct {
				type_  string
				worker string
			}

			var containers []container
			for _, c := range containerTable {
				var con container
				con.type_ = c["type"]
				con.worker = c["worker"]

				containers = append(containers, con)
			}

			Expect(containers).To(ContainElement(container{type_: "check", worker: teamWorkerName}))
		})
	})
})
