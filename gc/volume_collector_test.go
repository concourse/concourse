package gc_test

import (
	"context"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/gc"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim/baggageclaimfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeCollector", func() {
	var (
		volumeCollector gc.Collector

		volumeRepository   db.VolumeRepository
		workerFactory      db.WorkerFactory
		fakeBCVolume       *baggageclaimfakes.FakeVolume
		creatingContainer1 db.CreatingContainer
		team               db.Team
		worker             db.Worker

		fakeWorker *workerfakes.FakeWorker
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		volumeRepository = db.NewVolumeRepository(dbConn)
		workerFactory = db.NewWorkerFactory(dbConn)

		fakeBCVolume = new(baggageclaimfakes.FakeVolume)

		fakeWorker = new(workerfakes.FakeWorker)

		volumeCollector = gc.NewVolumeCollector(
			volumeRepository,
		)
	})

	Describe("Run", func() {

		Context("when there are failed volumes", func() {
			var failedVolume1 db.FailedVolume

			BeforeEach(func() {
				var err error
				team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
				Expect(err).ToNot(HaveOccurred())

				build, err := team.CreateOneOffBuild()
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

				creatingVolume1, err := volumeRepository.CreateContainerVolume(team.ID(), worker.Name(), creatingContainer1, "some-path-1")
				Expect(err).NotTo(HaveOccurred())

				failedVolume1, err = creatingVolume1.Failed()
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
	})
})
