package dbng_test

import (
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		createdContainer dbng.CreatedContainer
		expectedHandles  []string
	)

	BeforeEach(func() {
		creatingContainer, err := containerFactory.CreateBuildContainer(defaultWorker, defaultBuild, "some-plan", dbng.ContainerMetadata{
			Type: "task",
			Name: "some-task",
		})
		Expect(err).ToNot(HaveOccurred())

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

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Volumes", func() {
		It("returns created container volumes", func() {
			volumes, err := volumeFactory.FindVolumesForContainer(createdContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(2))
			Expect([]string{volumes[0].Handle(), volumes[1].Handle()}).To(Equal(expectedHandles))
			Expect([]string{volumes[0].Path(), volumes[1].Path()}).To(ConsistOf("some-path-1", "some-path-2"))
		})
	})
})
