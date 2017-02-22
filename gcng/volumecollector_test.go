package gcng_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"
	"github.com/concourse/atc/gcng/gcngfakes"
	"github.com/concourse/baggageclaim/baggageclaimfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeCollector", func() {
	var (
		volumeCollector gcng.Collector

		volumeFactory          dbng.VolumeFactory
		containerFactory       dbng.ContainerFactory
		workerFactory          dbng.WorkerFactory
		fakeBCVolume           *baggageclaimfakes.FakeVolume
		fakeBaggageclaimClient *baggageclaimfakes.FakeClient
		createdVolume          dbng.CreatedVolume
		creatingContainer1     dbng.CreatingContainer
		creatingContainer2     dbng.CreatingContainer
		team                   dbng.Team
		worker                 dbng.Worker
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		containerFactory = dbng.NewContainerFactory(dbConn)
		volumeFactory = dbng.NewVolumeFactory(dbConn)
		workerFactory = dbng.NewWorkerFactory(dbConn)

		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeBaggageclaimClientFactory := new(gcngfakes.FakeBaggageclaimClientFactory)
		fakeBaggageclaimClientFactory.NewClientReturns(fakeBaggageclaimClient)

		fakeBCVolume = new(baggageclaimfakes.FakeVolume)
		fakeBaggageclaimClient.LookupVolumeReturns(fakeBCVolume, true, nil)

		logger := lagertest.NewTestLogger("volume-collector")
		volumeCollector = gcng.NewVolumeCollector(
			logger,
			volumeFactory,
			fakeBaggageclaimClientFactory,
		)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Run", func() {
		BeforeEach(func() {
			var err error
			team, err = teamFactory.CreateTeam("some-team")
			Expect(err).ToNot(HaveOccurred())

			build, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			worker, err = workerFactory.SaveWorker(atc.Worker{
				Name:            "some-worker",
				GardenAddr:      "1.2.3.4:7777",
				BaggageclaimURL: "1.2.3.4:7788",
			}, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			creatingContainer1, err = team.CreateBuildContainer(worker.Name(), build.ID(), "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingContainer2, err = team.CreateBuildContainer(worker.Name(), build.ID(), "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeFactory.CreateContainerVolume(team.ID(), worker, creatingContainer1, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume, err = creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())

			_, err = volumeFactory.CreateContainerVolume(team.ID(), worker, creatingContainer2, "some-path-2")
			Expect(err).NotTo(HaveOccurred())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(team.ID(), worker, creatingContainer1, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			_, err = createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())

			createdContainer1, err := creatingContainer1.Created()
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

		Context("when destroying the volume in db fails because volume has children", func() {
			BeforeEach(func() {
				_, err := createdVolume.CreateChildForContainer(creatingContainer2, "some-path-1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("leaves the volume in the db", func() {
				createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolumes).To(HaveLen(1))
				createdVolumeHandle := createdVolumes[0].Handle()
				Expect(destoryingVolumes).To(HaveLen(1))

				err = volumeCollector.Run()
				Expect(err).NotTo(HaveOccurred())

				createdVolumes, destoryingVolumes, err = volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolumes).To(HaveLen(1))
				Expect(destoryingVolumes).To(HaveLen(0))
				Expect(createdVolumes[0].Handle()).To(Equal(createdVolumeHandle))
			})
		})

		Context("when destroying the volume in baggageclaim fails", func() {
			BeforeEach(func() {
				fakeBCVolume.DestroyReturns(errors.New("oh no!"))
			})

			It("leaves the volume in the db", func() {
				createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolumes).To(HaveLen(1))
				Expect(destoryingVolumes).To(HaveLen(1))

				err = volumeCollector.Run()
				Expect(err).NotTo(HaveOccurred())

				createdVolumes, destoryingVolumes, err = volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolumes).To(HaveLen(0))
				Expect(destoryingVolumes).To(HaveLen(2))
			})
		})
	})
})
