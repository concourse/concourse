package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team", func() {

	var otherTeam dbng.Team

	BeforeEach(func() {
		otherTeam, err = teamFactory.CreateTeam("some-other-team")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("SaveWorker", func() {
		var (
			team        dbng.Team
			otherTeam   dbng.Team
			teamFactory dbng.TeamFactory

			atcWorker atc.Worker
		)

		BeforeEach(func() {
			var err error
			teamFactory = dbng.NewTeamFactory(dbConn)
			team, err = teamFactory.CreateTeam("team")
			Expect(err).NotTo(HaveOccurred())
			otherTeam, err = teamFactory.CreateTeam("otherTeam")
			Expect(err).NotTo(HaveOccurred())

			atcWorker = atc.Worker{
				GardenAddr:       "some-garden-addr",
				BaggageclaimURL:  "some-bc-url",
				HTTPProxyURL:     "some-http-proxy-url",
				HTTPSProxyURL:    "some-https-proxy-url",
				NoProxy:          "some-no-proxy",
				ActiveContainers: 140,
				ResourceTypes: []atc.WorkerResourceType{
					{
						Type:    "some-resource-type",
						Image:   "some-image",
						Version: "some-version",
					},
					{
						Type:    "other-resource-type",
						Image:   "other-image",
						Version: "other-version",
					},
				},
				Platform:  "some-platform",
				Tags:      atc.Tags{"some", "tags"},
				Name:      "some-name",
				StartTime: 55,
			}
		})

		Context("the worker already exists", func() {
			Context("the worker is not in stalled state", func() {
				Context("the team_id of the new worker is the same", func() {
					BeforeEach(func() {
						_, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})
					It("overwrites all the data", func() {
						atcWorker.GardenAddr = "new-garden-addr"
						savedWorker, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
						Expect(savedWorker.Name).To(Equal("some-name"))
						Expect(*savedWorker.GardenAddr).To(Equal("new-garden-addr"))
						Expect(savedWorker.State).To(Equal(dbng.WorkerStateRunning))
					})
				})
				Context("the team_id of the new worker is different", func() {
					BeforeEach(func() {
						_, err := otherTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})
					It("errors", func() {
						_, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("FindContainerByHandle", func() {
		var createdContainer dbng.CreatedContainer

		BeforeEach(func() {
			build, err := defaultPipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker, build, atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
			Expect(err).NotTo(HaveOccurred())

			createdContainer, err = creatingContainer.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when worker is no longer in database", func() {
			BeforeEach(func() {
				err = workerFactory.DeleteWorker(defaultWorker.Name)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the container goes away from the db", func() {
				_, found, err := defaultTeam.FindContainerByHandle(createdContainer.Handle())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		It("finds a container for the team", func() {
			container, found, err := defaultTeam.FindContainerByHandle(createdContainer.Handle())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(container).ToNot(BeNil())
			Expect(container.Handle()).To(Equal(createdContainer.Handle()))
		})

		It("does not find container for another team", func() {
			_, found, err := otherTeam.FindContainerByHandle(createdContainer.Handle())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("FindResourceCheckContainer", func() {
		var resourceConfig *dbng.UsedResourceConfig

		BeforeEach(func() {
			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfigForResource(
				logger,
				defaultResource,
				"some-base-resource-type",
				atc.Source{"some": "source"},
				defaultPipeline.ID(),
				atc.ResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).NotTo(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				_, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).NotTo(BeNil())
				Expect(creatingContainer).To(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainer(defaultWorker, resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).To(BeNil())
			})
		})
	})

	Describe("FindResourceGetContainer", func() {
		var containerMetadata dbng.ContainerMetadata

		BeforeEach(func() {
			containerMetadata = dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			}
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).NotTo(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				_, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).NotTo(BeNil())
				Expect(creatingContainer).To(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindBuildContainer(defaultWorker, defaultBuild, "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).To(BeNil())
			})
		})
	})
})
