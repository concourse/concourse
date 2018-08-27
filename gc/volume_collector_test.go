package gc_test

import (
	"context"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeCollector", func() {
	var (
		volumeCollector gc.Collector

		volumeRepository   db.VolumeRepository
		workerFactory      db.WorkerFactory
		creatingContainer1 db.CreatingContainer
		creatingContainer2 db.CreatingContainer
		team               db.Team
		worker             db.Worker
		build              db.Build
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		volumeRepository = db.NewVolumeRepository(dbConn)
		workerFactory = db.NewWorkerFactory(dbConn)

		volumeCollector = gc.NewVolumeCollector(
			volumeRepository,
		)
	})

	Describe("Run", func() {
		BeforeEach(func() {
			var err error
			team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			build, err = team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			worker, err = workerFactory.SaveWorker(atc.Worker{
				Name:            "some-worker",
				GardenAddr:      "1.2.3.4:7777",
				BaggageclaimURL: "1.2.3.4:7788",
			}, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			creatingContainer1, err = team.CreateContainer(worker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there are failed volumes", func() {
			JustBeforeEach(func() {
				creatingVolume1, err := volumeRepository.CreateContainerVolume(team.ID(), worker.Name(), creatingContainer1, "some-path-1")
				Expect(err).NotTo(HaveOccurred())

				_, err = creatingVolume1.Failed()
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes all the failed volumes from the database", func() {
				failedVolumesLen, err := volumeRepository.DestroyFailedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(failedVolumesLen).To(Equal(1))

				err = volumeCollector.Run(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				failedVolumesLen, err = volumeRepository.DestroyFailedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(failedVolumesLen).To(Equal(0))
			})
		})

		Context("when there are orphaned volumes", func() {
			var expectedOrphanedVolumeHandles []string

			JustBeforeEach(func() {
				creatingContainer2, err = team.CreateContainer(worker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
					Type:     "task",
					StepName: "some-task",
				})
				Expect(err).ToNot(HaveOccurred())

				creatingVolume1, err := volumeRepository.CreateContainerVolume(team.ID(), worker.Name(), creatingContainer1, "some-path-1")
				Expect(err).NotTo(HaveOccurred())
				expectedOrphanedVolumeHandles = append(expectedOrphanedVolumeHandles, creatingVolume1.Handle())

				_, err = creatingVolume1.Created()
				Expect(err).NotTo(HaveOccurred())

				creatingVolume2, err := volumeRepository.CreateContainerVolume(team.ID(), worker.Name(), creatingContainer2, "some-path-1")
				Expect(err).NotTo(HaveOccurred())

				_, err = creatingVolume2.Created()
				Expect(err).NotTo(HaveOccurred())

				createdContainer1, err := creatingContainer1.Created()
				Expect(err).NotTo(HaveOccurred())

				_, err = creatingContainer2.Created()
				Expect(err).NotTo(HaveOccurred())

				destroyingContainer, err := createdContainer1.Destroying()
				Expect(err).NotTo(HaveOccurred())

				destroyed, err := destroyingContainer.Destroy()
				Expect(err).NotTo(HaveOccurred())
				Expect(destroyed).To(BeTrue())
			})

			It("marks orphaned volumes as 'destroying'", func() {
				err = volumeCollector.Run(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				destroyingVolumes, err := volumeRepository.GetDestroyingVolumes(worker.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(destroyingVolumes).To(HaveLen(1))

				Expect(destroyingVolumes).To(Equal(expectedOrphanedVolumeHandles))
			})

		})
	})
})
