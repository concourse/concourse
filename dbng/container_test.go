package dbng_test

import (
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		creatingContainer dbng.CreatingContainer
		createdContainer  dbng.CreatedContainer
		expectedHandles   []string
	)

	BeforeEach(func() {
		var err error
		creatingContainer, err = defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", dbng.ContainerMetadata{
			Type: "task",
			Name: "some-task",
		})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Volumes", func() {
		BeforeEach(func() {
			creatingVolume1, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			_, err = creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedHandles = append(expectedHandles, creatingVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			_, err = creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedHandles = append(expectedHandles, creatingVolume2.Handle())

			createdContainer, err = creatingContainer.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns created container volumes", func() {
			volumes, err := volumeFactory.FindVolumesForContainer(createdContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(2))
			Expect([]string{volumes[0].Handle(), volumes[1].Handle()}).To(Equal(expectedHandles))
			Expect([]string{volumes[0].Path(), volumes[1].Path()}).To(ConsistOf("some-path-1", "some-path-2"))
		})
	})

	Describe("Created", func() {
		Context("when the container is already created", func() {
			BeforeEach(func() {
				_, err := creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a created container and no error", func() {
				createdContainer, err := creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).NotTo(BeNil())
			})
		})
	})

	Describe("Destroying", func() {
		Context("when the container is already in destroying state", func() {
			var createdContainer dbng.CreatedContainer

			BeforeEach(func() {
				var err error
				createdContainer, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
				_, err = createdContainer.Destroying()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a destroying container and no error", func() {
				destroyingContainer, err := createdContainer.Destroying()
				Expect(err).NotTo(HaveOccurred())
				Expect(destroyingContainer).NotTo(BeNil())
			})
		})
	})

	Describe("Discontinue", func() {
		Context("when the container is already in destroying state", func() {
			var createdContainer dbng.CreatedContainer

			BeforeEach(func() {
				var err error
				createdContainer, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
				_, err = createdContainer.Discontinue()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a discontinued container and no error", func() {
				destroyingContainer, err := createdContainer.Discontinue()
				Expect(err).NotTo(HaveOccurred())
				Expect(destroyingContainer).NotTo(BeNil())
			})
		})
	})
})
