package gcng_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeCollector", func() {
	var (
		volumeCollector gcng.VolumeCollector

		dbConn           dbng.Conn
		volumeFactory    dbng.VolumeFactory
		containerFactory *dbng.ContainerFactory
		teamFactory      dbng.TeamFactory
		workerFactory    dbng.WorkerFactory
		buildFactory     *dbng.BuildFactory
		fakeWorkerClient *workerfakes.FakeClient
		fakeWorker       *workerfakes.FakeWorker
		fakeBCVolume     *workerfakes.FakeVolume
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		containerFactory = dbng.NewContainerFactory(dbConn)
		volumeFactory = dbng.NewVolumeFactory(dbConn)
		teamFactory = dbng.NewTeamFactory(dbConn)
		buildFactory = dbng.NewBuildFactory(dbConn)
		workerFactory = dbng.NewWorkerFactory(dbConn)

		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
		fakeBCVolume = new(workerfakes.FakeVolume)
		fakeWorker.LookupVolumeReturns(fakeBCVolume, true, nil)
		fakeWorkerClient.WorkersReturns([]worker.Worker{fakeWorker}, nil)

		logger := lagertest.NewTestLogger("volume-collector")
		volumeCollector = gcng.NewVolumeCollector(
			logger,
			volumeFactory,
			fakeWorkerClient,
		)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Run", func() {
		BeforeEach(func() {
			team, err := teamFactory.CreateTeam("some-team")
			Expect(err).ToNot(HaveOccurred())

			build, err := buildFactory.CreateOneOffBuild(team)
			Expect(err).ToNot(HaveOccurred())

			worker, err := workerFactory.SaveWorker(atc.Worker{
				Name:       "some-worker",
				GardenAddr: "1.2.3.4:7777",
			}, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			creatingContainer1, err := containerFactory.CreateBuildContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingContainer2, err := containerFactory.CreateBuildContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer1, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			_, err = volumeFactory.CreateContainerVolume(team, worker, creatingContainer2, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			creatingVolume3, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer1, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			_, err = creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			_, err = createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())

			createdContainer1, err := containerFactory.ContainerCreated(creatingContainer1, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			destroyingContainer1, err := createdContainer1.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err := destroyingContainer1.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(BeTrue())
		})

		It("deletes created and destroying orphaned volumes", func() {
			createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(createdVolumes).To(HaveLen(1))
			Expect(destoryingVolumes).To(HaveLen(1))

			err = volumeCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			createdVolumes, destoryingVolumes, err = volumeFactory.GetOrphanedVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(createdVolumes).To(HaveLen(0))
			Expect(destoryingVolumes).To(HaveLen(0))

			Expect(fakeBCVolume.DestroyCallCount()).To(Equal(2))
		})
	})
})
