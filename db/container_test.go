package db_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
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

	var safelyCloseConection = func() {
		BeforeEach(func() {
			_ = dbConn.Close()
		})
		AfterEach(func() {
			dbConn = postgresRunner.OpenConn()
		})
	}

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
					creatingVolume1, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
					Expect(err).NotTo(HaveOccurred())
					_, err = creatingVolume1.Created()
					Expect(err).NotTo(HaveOccurred())
					expectedHandles = append(expectedHandles, creatingVolume1.Handle())

					creatingVolume2, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-2")
					Expect(err).NotTo(HaveOccurred())
					_, err = creatingVolume2.Created()
					Expect(err).NotTo(HaveOccurred())
					expectedHandles = append(expectedHandles, creatingVolume2.Handle())

					createdContainer, err = creatingContainer.Created()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns created container volumes", func() {
					volumes, err := volumeRepository.FindVolumesForContainer(createdContainer)
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

	Describe("Failed", func() {
		var failedContainer db.FailedContainer
		var failedErr error
		var failedContainerHandles []string

		Context("db conn is closed", func() {
			safelyCloseConection()
			It("returns an error", func() {
				failedContainer, failedErr = creatingContainer.Failed()
				Expect(failedContainer).To(BeNil())
				Expect(failedErr).To(HaveOccurred())
			})
		})

		Context("db conn is good", func() {
			JustBeforeEach(func() {
				By("transition to failed")
				failedContainer, failedErr = creatingContainer.Failed()

				rows, err := psql.Select("handle").
					From("containers").
					Where(sq.Eq{"state": atc.ContainerStateFailed}).
					RunWith(dbConn).
					Query()
				Expect(err).NotTo(HaveOccurred())

				for rows.Next() {
					var handle = "handle"
					columns := []interface{}{&handle}

					err = rows.Scan(columns...)
					Expect(err).NotTo(HaveOccurred())

					failedContainerHandles = append(failedContainerHandles, handle)
				}
			})

			BeforeEach(func() {
				failedErr = nil
				failedContainerHandles = []string{}
			})

			Context("when the container is in the creating state", func() {
				It("makes the state failed", func() {
					Expect(failedContainerHandles).To(HaveLen(1))
					Expect(failedContainerHandles).To(ContainElement(failedContainer.Handle()))
				})

				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})

			Context("when the container is already in failed state", func() {
				BeforeEach(func() {
					_, err := creatingContainer.Failed()
					Expect(err).ToNot(HaveOccurred())
				})

				It("keeps the state failed", func() {
					Expect(failedContainerHandles).To(HaveLen(1))
					Expect(failedContainerHandles).To(ContainElement(failedContainer.Handle()))
				})

				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})

			Context("when the container is actually in created state", func() {
				BeforeEach(func() {
					c, err := creatingContainer.Created()
					Expect(err).ToNot(HaveOccurred())
					Expect(c.Handle()).NotTo(BeNil())
				})

				It("does not mark it as failed", func() {
					Expect(failedContainerHandles).To(HaveLen(0))
					Expect(failedContainer).To(BeNil())
				})
			})

			Context("when the container is actually in destroying state", func() {
				BeforeEach(func() {
					createdContainer, err := creatingContainer.Created()
					Expect(err).ToNot(HaveOccurred())

					_, err = createdContainer.Destroying()
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not mark it as failed", func() {
					Expect(failedContainerHandles).To(HaveLen(0))
					Expect(failedContainer).To(BeNil())
				})

				It("returns an error", func() {
					Expect(failedErr).To(HaveOccurred())
				})
			})
		})
	})

	Describe("Destroy", func() {
		var destroyErr error
		var destroyed bool

		Context("called on a destroying container", func() {
			var destroyingContainer db.DestroyingContainer
			BeforeEach(func() {
				createdContainer, err := creatingContainer.Created()
				Expect(err).ToNot(HaveOccurred())
				destroyingContainer, err = createdContainer.Destroying()
				Expect(err).ToNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				destroyed, destroyErr = destroyingContainer.Destroy()
			})

			It("successfully removes the row from the db", func() {
				Expect(destroyed).To(BeTrue())
				Expect(destroyErr).ToNot(HaveOccurred())
			})

			Context("errors", func() {
				Context("when the db connection is closed", func() {
					safelyCloseConection()
					It("returns an error", func() {
						Expect(destroyErr).To(HaveOccurred())
					})
				})
				Context("when the container dissapears from the db", func() {
					BeforeEach(func() {
						_, err := destroyingContainer.Destroy()
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns an error", func() {
						Expect(destroyErr).To(HaveOccurred())
					})
				})
			})
		})

		Context("called on a failed container", func() {
			var failedContainer db.FailedContainer

			BeforeEach(func() {
				var err error
				failedContainer, err = creatingContainer.Failed()
				Expect(err).ToNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				destroyed, destroyErr = failedContainer.Destroy()
			})

			It("successfully removes the row from the db", func() {
				Expect(destroyed).To(BeTrue())
				Expect(destroyErr).ToNot(HaveOccurred())
			})

			Context("errors", func() {
				Context("when the db connection is closed", func() {
					safelyCloseConection()
					It("returns an error", func() {
						Expect(destroyErr).To(HaveOccurred())
					})
				})

				Context("when the container dissapears from the db", func() {
					BeforeEach(func() {
						_, err := failedContainer.Destroy()
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns an error", func() {
						Expect(destroyErr).To(HaveOccurred())
					})
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
