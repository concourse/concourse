package dbng_test

import (
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		dbConn           dbng.Conn
		volumeFactory    *dbng.VolumeFactory
		containerFactory *dbng.ContainerFactory
		teamFactory      *dbng.TeamFactory
		buildFactory     *dbng.BuildFactory

		createdContainer *dbng.CreatedContainer
		expectedHandles  []string
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		containerFactory = dbng.NewContainerFactory(dbConn)
		volumeFactory = dbng.NewVolumeFactory(dbConn)
		teamFactory = dbng.NewTeamFactory(dbConn)
		buildFactory = dbng.NewBuildFactory(dbConn)

		team, err := teamFactory.CreateTeam("some-team")
		Expect(err).ToNot(HaveOccurred())

		build, err := buildFactory.CreateOneOffBuild(team)
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

		creatingVolume1, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-1")
		Expect(err).NotTo(HaveOccurred())
		_, err = creatingVolume1.Created()
		Expect(err).NotTo(HaveOccurred())
		expectedHandles = append(expectedHandles, creatingVolume1.Handle())

		creatingVolume2, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-2")
		Expect(err).NotTo(HaveOccurred())
		_, err = creatingVolume2.Created()
		Expect(err).NotTo(HaveOccurred())
		expectedHandles = append(expectedHandles, creatingVolume2.Handle())

		createdContainer, err = creatingContainer.Created("some-handle")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Volumes", func() {
		It("returns created container volumes", func() {
			volumes, err := createdContainer.Volumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(2))
			Expect([]string{volumes[0].Handle(), volumes[1].Handle()}).To(Equal(expectedHandles))
			Expect([]string{volumes[0].Path(), volumes[1].Path()}).To(ConsistOf("some-path-1", "some-path-2"))
		})
	})
})
