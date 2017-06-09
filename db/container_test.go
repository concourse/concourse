package db_test

import (
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		creatingContainer db.CreatingContainer
		expectedHandles   []string
		build             db.Build
	)

	BeforeEach(func() {
		var err error
		build, err = defaultTeam.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())

		creatingContainer, err = defaultTeam.CreateContainer(
			defaultWorker.Name(),
			db.NewBuildStepContainerOwner(build.ID(), "some-plan"),
			fullMetadata,
		)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Metadata", func() {
		It("returns the container metadata", func() {
			Expect(creatingContainer.Metadata()).To(Equal(fullMetadata))
		})
	})

	Describe("Created", func() {
		Context("when the container is already created", func() {
			var createdContainer db.CreatedContainer

			BeforeEach(func() {
				var err error
				createdContainer, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a created container and no error", func() {
				Expect(createdContainer).NotTo(BeNil())
			})

			Describe("Volumes", func() {
				BeforeEach(func() {
					creatingVolume1, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
					Expect(err).NotTo(HaveOccurred())
					_, err = creatingVolume1.Created()
					Expect(err).NotTo(HaveOccurred())
					expectedHandles = append(expectedHandles, creatingVolume1.Handle())

					creatingVolume2, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-2")
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
					Expect([]string{volumes[0].Handle(), volumes[1].Handle()}).To(ConsistOf(expectedHandles))
					Expect([]string{volumes[0].Path(), volumes[1].Path()}).To(ConsistOf("some-path-1", "some-path-2"))
				})
			})

			Describe("Metadata", func() {
				It("returns the container metadata", func() {
					Expect(createdContainer.Metadata()).To(Equal(fullMetadata))
				})
			})
		})
	})

	Describe("Destroying", func() {
		Context("when the container is already in destroying state", func() {
			var createdContainer db.CreatedContainer
			var destroyingContainer db.DestroyingContainer

			BeforeEach(func() {
				var err error
				createdContainer, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())

				destroyingContainer, err = createdContainer.Destroying()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a destroying container and no error", func() {
				Expect(destroyingContainer).NotTo(BeNil())
			})

			Describe("Metadata", func() {
				It("returns the container metadata", func() {
					Expect(destroyingContainer.Metadata()).To(Equal(fullMetadata))
				})
			})
		})
	})

	Describe("Discontinue", func() {
		Context("when the container is already in destroying state", func() {
			var createdContainer db.CreatedContainer

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
