package dbng_test

import (
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeFactory", func() {
	var (
		dbConn           dbng.Conn
		volumeFactory    *dbng.VolumeFactory
		containerFactory *dbng.ContainerFactory
		teamFactory      *dbng.TeamFactory
		buildFactory     *dbng.BuildFactory
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		containerFactory = dbng.NewContainerFactory(dbConn)
		volumeFactory = dbng.NewVolumeFactory(dbConn)
		teamFactory = dbng.NewTeamFactory(dbConn)
		buildFactory = dbng.NewBuildFactory(dbConn)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetOrphanedVolumes", func() {
		var (
			volume1 *dbng.InitializedVolume
			volume2 *dbng.InitializedVolume
			volume3 *dbng.DestroyingVolume
			volume4 *dbng.InitializedVolume
			build   *dbng.Build
		)

		BeforeEach(func() {
			team, err := teamFactory.CreateTeam("some-team")
			Expect(err).ToNot(HaveOccurred())

			build, err = buildFactory.CreateOneOffBuild(team)
			Expect(err).ToNot(HaveOccurred())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			worker := &dbng.Worker{
				Name:       "some-worker",
				GardenAddr: "1.2.3.4:7777",
			}
			err = worker.Create(setupTx)
			Expect(err).ToNot(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			creatingContainer, err := containerFactory.CreateTaskContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer)
			Expect(err).NotTo(HaveOccurred())
			creatingVolume2, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer)
			Expect(err).NotTo(HaveOccurred())
			creatingVolume3, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer)
			Expect(err).NotTo(HaveOccurred())
			creatingVolume4, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer)
			Expect(err).NotTo(HaveOccurred())

			createdVolume1, err := creatingVolume1.Created("some-handle-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created("some-handle-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created("some-handle-3")
			Expect(err).NotTo(HaveOccurred())

			initializingVolume1, err := createdVolume1.Initializing(creatingContainer)
			Expect(err).NotTo(HaveOccurred())
			initializingVolume2, err := createdVolume2.Initializing(creatingContainer)
			Expect(err).NotTo(HaveOccurred())
			initializingVolume3, err := createdVolume3.Initializing(creatingContainer)
			Expect(err).NotTo(HaveOccurred())

			initializedVolume1, err := initializingVolume1.Initialized()
			Expect(err).NotTo(HaveOccurred())
			initializedVolume2, err := initializingVolume2.Initialized()
			Expect(err).NotTo(HaveOccurred())
			initializedVolume3, err := initializingVolume3.Initialized()
			Expect(err).NotTo(HaveOccurred())
			initializedVolume4, err := creatingVolume4.Initialized("some-handle-4")
			Expect(err).NotTo(HaveOccurred())

			destroyingVolume3, err := initializedVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())

			volume1 = initializedVolume1
			volume2 = initializedVolume2
			volume3 = destroyingVolume3
			volume4 = initializedVolume4

			createdContainer, err := creatingContainer.Created("some-handle")
			Expect(err).NotTo(HaveOccurred())
			destroyingContainer, err := createdContainer.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err := destroyingContainer.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(BeTrue())
		})

		It("returns orphaned volumes", func() {
			initializedVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
			Expect(err).NotTo(HaveOccurred())
			initializedHandles := []string{}
			for _, vol := range initializedVolumes {
				initializedHandles = append(initializedHandles, vol.Handle)
			}
			Expect(initializedHandles).To(ConsistOf("some-handle-1", "some-handle-2", "some-handle-4"))

			destoryingHandles := []string{}
			for _, vol := range destoryingVolumes {
				destoryingHandles = append(destoryingHandles, vol.Handle)
			}
			Expect(destoryingHandles).To(ConsistOf("some-handle-3"))
		})
	})
})

// 	Describe("CreateWorkerResourceTypeVolume", func() {
// 		var worker dbng.Worker
// 		var wrt dbng.WorkerResourceType

// 		BeforeEach(func() {
// 			worker = dbng.Worker{
// 				Name:       "some-worker",
// 				GardenAddr: "1.2.3.4:7777",
// 			}

// 			wrt = dbng.WorkerResourceType{
// 				WorkerName: worker.Name,
// 				Type:       "some-worker-resource-type",
// 				Image:      "some-worker-resource-image",
// 				Version:    "some-worker-resource-version",
// 			}
// 		})

// 		Context("when the worker resource type exists", func() {
// 			BeforeEach(func() {
// 				setupTx, err := dbConn.Begin()
// 				Expect(err).ToNot(HaveOccurred())

// 				defer setupTx.Rollback()

// 				err = worker.Create(setupTx)
// 				Expect(err).ToNot(HaveOccurred())

// 				_, err = wrt.Create(setupTx)
// 				Expect(err).ToNot(HaveOccurred())

// 				Expect(setupTx.Commit()).To(Succeed())
// 			})

// 			It("returns the created volume", func() {
// 				volume, err := factory.CreateWorkerResourceTypeVolume(wrt)
// 				Expect(err).ToNot(HaveOccurred())
// 				Expect(volume.ID).ToNot(BeZero())
// 			})
// 		})

// 		Context("when the worker resource type does not exist", func() {
// 			It("returns ErrWorkerResourceTypeNotFound", func() {
// 				_, err := factory.CreateWorkerResourceTypeVolume(wrt)
// 				Expect(err).To(Equal(dbng.ErrWorkerResourceTypeNotFound))
// 			})
// 		})
// 	})
// })
