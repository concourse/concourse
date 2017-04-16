package dbng_test

import (
	"encoding/json"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team", func() {
	var (
		team      dbng.Team
		otherTeam dbng.Team
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())
		otherTeam, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			team, found, err := teamFactory.FindTeam("some-other-team")
			Expect(team.Name()).To(Equal("some-other-team"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			err = otherTeam.Delete()
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes the team", func() {
			team, found, err := teamFactory.FindTeam("some-other-team")
			Expect(team).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("SaveWorker", func() {
		var (
			team      dbng.Team
			otherTeam dbng.Team
			atcWorker atc.Worker
			err       error
		)

		BeforeEach(func() {
			postgresRunner.Truncate()
			team, err = teamFactory.CreateTeam(atc.Team{Name: "team"})
			Expect(err).NotTo(HaveOccurred())

			otherTeam, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
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
						Expect(savedWorker.Name()).To(Equal("some-name"))
						Expect(*savedWorker.GardenAddr()).To(Equal("new-garden-addr"))
						Expect(savedWorker.State()).To(Equal(dbng.WorkerStateRunning))
					})
				})
				Context("the team_id of the new worker is different", func() {
					BeforeEach(func() {
						_, err = otherTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})
					It("errors", func() {
						_, err = team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("Workers", func() {
		var (
			team      dbng.Team
			otherTeam dbng.Team
			atcWorker atc.Worker
			err       error
		)

		BeforeEach(func() {
			postgresRunner.Truncate()
			team, err = teamFactory.CreateTeam(atc.Team{Name: "team"})
			Expect(err).NotTo(HaveOccurred())

			otherTeam, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
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

		Context("when there are global workers and workers for the team", func() {
			BeforeEach(func() {
				_, err = team.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "some-new-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-bc-url"
				_, err = workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds them without error", func() {
				workers, err := team.Workers()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(workers)).To(Equal(2))

				Expect(workers[0].Name()).To(Equal("some-name"))
				Expect(*workers[0].GardenAddr()).To(Equal("some-garden-addr"))
				Expect(*workers[0].BaggageclaimURL()).To(Equal("some-bc-url"))

				Expect(workers[1].Name()).To(Equal("some-new-worker"))
				Expect(*workers[1].GardenAddr()).To(Equal("some-other-garden-addr"))
				Expect(*workers[1].BaggageclaimURL()).To(Equal("some-other-bc-url"))
			})
		})

		Context("when there are workers for another team", func() {
			BeforeEach(func() {
				atcWorker.Name = "some-other-team-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-bc-url"
				_, err = otherTeam.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not find the other team workers", func() {
				workers, err := team.Workers()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(workers)).To(Equal(0))
			})
		})

		Context("when there are no workers", func() {
			It("returns an error", func() {
				workers, err := workerFactory.Workers()
				Expect(err).NotTo(HaveOccurred())
				Expect(workers).To(BeEmpty())
			})
		})
	})

	Describe("FindContainersByMetadata", func() {
		var sampleMetadata []dbng.ContainerMetadata
		var metaContainers map[dbng.ContainerMetadata][]dbng.Container

		BeforeEach(func() {
			baseMetadata := fullMetadata

			diffType := fullMetadata
			diffType.Type = dbng.ContainerTypeCheck

			diffStepName := fullMetadata
			diffStepName.StepName = fullMetadata.StepName + "-other"

			diffAttempt := fullMetadata
			diffAttempt.Attempt = fullMetadata.Attempt + ",2"

			diffPipelineID := fullMetadata
			diffPipelineID.PipelineID = fullMetadata.PipelineID + 1

			diffJobID := fullMetadata
			diffJobID.JobID = fullMetadata.JobID + 1

			diffBuildID := fullMetadata
			diffBuildID.BuildID = fullMetadata.BuildID + 1

			diffWorkingDirectory := fullMetadata
			diffWorkingDirectory.WorkingDirectory = fullMetadata.WorkingDirectory + "/other"

			diffUser := fullMetadata
			diffUser.User = fullMetadata.User + "-other"

			sampleMetadata = []dbng.ContainerMetadata{
				baseMetadata,
				diffType,
				diffStepName,
				diffAttempt,
				diffPipelineID,
				diffJobID,
				diffBuildID,
				diffWorkingDirectory,
				diffUser,
			}

			build, err := defaultPipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			metaContainers = make(map[dbng.ContainerMetadata][]dbng.Container)
			for _, meta := range sampleMetadata {
				firstContainerCreating, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), meta)
				Expect(err).NotTo(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], firstContainerCreating)

				secondContainerCreating, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), meta)
				Expect(err).NotTo(HaveOccurred())

				secondContainerCreated, err := secondContainerCreating.Created()
				Expect(err).NotTo(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], secondContainerCreated)

				thirdContainerCreating, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), meta)
				Expect(err).NotTo(HaveOccurred())

				thirdContainerCreated, err := thirdContainerCreating.Created()
				Expect(err).NotTo(HaveOccurred())

				// third container is not appended; we don't want Destroying containers
				thirdContainerDestroying, err := thirdContainerCreated.Destroying()
				Expect(err).NotTo(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], thirdContainerDestroying)
			}
		})

		It("finds creating, created, and destroying containers for the team, matching the metadata in full", func() {
			for _, meta := range sampleMetadata {
				expectedHandles := []string{}
				for _, c := range metaContainers[meta] {
					expectedHandles = append(expectedHandles, c.Handle())
				}

				containers, err := defaultTeam.FindContainersByMetadata(meta)
				Expect(err).ToNot(HaveOccurred())

				foundHandles := []string{}
				for _, c := range containers {
					foundHandles = append(foundHandles, c.Handle())
				}

				// should always find a Creating container and a Created container
				Expect(foundHandles).To(HaveLen(3))
				Expect(foundHandles).To(ConsistOf(expectedHandles))
			}
		})

		It("finds containers for the team, matching partial metadata", func() {
			containers, err := defaultTeam.FindContainersByMetadata(dbng.ContainerMetadata{
				Type: dbng.ContainerTypeTask,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).ToNot(BeEmpty())

			foundHandles := []string{}
			for _, c := range containers {
				foundHandles = append(foundHandles, c.Handle())
			}

			var notFound int
			for meta, cs := range metaContainers {
				if meta.Type == dbng.ContainerTypeTask {
					for _, c := range cs {
						Expect(foundHandles).To(ContainElement(c.Handle()))
					}
				} else {
					for _, c := range cs {
						Expect(foundHandles).ToNot(ContainElement(c.Handle()))
						notFound++
					}
				}
			}

			// just to assert test setup is valid
			Expect(notFound).ToNot(BeZero())
		})

		It("finds all containers for the team when given empty metadata", func() {
			containers, err := defaultTeam.FindContainersByMetadata(dbng.ContainerMetadata{})
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).ToNot(BeEmpty())

			foundHandles := []string{}
			for _, c := range containers {
				foundHandles = append(foundHandles, c.Handle())
			}

			for _, cs := range metaContainers {
				for _, c := range cs {
					Expect(foundHandles).To(ContainElement(c.Handle()))
				}
			}
		})

		It("does not find containers for other teams", func() {
			for _, meta := range sampleMetadata {
				containers, err := otherTeam.FindContainersByMetadata(meta)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(BeEmpty())
			}
		})
	})

	Describe("FindContainerByHandle", func() {
		var createdContainer dbng.CreatedContainer

		BeforeEach(func() {
			build, err := defaultPipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", StepName: "some-task"})
			Expect(err).NotTo(HaveOccurred())

			createdContainer, err = creatingContainer.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when worker is no longer in database", func() {
			BeforeEach(func() {
				err := defaultWorker.Delete()
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

	Describe("FindWorkerForResourceCheckContainer", func() {
		var resourceConfig *dbng.UsedResourceConfig

		BeforeEach(func() {
			var err error
			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
				logger,
				dbng.ForResource(defaultResource.ID()),
				"some-base-resource-type",
				atc.Source{"some": "source"},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig, dbng.ContainerMetadata{Type: "check"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForResourceCheckContainer(resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).NotTo(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForResourceCheckContainer(resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			var originalCreatedContainer dbng.CreatedContainer

			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig, dbng.ContainerMetadata{Type: "check"})
				Expect(err).NotTo(HaveOccurred())
				originalCreatedContainer, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForResourceCheckContainer(resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).NotTo(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForResourceCheckContainer(resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})

			Context("when container is expired", func() {
				BeforeEach(func() {
					_, err := psql.Update("containers").
						Set("best_if_used_by", sq.Expr("NOW() - '1 second'::INTERVAL")).
						Where(sq.Eq{"id": originalCreatedContainer.ID()}).
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find it", func() {
					worker, found, err := defaultTeam.FindWorkerForResourceCheckContainer(resourceConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
					Expect(worker).To(BeNil())
				})
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				worker, found, err := defaultTeam.FindWorkerForResourceCheckContainer(resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})
	})

	Describe("FindResourceCheckContainerOnWorker", func() {
		var resourceConfig *dbng.UsedResourceConfig

		BeforeEach(func() {
			var err error
			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
				logger,
				dbng.ForResource(defaultResource.ID()),
				"some-base-resource-type",
				atc.Source{"some": "source"},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig, dbng.ContainerMetadata{Type: "check"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainerOnWorker(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).NotTo(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindResourceCheckContainerOnWorker(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			var originalCreatedContainer dbng.CreatedContainer

			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig, dbng.ContainerMetadata{Type: "check"})
				Expect(err).NotTo(HaveOccurred())
				originalCreatedContainer, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainerOnWorker(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).NotTo(BeNil())
				Expect(creatingContainer).To(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindResourceCheckContainerOnWorker(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})

			Context("when container is expired", func() {
				BeforeEach(func() {
					_, err := psql.Update("containers").
						Set("best_if_used_by", sq.Expr("NOW() - '1 second'::INTERVAL")).
						Where(sq.Eq{"id": originalCreatedContainer.ID()}).
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find it", func() {
					creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainerOnWorker(defaultWorker.Name(), resourceConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(creatingContainer).To(BeNil())
					Expect(createdContainer).To(BeNil())
				})
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindResourceCheckContainerOnWorker(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).To(BeNil())
			})
		})
	})

	Describe("FindWorkerForContainer", func() {
		var containerMetadata dbng.ContainerMetadata
		var defaultBuild dbng.Build

		BeforeEach(func() {
			var err error
			containerMetadata = dbng.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}
			defaultBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			var container dbng.CreatingContainer

			BeforeEach(func() {
				var err error
				container, err = defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer(container.Handle())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).NotTo(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForContainer(container.Handle())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			var container dbng.CreatedContainer

			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())

				container, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer(container.Handle())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).NotTo(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForContainer(container.Handle())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer("bogus-handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})
	})

	Describe("FindWorkerForBuildContainer", func() {
		var containerMetadata dbng.ContainerMetadata
		var defaultBuild dbng.Build

		BeforeEach(func() {
			containerMetadata = dbng.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}
			var err error
			defaultBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForBuildContainer(defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).NotTo(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForBuildContainer(defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				_, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForBuildContainer(defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).NotTo(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForBuildContainer(defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				worker, found, err := defaultTeam.FindWorkerForBuildContainer(defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})
	})

	Describe("FindBuildContainerOnWorker", func() {
		var containerMetadata dbng.ContainerMetadata
		var defaultBuild dbng.Build

		BeforeEach(func() {
			containerMetadata = dbng.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}
			var err error
			defaultBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindBuildContainerOnWorker(defaultWorker.Name(), defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).NotTo(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindBuildContainerOnWorker(defaultWorker.Name(), defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), defaultBuild.ID(), "some-plan", containerMetadata)
				Expect(err).NotTo(HaveOccurred())
				_, err = creatingContainer.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns it", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindBuildContainerOnWorker(defaultWorker.Name(), defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).NotTo(BeNil())
				Expect(creatingContainer).To(BeNil())
			})

			It("does not find container for another team", func() {
				creatingContainer, createdContainer, err := otherTeam.FindBuildContainerOnWorker(defaultWorker.Name(), defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingContainer).To(BeNil())
				Expect(createdContainer).To(BeNil())
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				creatingContainer, createdContainer, err := defaultTeam.FindBuildContainerOnWorker(defaultWorker.Name(), defaultBuild.ID(), "some-plan")
				Expect(err).NotTo(HaveOccurred())
				Expect(createdContainer).To(BeNil())
				Expect(creatingContainer).To(BeNil())
			})
		})
	})

	Describe("Updating Auth", func() {
		var (
			basicAuth    *atc.BasicAuth
			authProvider map[string]*json.RawMessage
		)

		BeforeEach(func() {
			basicAuth = &atc.BasicAuth{
				BasicAuthUsername: "fake user",
				BasicAuthPassword: "no, bad",
			}

			data := []byte(`{"credit_card":"please"}`)
			authProvider = map[string]*json.RawMessage{
				"fake-provider": (*json.RawMessage)(&data),
			}
		})

		Describe("UpdateBasicAuth", func() {
			It("saves basic auth team info without overwriting the provider auth", func() {
				err := team.UpdateProviderAuth(authProvider)
				Expect(err).NotTo(HaveOccurred())

				err = team.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(team.Auth()).To(Equal(authProvider))
			})

			It("saves basic auth team info to the existing team", func() {
				err := team.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(team.BasicAuth().BasicAuthUsername).To(Equal(basicAuth.BasicAuthUsername))
				Expect(bcrypt.CompareHashAndPassword([]byte(team.BasicAuth().BasicAuthPassword),
					[]byte(basicAuth.BasicAuthPassword))).To(BeNil())
			})

			It("nulls basic auth when has a blank username", func() {
				basicAuth.BasicAuthUsername = ""
				err := team.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(team.BasicAuth()).To(BeNil())
			})

			It("nulls basic auth when has a blank password", func() {
				basicAuth.BasicAuthPassword = ""
				err := team.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(team.BasicAuth()).To(BeNil())
			})
		})

		Describe("UpdateProviderAuth", func() {
			It("saves auth team info to the existing team", func() {
				err := team.UpdateProviderAuth(authProvider)
				Expect(err).NotTo(HaveOccurred())

				Expect(team.Auth()).To(Equal(authProvider))
			})

			It("saves github auth team info without over writing the basic auth", func() {
				err := team.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				err = team.UpdateProviderAuth(authProvider)
				Expect(err).NotTo(HaveOccurred())

				Expect(team.BasicAuth().BasicAuthUsername).To(Equal(basicAuth.BasicAuthUsername))
				Expect(bcrypt.CompareHashAndPassword([]byte(team.BasicAuth().BasicAuthPassword),
					[]byte(basicAuth.BasicAuthPassword))).To(BeNil())
			})
		})
	})

	Describe("Pipelines", func() {
		var (
			pipelines []dbng.Pipeline
			pipeline1 dbng.Pipeline
			pipeline2 dbng.Pipeline
		)

		JustBeforeEach(func() {
			var err error
			pipelines, err = team.Pipelines()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team has configured pipelines", func() {
			BeforeEach(func() {
				var err error
				pipeline1, _, err = team.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = team.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the pipelines", func() {
				Expect(pipelines).To(Equal([]dbng.Pipeline{pipeline1, pipeline2}))
			})
		})
		Context("when the team has no configured pipelines", func() {
			It("returns no pipelines", func() {
				Expect(pipelines).To(Equal([]dbng.Pipeline{}))
			})
		})
	})

	Describe("PublicPipelines", func() {
		var (
			pipelines []dbng.Pipeline
			pipeline1 dbng.Pipeline
			pipeline2 dbng.Pipeline
		)

		JustBeforeEach(func() {
			var err error
			pipelines, err = team.PublicPipelines()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team has configured pipelines", func() {
			BeforeEach(func() {
				var err error
				pipeline1, _, err = team.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = team.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				err = pipeline2.Expose()
				Expect(err).ToNot(HaveOccurred())

				found, err := pipeline2.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns the pipelines", func() {
				Expect(pipelines).To(Equal([]dbng.Pipeline{pipeline2}))
			})
		})
		Context("when the team has no configured pipelines", func() {
			It("returns no pipelines", func() {
				Expect(pipelines).To(Equal([]dbng.Pipeline{}))
			})
		})
	})

	Describe("VisiblePipelines", func() {
		var (
			pipelines []dbng.Pipeline
			pipeline1 dbng.Pipeline
			pipeline2 dbng.Pipeline
		)

		JustBeforeEach(func() {
			var err error
			pipelines, err = team.VisiblePipelines()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team has configured pipelines", func() {
			BeforeEach(func() {
				var err error
				pipeline1, _, err = team.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = otherTeam.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				Expect(pipeline2.Expose()).To(Succeed())
				Expect(pipeline2.Reload()).To(BeTrue())
			})

			It("returns the pipelines", func() {
				Expect(pipelines).To(Equal([]dbng.Pipeline{pipeline1, pipeline2}))
			})

			Context("when the other team has a private pipeline", func() {
				var pipeline3 dbng.Pipeline
				BeforeEach(func() {
					var err error
					pipeline3, _, err = otherTeam.SavePipeline("fake-pipeline-three", atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "job-fake-again"},
						},
					}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not return the other team private pipeline", func() {
					Expect(pipelines).To(Equal([]dbng.Pipeline{pipeline1, pipeline2}))
				})
			})
		})

		Context("when the team has no configured pipelines", func() {
			It("returns no pipelines", func() {
				Expect(pipelines).To(Equal([]dbng.Pipeline{}))
			})
		})
	})

	Describe("OrderPipelines", func() {
		var pipeline1 dbng.Pipeline
		var pipeline2 dbng.Pipeline
		var otherPipeline1 dbng.Pipeline
		var otherPipeline2 dbng.Pipeline

		BeforeEach(func() {
			var err error
			pipeline1, _, err = team.SavePipeline("pipeline-name-a", atc.Config{}, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			pipeline2, _, err = team.SavePipeline("pipeline-name-b", atc.Config{}, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline1, _, err = otherTeam.SavePipeline("pipeline-name-a", atc.Config{}, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			otherPipeline2, _, err = otherTeam.SavePipeline("pipeline-name-b", atc.Config{}, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		It("orders pipelines that belong to team (case insensitive)", func() {
			err := team.OrderPipelines([]string{"pipeline-name-b", "pipeline-name-a"})
			Expect(err).NotTo(HaveOccurred())

			err = otherTeam.OrderPipelines([]string{"pipeline-name-a", "pipeline-name-b"})
			Expect(err).NotTo(HaveOccurred())

			orderedPipelines, err := team.Pipelines()

			Expect(err).NotTo(HaveOccurred())
			Expect(orderedPipelines).To(HaveLen(2))
			Expect(orderedPipelines[0].ID()).To(Equal(pipeline2.ID()))
			Expect(orderedPipelines[1].ID()).To(Equal(pipeline1.ID()))

			otherTeamOrderedPipelines, err := otherTeam.Pipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(otherTeamOrderedPipelines).To(HaveLen(2))
			Expect(otherTeamOrderedPipelines[0].ID()).To(Equal(otherPipeline1.ID()))
			Expect(otherTeamOrderedPipelines[1].ID()).To(Equal(otherPipeline2.ID()))
		})
	})

	Describe("CreateOneOffBuild", func() {
		var (
			oneOffBuild dbng.Build
			err         error
		)

		BeforeEach(func() {
			oneOffBuild, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can create one-off builds", func() {
			Expect(oneOffBuild.ID()).NotTo(BeZero())
			Expect(oneOffBuild.JobName()).To(BeZero())
			Expect(oneOffBuild.PipelineName()).To(BeZero())
			Expect(oneOffBuild.Name()).To(Equal(strconv.Itoa(oneOffBuild.ID())))
			Expect(oneOffBuild.TeamName()).To(Equal(team.Name()))
			Expect(oneOffBuild.Status()).To(Equal(dbng.BuildStatusPending))
		})
	})

	Describe("GetPrivateAndPublicBuilds", func() {
		Context("when there are no builds", func() {
			It("returns an empty list of builds", func() {
				builds, pagination, err := team.GetPrivateAndPublicBuilds(dbng.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(pagination.Next).To(BeNil())
				Expect(pagination.Previous).To(BeNil())
				Expect(builds).To(BeEmpty())
			})
		})

		Context("when there are builds", func() {
			var allBuilds [5]dbng.Build
			var pipeline dbng.Pipeline
			var pipelineBuilds [2]dbng.Build

			BeforeEach(func() {
				for i := 0; i < 3; i++ {
					build, err := team.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())
					allBuilds[i] = build
				}

				config := atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
				}
				var err error
				pipeline, _, err = team.SavePipeline("some-pipeline", config, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				for i := 3; i < 5; i++ {
					build, err := pipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
					allBuilds[i] = build
					pipelineBuilds[i-3] = build
				}
			})

			It("returns all team builds with correct pagination", func() {
				builds, pagination, err := team.GetPrivateAndPublicBuilds(dbng.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[4]))
				Expect(builds[1]).To(Equal(allBuilds[3]))

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: allBuilds[3].ID(), Limit: 2}))

				builds, pagination, err = team.GetPrivateAndPublicBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))

				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&dbng.Page{Until: allBuilds[2].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: allBuilds[1].ID(), Limit: 2}))

				builds, pagination, err = team.GetPrivateAndPublicBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0]).To(Equal(allBuilds[0]))

				Expect(pagination.Previous).To(Equal(&dbng.Page{Until: allBuilds[0].ID(), Limit: 2}))
				Expect(pagination.Next).To(BeNil())

				builds, pagination, err = team.GetPrivateAndPublicBuilds(*pagination.Previous)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&dbng.Page{Until: allBuilds[2].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&dbng.Page{Since: allBuilds[1].ID(), Limit: 2}))
			})

			Context("when there are builds that belong to different teams", func() {
				var teamABuilds [3]dbng.Build
				var teamBBuilds [3]dbng.Build

				var caseInsensitiveTeamA dbng.Team
				var caseInsensitiveTeamB dbng.Team

				BeforeEach(func() {
					_, err := teamFactory.CreateTeam(atc.Team{Name: "team-a"})
					Expect(err).NotTo(HaveOccurred())

					_, err = teamFactory.CreateTeam(atc.Team{Name: "team-b"})
					Expect(err).NotTo(HaveOccurred())

					var found bool
					caseInsensitiveTeamA, found, err = teamFactory.FindTeam("team-A")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					caseInsensitiveTeamB, found, err = teamFactory.FindTeam("team-B")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					for i := 0; i < 3; i++ {
						teamABuilds[i], err = caseInsensitiveTeamA.CreateOneOffBuild()
						Expect(err).NotTo(HaveOccurred())

						teamBBuilds[i], err = caseInsensitiveTeamB.CreateOneOffBuild()
						Expect(err).NotTo(HaveOccurred())
					}
				})

				Context("when other team builds are private", func() {
					It("returns only builds for requested team", func() {
						builds, _, err := caseInsensitiveTeamA.GetPrivateAndPublicBuilds(dbng.Page{Limit: 10})
						Expect(err).NotTo(HaveOccurred())

						Expect(len(builds)).To(Equal(3))
						Expect(builds).To(ConsistOf(teamABuilds))

						builds, _, err = caseInsensitiveTeamB.GetPrivateAndPublicBuilds(dbng.Page{Limit: 10})
						Expect(err).NotTo(HaveOccurred())

						Expect(len(builds)).To(Equal(3))
						Expect(builds).To(ConsistOf(teamBBuilds))
					})
				})

				Context("when other team builds are public", func() {
					BeforeEach(func() {
						pipeline.Expose()
					})

					It("returns builds for requested team and public builds", func() {
						builds, _, err := caseInsensitiveTeamA.GetPrivateAndPublicBuilds(dbng.Page{Limit: 10})
						Expect(err).NotTo(HaveOccurred())

						Expect(builds).To(HaveLen(5))
						expectedBuilds := []dbng.Build{}
						for _, b := range teamABuilds {
							expectedBuilds = append(expectedBuilds, b)
						}
						for _, b := range pipelineBuilds {
							expectedBuilds = append(expectedBuilds, b)
						}
						Expect(builds).To(ConsistOf(expectedBuilds))
					})
				})
			})
		})
	})
})
