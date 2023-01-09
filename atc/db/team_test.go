package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team", func() {
	var (
		team      db.Team
		otherTeam db.Team
	)

	expectConfigsEqual := func(config, expectedConfig atc.Config) {
		ExpectWithOffset(1, config.Groups).To(ConsistOf(expectedConfig.Groups))
		ExpectWithOffset(1, config.Resources).To(ConsistOf(expectedConfig.Resources))
		ExpectWithOffset(1, config.ResourceTypes).To(ConsistOf(expectedConfig.ResourceTypes))
		ExpectWithOffset(1, config.Jobs).To(ConsistOf(expectedConfig.Jobs))
	}

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())
		otherTeam, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Delete", func() {
		var err error

		BeforeEach(func() {
			err = otherTeam.Delete()
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes the team", func() {
			team, found, err := teamFactory.FindTeam("some-other-team")
			Expect(team).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("drops the team_build_events_ID table", func() {
			var exists bool
			err := dbConn.QueryRow(fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'team_build_events_%d')", otherTeam.ID())).Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("Rename", func() {
		JustBeforeEach(func() {
			Expect(team.Rename("oopsies")).To(Succeed())
		})

		It("find the renamed team", func() {
			_, found, _ := teamFactory.FindTeam("oopsies")
			Expect(found).To(BeTrue())
		})
	})

	Describe("SaveWorker", func() {
		var (
			team      db.Team
			otherTeam db.Team
			atcWorker atc.Worker
			err       error
		)

		BeforeEach(func() {
			postgresRunner.Truncate()
			team, err = teamFactory.CreateTeam(atc.Team{Name: "team"})
			Expect(err).ToNot(HaveOccurred())

			otherTeam, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
			Expect(err).ToNot(HaveOccurred())
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
						Expect(err).ToNot(HaveOccurred())
					})
					It("overwrites all the data", func() {
						atcWorker.GardenAddr = "new-garden-addr"
						savedWorker, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).ToNot(HaveOccurred())
						Expect(savedWorker.Name()).To(Equal("some-name"))
						Expect(*savedWorker.GardenAddr()).To(Equal("new-garden-addr"))
						Expect(savedWorker.State()).To(Equal(db.WorkerStateRunning))
					})
				})
				Context("the team_id of the new worker is different", func() {
					BeforeEach(func() {
						_, err = otherTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).ToNot(HaveOccurred())
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
			team      db.Team
			otherTeam db.Team
			atcWorker atc.Worker
			err       error
		)

		BeforeEach(func() {
			postgresRunner.Truncate()
			team, err = teamFactory.CreateTeam(atc.Team{Name: "team"})
			Expect(err).ToNot(HaveOccurred())

			otherTeam, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
			Expect(err).ToNot(HaveOccurred())
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
				Expect(err).ToNot(HaveOccurred())

				atcWorker.Name = "some-new-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-bc-url"
				_, err = workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds them without error", func() {
				workers, err := team.Workers()
				Expect(err).ToNot(HaveOccurred())
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
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not find the other team workers", func() {
				workers, err := team.Workers()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(workers)).To(Equal(0))
			})
		})

		Context("when there are no workers", func() {
			It("returns an error", func() {
				workers, err := workerFactory.Workers()
				Expect(err).ToNot(HaveOccurred())
				Expect(workers).To(BeEmpty())
			})
		})
	})

	Describe("FindContainersByMetadata", func() {
		var sampleMetadata []db.ContainerMetadata
		var metaContainers map[db.ContainerMetadata][]db.Container

		BeforeEach(func() {
			baseMetadata := fullMetadata

			diffType := fullMetadata
			diffType.Type = db.ContainerTypeCheck

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

			sampleMetadata = []db.ContainerMetadata{
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

			job, found, err := defaultPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			Expect(build.CreatedBy()).ToNot(BeNil())
			Expect(*build.CreatedBy()).To(Equal(defaultBuildCreatedBy))

			metaContainers = make(map[db.ContainerMetadata][]db.Container)
			for _, meta := range sampleMetadata {
				firstContainerCreating, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job"), defaultTeam.ID()), meta)
				Expect(err).ToNot(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], firstContainerCreating)

				secondContainerCreating, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job"), defaultTeam.ID()), meta)
				Expect(err).ToNot(HaveOccurred())

				secondContainerCreated, err := secondContainerCreating.Created()
				Expect(err).ToNot(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], secondContainerCreated)

				thirdContainerCreating, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job"), defaultTeam.ID()), meta)
				Expect(err).ToNot(HaveOccurred())

				thirdContainerCreated, err := thirdContainerCreating.Created()
				Expect(err).ToNot(HaveOccurred())

				// third container is not appended; we don't want Destroying containers
				thirdContainerDestroying, err := thirdContainerCreated.Destroying()
				Expect(err).ToNot(HaveOccurred())

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
			containers, err := defaultTeam.FindContainersByMetadata(db.ContainerMetadata{
				Type: db.ContainerTypeTask,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).ToNot(BeEmpty())

			foundHandles := []string{}
			for _, c := range containers {
				foundHandles = append(foundHandles, c.Handle())
			}

			var notFound int
			for meta, cs := range metaContainers {
				if meta.Type == db.ContainerTypeTask {
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
			containers, err := defaultTeam.FindContainersByMetadata(db.ContainerMetadata{})
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

	Describe("Containers", func() {
		var (
			fakeSecretManager      *credsfakes.FakeSecrets
			resourceContainer      db.CreatingContainer
			resourceTypeContainer  db.CreatingContainer
			prototypeContainer     db.CreatingContainer
			firstContainerCreating db.CreatingContainer
			scenario               *dbtest.Scenario
		)

		resourceConfigCheckContainer := func(worker db.Worker, resourceConfigID int) db.CreatingContainer {
			rc, found, err := resourceConfigFactory.FindResourceConfigByID(resourceConfigID)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			c, err := worker.CreateContainer(
				db.NewResourceConfigCheckSessionContainerOwner(
					rc.ID(),
					rc.OriginBaseResourceType().ID,
					db.ContainerOwnerExpiries{
						Min: 5 * time.Minute,
						Max: 1 * time.Hour,
					},
				),
				db.ContainerMetadata{},
			)
			Expect(err).ToNot(HaveOccurred())

			return c
		}

		Context("when there are task and check containers", func() {
			BeforeEach(func() {
				fakeSecretManager = new(credsfakes.FakeSecrets)
				fakeSecretManager.GetReturns("", nil, false, nil)

				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some": "source",
								},
							},
						},
						ResourceTypes: atc.ResourceTypes{
							{
								Name: "some-type",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some-type": "source",
								},
							},
						},
						Prototypes: atc.Prototypes{
							{
								Name: "some-prototype",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some-prototype": "source",
								},
							},
						},
					}),
					builder.WithWorker(atc.Worker{
						ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
						Name:            "some-default-worker",
						GardenAddr:      "3.4.5.6:7777",
						BaggageclaimURL: "7.8.9.10:7878",
					}),
					builder.WithResourceVersions("some-resource"),
					builder.WithResourceTypeVersions("some-type"),
					builder.WithPrototypeVersions("some-prototype"),
				)

				build, err := scenario.Job("some-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				firstContainerCreating, err = scenario.Workers[0].CreateContainer(db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job"), scenario.Team.ID()), db.ContainerMetadata{Type: "task", StepName: "some-task"})
				Expect(err).ToNot(HaveOccurred())

				resourceContainer = resourceConfigCheckContainer(scenario.Workers[0], scenario.Resource("some-resource").ResourceConfigID())
				resourceTypeContainer = resourceConfigCheckContainer(scenario.Workers[0], scenario.ResourceType("some-type").ResourceConfigID())
				prototypeContainer = resourceConfigCheckContainer(scenario.Workers[0], scenario.Prototype("some-prototype").ResourceConfigID())
			})

			It("finds all the containers", func() {
				containers, err := scenario.Team.Containers()
				Expect(err).ToNot(HaveOccurred())

				Expect(containers).To(ConsistOf(firstContainerCreating, resourceContainer, resourceTypeContainer, prototypeContainer))
			})

			It("does not find containers for other teams", func() {
				containers, err := otherTeam.Containers()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(BeEmpty())
			})
		})

		Context("when there is a check container on a team worker", func() {
			var resourceContainer db.Container

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithTeam("some-test-team"),
					builder.WithPipeline(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some": "source",
								},
							},
						},
						ResourceTypes: atc.ResourceTypes{
							{
								Name: "some-type",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some-type": "source",
								},
							},
						},
					}),
					builder.WithWorker(atc.Worker{
						Name:            "default-team-worker",
						Team:            "some-test-team",
						ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
						GardenAddr:      "3.4.5.6:7777",
						BaggageclaimURL: "7.8.9.10:7878",
					}),
					builder.WithResourceVersions("some-resource"),
				)

				expiries := db.ContainerOwnerExpiries{
					Min: 5 * time.Minute,
					Max: 1 * time.Hour,
				}

				rc, found, err := resourceConfigFactory.FindResourceConfigByID(scenario.Resource("some-resource").ResourceConfigID())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceContainer, err = scenario.Workers[0].CreateContainer(
					db.NewResourceConfigCheckSessionContainerOwner(
						rc.ID(),
						rc.OriginBaseResourceType().ID,
						expiries,
					),
					db.ContainerMetadata{
						Type: "check",
					},
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds the container", func() {
				containers, err := scenario.Team.Containers()
				Expect(err).ToNot(HaveOccurred())

				Expect(containers).To(HaveLen(1))
				Expect(containers).To(ConsistOf(resourceContainer))
			})

			Context("when there is another check container with the same resource config on a different team worker", func() {
				var (
					resource2Container db.Container
					otherScenario      *dbtest.Scenario
				)

				BeforeEach(func() {
					otherScenario = dbtest.Setup(
						builder.WithTeam("other-team"),
						builder.WithPipeline(atc.Config{
							Jobs: atc.JobConfigs{
								{
									Name: "some-job",
								},
							},
							Resources: atc.ResourceConfigs{
								{
									Name: "some-resource",
									Type: "some-base-resource-type",
									Source: atc.Source{
										"some": "source",
									},
								},
							},
							ResourceTypes: atc.ResourceTypes{
								{
									Name: "some-type",
									Type: "some-base-resource-type",
									Source: atc.Source{
										"some-type": "source",
									},
								},
							},
						}),
						builder.WithWorker(atc.Worker{
							Name:            "other-team-worker",
							Team:            "other-team",
							ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
							GardenAddr:      "4.5.6.7:7777",
							BaggageclaimURL: "8.9.10.11:7878",
						}),
						builder.WithResourceVersions("some-resource"),
					)

					resource2Container = resourceConfigCheckContainer(otherScenario.Workers[0], otherScenario.Resource("some-resource").ResourceConfigID())
				})

				It("returns the container only from the team", func() {
					containers, err := otherScenario.Team.Containers()
					Expect(err).ToNot(HaveOccurred())

					Expect(containers).To(HaveLen(1))
					Expect(containers).To(ConsistOf(resource2Container))
				})
			})

			Context("when there is a check container with the same resource config on a global worker", func() {
				var (
					globalResourceContainer db.Container
				)

				BeforeEach(func() {
					globalResourceContainer = resourceConfigCheckContainer(scenario.Workers[0], scenario.Resource("some-resource").ResourceConfigID())
				})

				It("returns the container only from the team worker and global worker", func() {
					containers, err := scenario.Team.Containers()
					Expect(err).ToNot(HaveOccurred())

					Expect(containers).To(HaveLen(2))
					Expect(containers).To(ConsistOf(resourceContainer, globalResourceContainer))
				})
			})
		})
	})

	Describe("FindContainerByHandle", func() {
		var createdContainer db.CreatedContainer

		BeforeEach(func() {
			job, found, err := defaultPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job"), defaultTeam.ID()), db.ContainerMetadata{Type: "task", StepName: "some-task"})
			Expect(err).ToNot(HaveOccurred())

			createdContainer, err = creatingContainer.Created()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when worker is no longer in database", func() {
			BeforeEach(func() {
				err := defaultWorker.Delete()
				Expect(err).ToNot(HaveOccurred())
			})

			It("the container goes away from the db", func() {
				_, found, err := defaultTeam.FindContainerByHandle(createdContainer.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		It("finds a container for the team", func() {
			container, found, err := defaultTeam.FindContainerByHandle(createdContainer.Handle())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(container).ToNot(BeNil())
			Expect(container.Handle()).To(Equal(createdContainer.Handle()))
		})
	})

	Describe("FindVolumeForWorkerArtifact", func() {

		Context("when the artifact doesn't exist", func() {
			It("returns not found", func() {
				_, found, err := defaultTeam.FindVolumeForWorkerArtifact(12)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the artifact exists", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO worker_artifacts (id, name) VALUES ($1, '')", 18)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the associated volume doesn't exist", func() {
				It("returns not found", func() {
					_, found, err := defaultTeam.FindVolumeForWorkerArtifact(18)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			Context("when the associated volume exists", func() {
				BeforeEach(func() {
					_, err := dbConn.Exec("INSERT INTO volumes (handle, team_id, worker_name, worker_artifact_id, state) VALUES ('some-handle', $1, $2, $3, $4)", defaultTeam.ID(), defaultWorker.Name(), 18, db.VolumeStateCreated)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the volume", func() {
					volume, found, err := defaultTeam.FindVolumeForWorkerArtifact(18)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(volume.Handle()).To(Equal("some-handle"))
					Expect(volume.WorkerArtifactID()).To(Equal(18))
				})
			})
		})
	})

	Describe("FindWorkerForContainer", func() {
		var containerMetadata db.ContainerMetadata
		var defaultBuild db.Build

		BeforeEach(func() {
			var err error
			containerMetadata = db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}
			defaultBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			var container db.CreatingContainer

			BeforeEach(func() {
				var err error
				container, err = defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan", defaultTeam.ID()), containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer(container.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).ToNot(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})
		})

		Context("when there is a created container", func() {
			var container db.CreatedContainer

			BeforeEach(func() {
				creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan", defaultTeam.ID()), containerMetadata)
				Expect(err).ToNot(HaveOccurred())

				container, err = creatingContainer.Created()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer(container.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).ToNot(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer("bogus-handle")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})
	})

	Describe("FindWorkersForResourceCache", func() {
		var atcWorker = atc.Worker{
			GardenAddr:       "some-garden-addr",
			BaggageclaimURL:  "some-bc-url",
			HTTPProxyURL:     "some-http-proxy-url",
			HTTPSProxyURL:    "some-https-proxy-url",
			NoProxy:          "some-no-proxy",
			ActiveContainers: 140,
			ResourceTypes: []atc.WorkerResourceType{
				{
					Type:    "some-base-resource-type",
					Image:   "some-image",
					Version: "some-version",
				},
			},
			Platform:  "some-platform",
			Name:      "some-worker-name",
			StartTime: 55,
		}

		var urc db.ResourceCache
		var uwrc *db.UsedWorkerResourceCache
		var buildStartTime time.Time

		BeforeEach(func() {
			_, err := workerFactory.SaveWorker(atcWorker, 0)
			Expect(err).ToNot(HaveOccurred())

			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			buildStartTime = time.Now().Add(-100 * time.Second)

			urc, err = resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-base-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			creatingVolume, err := volumeRepository.CreateVolume(defaultTeam.ID(), atcWorker.Name, db.VolumeTypeResource)
			Expect(err).ToNot(HaveOccurred())

			createdVolume, err := creatingVolume.Created()
			Expect(err).ToNot(HaveOccurred())

			uwrc, err = createdVolume.InitializeResourceCache(urc)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the worker is running", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateRunning)
				_, err := workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should find the worker", func() {
				workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(workers)).To(Equal(1))
				Expect(workers[0].Name()).To(Equal("some-worker-name"))
			})
		})

		Context("when the worker is stalled", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateStalled)
				_, err := workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not find the worker", func() {
				workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(workers)).To(Equal(0))
			})
		})

		Context("when the worker is retiring", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateRetiring)
				_, err := workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not find the worker", func() {
				workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(workers)).To(Equal(0))
			})
		})

		Context("when the worker is landing", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateLanding)
				_, err := workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not find the worker", func() {
				workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(workers)).To(Equal(0))
			})
		})

		Context("when the worker is landed", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateLanded)
				_, err := workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not find the worker", func() {
				workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(workers)).To(Equal(0))
			})
		})

		Context("when there is a second worker", func() {
			var atcWorker2 = atc.Worker{
				GardenAddr:       "some-garden-addr-2",
				BaggageclaimURL:  "some-bc-url-2",
				HTTPProxyURL:     "some-http-proxy-url-2",
				HTTPSProxyURL:    "some-https-proxy-url-2",
				NoProxy:          "some-no-proxy-2",
				ActiveContainers: 140,
				ResourceTypes: []atc.WorkerResourceType{
					{
						Type:    "some-base-resource-type",
						Image:   "some-image",
						Version: "some-version",
					},
				},
				Platform:  "some-platform",
				Name:      "some-worker-name-2",
				StartTime: 55,
			}

			var volumeOnWorker2 db.CreatedVolume
			var uwrc2 *db.UsedWorkerResourceCache

			BeforeEach(func() {
				var err error
				_, err = workerFactory.SaveWorker(atcWorker2, 0)
				Expect(err).ToNot(HaveOccurred())

				creatingVolume, err := volumeRepository.CreateVolume(defaultTeam.ID(), atcWorker2.Name, db.VolumeTypeResource)
				Expect(err).ToNot(HaveOccurred())

				volumeOnWorker2, err = creatingVolume.Created()
				Expect(err).ToNot(HaveOccurred())

				uwrc2, err = volumeOnWorker2.InitializeStreamedResourceCache(urc, uwrc.ID)
				Expect(err).ToNot(HaveOccurred())
				Expect(uwrc2).ToNot(BeNil())
			})

			Context("when the workers are running", func() {
				BeforeEach(func() {
					atcWorker.State = string(db.WorkerStateRunning)
					_, err := workerFactory.SaveWorker(atcWorker, 0)
					Expect(err).ToNot(HaveOccurred())

					atcWorker2.State = string(db.WorkerStateRunning)
					_, err = workerFactory.SaveWorker(atcWorker2, 0)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should find both workers", func() {
					workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
					Expect(err).ToNot(HaveOccurred())
					Expect(len(workers)).To(Equal(2))
					Expect(workers[0].Name()).To(Equal("some-worker-name"))
					Expect(workers[1].Name()).To(Equal("some-worker-name-2"))
				})

				Context("when worker1 is pruned", func() {
					BeforeEach(func() {
						worker, found, err := workerFactory.GetWorker("some-worker-name")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())
						err = worker.Land()
						Expect(err).ToNot(HaveOccurred())
						err = worker.Prune()
						Expect(err).ToNot(HaveOccurred())
					})

					Context("when build started before worker1 is pruned", func() {
						BeforeEach(func() {
							buildStartTime = time.Now().Add(-100 * time.Second)
						})

						It("should still find worker2", func() {
							workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
							Expect(err).ToNot(HaveOccurred())
							Expect(len(workers)).To(Equal(1))
							Expect(workers[0].Name()).To(Equal("some-worker-name-2"))
						})

						It("cached volume should found on worker2", func() {
							volume, found, err := volumeRepository.FindResourceCacheVolume("some-worker-name-2", urc, buildStartTime)
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(volume.Handle()).To(Equal(volumeOnWorker2.Handle()))
						})

						It("the volume should be still there", func() {
							v, found, err := volumeRepository.FindVolume(volumeOnWorker2.Handle())
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(v.WorkerResourceCacheID()).To(Equal(uwrc2.ID))
						})
					})

					Context("when build started after worker1 is pruned", func() {
						BeforeEach(func() {
							buildStartTime = time.Now().Add(100 * time.Second)
						})

						It("should not find any worker", func() {
							workers, err := defaultTeam.FindWorkersForResourceCache(urc.ID(), buildStartTime)
							Expect(err).ToNot(HaveOccurred())
							Expect(len(workers)).To(Equal(0))
						})

						It("cached volume should not found on worker2", func() {
							_, found, err := volumeRepository.FindResourceCacheVolume("some-worker-name-2", urc, buildStartTime)
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeFalse())
						})

						It("but the volume should be still there", func() {
							v, found, err := volumeRepository.FindVolume(volumeOnWorker2.Handle())
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(v.WorkerResourceCacheID()).To(Equal(uwrc2.ID))
						})
					})
				})
			})
		})
	})

	Describe("Updating Auth", func() {
		var (
			authProvider atc.TeamAuth
		)

		BeforeEach(func() {
			authProvider = atc.TeamAuth{
				"owner": {"users": []string{"local:username"}},
			}
		})

		Describe("UpdateProviderAuth", func() {
			It("saves auth team info to the existing team", func() {
				err := team.UpdateProviderAuth(authProvider)
				Expect(err).ToNot(HaveOccurred())

				Expect(team.Auth()).To(Equal(authProvider))
			})

			It("resets legacy_auth to NULL", func() {
				oldLegacyAuth := `{"basicauth": {"username": "u", "password": "p"}}`
				_, err := dbConn.Exec("UPDATE teams SET legacy_auth = $1 WHERE id = $2", oldLegacyAuth, team.ID())
				Expect(err).ToNot(HaveOccurred())
				team.UpdateProviderAuth(authProvider)

				var newLegacyAuth sql.NullString
				err = dbConn.QueryRow("SELECT legacy_auth FROM teams WHERE id = $1", team.ID()).Scan(&newLegacyAuth)
				Expect(err).ToNot(HaveOccurred())

				value, err := newLegacyAuth.Value()
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(BeNil())
			})

			Context("when team auth is already set", func() {
				BeforeEach(func() {
					team.UpdateProviderAuth(atc.TeamAuth{
						"owner":  {"users": []string{"local:somebody"}},
						"viewer": {"users": []string{"local:someone"}},
					})
				})

				It("overrides the existing auth with the new config", func() {
					err := team.UpdateProviderAuth(authProvider)
					Expect(err).ToNot(HaveOccurred())

					Expect(team.Auth()).To(Equal(authProvider))
				})
			})
		})
	})

	Describe("Pipelines", func() {
		var (
			pipelines []db.Pipeline
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
			pipeline3 db.Pipeline
		)

		JustBeforeEach(func() {
			var err error
			pipelines, err = team.Pipelines()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team has configured pipelines", func() {
			BeforeEach(func() {
				var err error
				pipeline1, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline", InstanceVars: atc.InstanceVars{"branch": "feature/foo"}}, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				pipeline3, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-two"}, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the pipelines", func() {
				Expect(pipelines[0].Name()).To(Equal(pipeline1.Name()))
				Expect(pipelines[0].InstanceVars()).To(Equal(pipeline1.InstanceVars()))

				Expect(pipelines[1].Name()).To(Equal(pipeline2.Name()))
				Expect(pipelines[1].InstanceVars()).To(Equal(pipeline2.InstanceVars()))

				Expect(pipelines[2].Name()).To(Equal(pipeline3.Name()))
				Expect(pipelines[2].InstanceVars()).To(BeNil())
			})

			Context("when a pipeline with the same name as an existing pipeline is created", func() {
				var pipeline4 db.Pipeline

				BeforeEach(func() {
					var err error
					pipeline4, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline", InstanceVars: atc.InstanceVars{"branch": "other"}}, atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "job-name"},
						},
					}, db.ConfigVersion(1), false)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the pipelines with the same name grouped together, ordered by creation time", func() {
					Expect(pipelines[0].Name()).To(Equal(pipeline1.Name()))
					Expect(pipelines[0].InstanceVars()).To(Equal(pipeline1.InstanceVars()))

					Expect(pipelines[1].Name()).To(Equal(pipeline2.Name()))
					Expect(pipelines[1].InstanceVars()).To(Equal(pipeline2.InstanceVars()))

					Expect(pipelines[2].Name()).To(Equal(pipeline4.Name()))
					Expect(pipelines[2].InstanceVars()).To(Equal(pipeline4.InstanceVars()))

					Expect(pipelines[3].Name()).To(Equal(pipeline3.Name()))
					Expect(pipelines[3].InstanceVars()).To(BeNil())
				})
			})
		})
		Context("when the team has no configured pipelines", func() {
			It("returns no pipelines", func() {
				Expect(pipelines).To(Equal([]db.Pipeline{}))
			})
		})
	})

	Describe("PublicPipelines", func() {
		var (
			pipelines []db.Pipeline
			pipeline2 db.Pipeline
		)

		JustBeforeEach(func() {
			var err error
			pipelines, err = team.PublicPipelines()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team has configured pipelines", func() {
			BeforeEach(func() {
				var err error
				_, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline"}, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline-two"}, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				err = pipeline2.Expose()
				Expect(err).ToNot(HaveOccurred())

				found, err := pipeline2.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns the pipelines", func() {
				Expect(pipelines).To(Equal([]db.Pipeline{pipeline2}))
			})
		})
		Context("when the team has no configured pipelines", func() {
			It("returns no pipelines", func() {
				Expect(pipelines).To(Equal([]db.Pipeline{}))
			})
		})
	})

	Describe("OrderPipelines", func() {
		var (
			instancePipeline1 db.Pipeline
			instancePipeline2 db.Pipeline
			pipeline1         db.Pipeline
			pipeline2         db.Pipeline

			otherTeamPipeline1 db.Pipeline
			otherTeamPipeline2 db.Pipeline
		)

		BeforeEach(func() {
			var err error
			instancePipeline1, _, err = team.SavePipeline(atc.PipelineRef{Name: "group", InstanceVars: atc.InstanceVars{"branch": "master"}}, atc.Config{}, 0, false)
			Expect(err).ToNot(HaveOccurred())
			instancePipeline2, _, err = team.SavePipeline(atc.PipelineRef{Name: "group", InstanceVars: atc.InstanceVars{"branch": "feature/foo"}}, atc.Config{}, 0, false)
			Expect(err).ToNot(HaveOccurred())

			pipeline1, _, err = team.SavePipeline(atc.PipelineRef{Name: "pipeline1"}, atc.Config{}, 0, false)
			Expect(err).ToNot(HaveOccurred())
			pipeline2, _, err = team.SavePipeline(atc.PipelineRef{Name: "pipeline2"}, atc.Config{}, 0, false)
			Expect(err).ToNot(HaveOccurred())

			otherTeamPipeline1, _, err = otherTeam.SavePipeline(atc.PipelineRef{Name: "pipeline1"}, atc.Config{}, 0, false)
			Expect(err).ToNot(HaveOccurred())
			otherTeamPipeline2, _, err = otherTeam.SavePipeline(atc.PipelineRef{Name: "pipeline2"}, atc.Config{}, 0, false)
			Expect(err).ToNot(HaveOccurred())
		})

		It("orders pipelines that belong to team", func() {
			err := team.OrderPipelines([]string{"pipeline2", "group", "pipeline1"})
			Expect(err).ToNot(HaveOccurred())

			err = otherTeam.OrderPipelines([]string{"pipeline2", "pipeline1"})
			Expect(err).ToNot(HaveOccurred())

			orderedPipelines, err := team.Pipelines()
			Expect(err).ToNot(HaveOccurred())
			Expect(orderedPipelines).To(HaveLen(4))
			Expect(orderedPipelines[0].ID()).To(Equal(pipeline2.ID()))
			Expect(orderedPipelines[1].ID()).To(Equal(instancePipeline1.ID()))
			Expect(orderedPipelines[2].ID()).To(Equal(instancePipeline2.ID()))
			Expect(orderedPipelines[3].ID()).To(Equal(pipeline1.ID()))

			otherTeamOrderedPipelines, err := otherTeam.Pipelines()
			Expect(err).ToNot(HaveOccurred())
			Expect(otherTeamOrderedPipelines).To(HaveLen(2))
			Expect(otherTeamOrderedPipelines[0].ID()).To(Equal(otherTeamPipeline2.ID()))
			Expect(otherTeamOrderedPipelines[1].ID()).To(Equal(otherTeamPipeline1.ID()))
		})

		Context("when pipeline does not exist", func() {
			It("returns not found error", func() {
				err := otherTeam.OrderPipelines([]string{"pipeline1", "invalid-pipeline"})
				Expect(err).To(MatchError(db.ErrPipelineNotFound{Name: "invalid-pipeline"}))
			})
		})
	})

	Describe("CreateOneOffBuild", func() {
		var (
			oneOffBuild db.Build
			err         error
		)

		BeforeEach(func() {
			oneOffBuild, err = team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())
		})

		It("can create one-off builds", func() {
			Expect(oneOffBuild.ID()).ToNot(BeZero())
			Expect(oneOffBuild.JobName()).To(BeZero())
			Expect(oneOffBuild.PipelineName()).To(BeZero())
			Expect(oneOffBuild.Name()).To(Equal(strconv.Itoa(oneOffBuild.ID())))
			Expect(oneOffBuild.TeamName()).To(Equal(team.Name()))
			Expect(oneOffBuild.Status()).To(Equal(db.BuildStatusPending))
		})
	})

	Describe("CreateStartedBuild", func() {
		var (
			plan         atc.Plan
			startedBuild db.Build
			err          error
		)

		BeforeEach(func() {
			imageCheckPlanID := atc.PlanID("image-check")

			plan = atc.Plan{
				ID: atc.PlanID("56"),
				Get: &atc.GetPlan{
					Type:     "some-type",
					Name:     "some-name",
					Resource: "some-resource",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Version:  &atc.Version{"some": "version"},
					Tags:     atc.Tags{"some-tags"},
					TypeImage: atc.TypeImage{
						BaseType: "some-type",
						CheckPlan: &atc.Plan{
							ID: imageCheckPlanID,
							Check: &atc.CheckPlan{
								Name:   "some-name",
								Source: atc.Source{"some": "source"},
								Type:   "some-type",
								TypeImage: atc.TypeImage{
									BaseType:   "some-type",
									Privileged: true,
								},
								Tags: atc.Tags{"some-tags"},
							},
						},
						GetPlan: &atc.Plan{
							ID: "image-get",
							Get: &atc.GetPlan{
								Name:   "some-name",
								Source: atc.Source{"some": "source"},
								Type:   "some-type",
								TypeImage: atc.TypeImage{
									BaseType:   "some-type",
									Privileged: true,
								},
								Tags:        atc.Tags{"some-tags"},
								VersionFrom: &imageCheckPlanID,
							},
						},
					},
				},
			}

			startedBuild, err = team.CreateStartedBuild(plan)
			Expect(err).ToNot(HaveOccurred())
		})

		It("can create started builds with plans", func() {
			Expect(startedBuild.ID()).ToNot(BeZero())
			Expect(startedBuild.JobName()).To(BeZero())
			Expect(startedBuild.PipelineName()).To(BeZero())
			Expect(startedBuild.Name()).To(Equal(strconv.Itoa(startedBuild.ID())))
			Expect(startedBuild.TeamName()).To(Equal(team.Name()))
			Expect(startedBuild.Status()).To(Equal(db.BuildStatusStarted))
		})

		It("saves the public plan", func() {
			found, err := startedBuild.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(startedBuild.PublicPlan()).To(Equal(plan.Public()))
		})

		It("creates Start event", func() {
			found, err := startedBuild.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			events, err := startedBuild.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   startedBuild.StartTime().Unix(),
			}, "0")))
		})
	})

	Describe("PrivateAndPublicBuilds", func() {
		Context("when there are no builds", func() {
			It("returns an empty list of builds", func() {
				builds, pagination, err := team.PrivateAndPublicBuilds(db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Older).To(BeNil())
				Expect(pagination.Newer).To(BeNil())
				Expect(builds).To(BeEmpty())
			})
		})

		Context("when there are builds", func() {
			var allBuilds [5]db.Build
			var pipeline db.Pipeline
			var pipelineBuilds [2]db.Build

			BeforeEach(func() {
				for i := 0; i < 3; i++ {
					build, err := team.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())
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
				pipeline, _, err = team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, config, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				job, found, err := pipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				for i := 3; i < 5; i++ {
					build, err := job.CreateBuild(defaultBuildCreatedBy)
					Expect(err).ToNot(HaveOccurred())
					allBuilds[i] = build
					pipelineBuilds[i-3] = build
				}
			})

			It("returns all team builds with correct pagination", func() {
				builds, pagination, err := team.PrivateAndPublicBuilds(db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[4]))
				Expect(builds[1]).To(Equal(allBuilds[3]))

				Expect(pagination.Newer).To(BeNil())
				Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(allBuilds[2].ID()), Limit: 2}))

				builds, pagination, err = team.PrivateAndPublicBuilds(*pagination.Older)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(2))

				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(allBuilds[3].ID()), Limit: 2}))
				Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(allBuilds[0].ID()), Limit: 2}))

				builds, pagination, err = team.PrivateAndPublicBuilds(*pagination.Older)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0]).To(Equal(allBuilds[0]))

				Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(allBuilds[1].ID()), Limit: 2}))
				Expect(pagination.Older).To(BeNil())

				builds, pagination, err = team.PrivateAndPublicBuilds(*pagination.Newer)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))
				Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(allBuilds[3].ID()), Limit: 2}))
				Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(allBuilds[0].ID()), Limit: 2}))
			})

			Context("when there are builds that belong to different teams", func() {
				var teamABuilds [3]db.Build
				var teamBBuilds [3]db.Build

				var caseInsensitiveTeamA db.Team
				var caseInsensitiveTeamB db.Team

				BeforeEach(func() {
					_, err := teamFactory.CreateTeam(atc.Team{Name: "team-a"})
					Expect(err).ToNot(HaveOccurred())

					_, err = teamFactory.CreateTeam(atc.Team{Name: "team-b"})
					Expect(err).ToNot(HaveOccurred())

					var found bool
					caseInsensitiveTeamA, found, err = teamFactory.FindTeam("team-A")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					caseInsensitiveTeamB, found, err = teamFactory.FindTeam("team-B")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					for i := 0; i < 3; i++ {
						teamABuilds[i], err = caseInsensitiveTeamA.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())

						teamBBuilds[i], err = caseInsensitiveTeamB.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())
					}
				})

				Context("when other team builds are private", func() {
					It("returns only builds for requested team", func() {
						builds, _, err := caseInsensitiveTeamA.PrivateAndPublicBuilds(db.Page{Limit: 10})
						Expect(err).ToNot(HaveOccurred())

						Expect(len(builds)).To(Equal(3))
						Expect(builds).To(ConsistOf(teamABuilds))

						builds, _, err = caseInsensitiveTeamB.PrivateAndPublicBuilds(db.Page{Limit: 10})
						Expect(err).ToNot(HaveOccurred())

						Expect(len(builds)).To(Equal(3))
						Expect(builds).To(ConsistOf(teamBBuilds))
					})
				})

				Context("when other team builds are public", func() {
					BeforeEach(func() {
						err := pipeline.Expose()
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns builds for requested team and public builds", func() {
						builds, _, err := caseInsensitiveTeamA.PrivateAndPublicBuilds(db.Page{Limit: 10})
						Expect(err).ToNot(HaveOccurred())

						Expect(builds).To(HaveLen(5))
						expectedBuilds := []db.Build{}
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

	Describe("BuildsWithTime", func() {
		var (
			pipeline db.Pipeline
			builds   = make([]db.Build, 4)
		)

		BeforeEach(func() {
			var (
				err   error
				found bool
			)

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
			}
			pipeline, _, err = team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, config, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			for i := range builds {
				builds[i], err = job.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				buildStart := time.Date(2020, 11, i+1, 0, 0, 0, 0, time.UTC)
				_, err = dbConn.Exec("UPDATE builds SET start_time = to_timestamp($1) WHERE id = $2", buildStart.Unix(), builds[i].ID())
				Expect(err).NotTo(HaveOccurred())

				builds[i], found, err = job.Build(builds[i].Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			}
		})

		Context("When not providing boundaries", func() {
			Context("without a limit specified", func() {
				It("returns no builds", func() {
					returnedBuilds, _, err := team.BuildsWithTime(db.Page{})
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedBuilds).To(BeEmpty())
				})
			})

			Context("when a limit specified", func() {
				It("returns a subset of the builds", func() {
					returnedBuilds, _, err := team.BuildsWithTime(db.Page{
						Limit: 2,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[3], builds[2]))
				})
			})

		})

		Context("When providing boundaries", func() {
			Context("only to", func() {
				It("returns only those before to", func() {
					returnedBuilds, _, err := team.BuildsWithTime(db.Page{
						To:    db.NewIntPtr(int(builds[2].StartTime().Unix())),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[0], builds[1], builds[2]))
				})
			})

			Context("only from", func() {
				It("returns only those after from", func() {
					returnedBuilds, _, err := team.BuildsWithTime(db.Page{
						From:  db.NewIntPtr(int(builds[1].StartTime().Unix())),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2], builds[3]))
				})
			})

			Context("from and to", func() {
				It("returns only elements in the range", func() {
					returnedBuilds, _, err := team.BuildsWithTime(db.Page{
						From:  db.NewIntPtr(int(builds[1].StartTime().Unix())),
						To:    db.NewIntPtr(int(builds[2].StartTime().Unix())),
						Limit: 50,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2]))
				})
			})
		})
	})

	Describe("Builds", func() {
		var (
			expectedBuilds                              []db.Build
			pipeline                                    db.Pipeline
			oneOffBuild, build, secondBuild, thirdBuild db.Build
		)

		BeforeEach(func() {
			var err error

			oneOffBuild, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, oneOffBuild)

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
			}
			pipeline, _, err = team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, config, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err = job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			thirdBuild, err = someOtherJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, thirdBuild)
		})

		It("returns builds for the current team", func() {
			builds, _, err := team.Builds(db.Page{Limit: 10})
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		Context("when limiting the range of build ids", func() {
			Context("specifying only from", func() {
				It("returns all builds after and including the specified id", func() {
					builds, _, err := team.Builds(db.Page{Limit: 50, From: db.NewIntPtr(secondBuild.ID())})
					Expect(err).NotTo(HaveOccurred())
					Expect(builds).To(ConsistOf(secondBuild, thirdBuild))
				})
			})

			Context("specifying only to", func() {
				It("returns all builds before and including the specified id", func() {
					builds, _, err := team.Builds(db.Page{Limit: 50, To: db.NewIntPtr(secondBuild.ID())})
					Expect(err).NotTo(HaveOccurred())
					Expect(builds).To(ConsistOf(oneOffBuild, build, secondBuild))
				})
			})

			Context("specifying both from and to", func() {
				It("returns all builds within range of ids", func() {
					builds, _, err := team.Builds(db.Page{Limit: 50, From: db.NewIntPtr(build.ID()), To: db.NewIntPtr(thirdBuild.ID())})
					Expect(err).NotTo(HaveOccurred())
					Expect(builds).To(ConsistOf(build, secondBuild, thirdBuild))
				})
			})

			Context("specifying from greater than the biggest ID in the database", func() {
				It("returns no rows error", func() {
					builds, _, err := team.Builds(db.Page{Limit: 50, From: db.NewIntPtr(thirdBuild.ID() + 1)})
					Expect(err).ToNot(HaveOccurred())
					Expect(builds).To(BeEmpty())
				})
			})

			Context("specifying invalid boundaries", func() {
				It("should fail", func() {
					_, _, err := team.Builds(db.Page{Limit: 50, From: db.NewIntPtr(thirdBuild.ID()), To: db.NewIntPtr(secondBuild.ID())})
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when there are builds that belong to different teams", func() {
			var teamABuilds [3]db.Build
			var teamBBuilds [3]db.Build

			var caseInsensitiveTeamA db.Team
			var caseInsensitiveTeamB db.Team

			BeforeEach(func() {
				_, err := teamFactory.CreateTeam(atc.Team{Name: "team-a"})
				Expect(err).ToNot(HaveOccurred())

				_, err = teamFactory.CreateTeam(atc.Team{Name: "team-b"})
				Expect(err).ToNot(HaveOccurred())

				var found bool
				caseInsensitiveTeamA, found, err = teamFactory.FindTeam("team-A")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				caseInsensitiveTeamB, found, err = teamFactory.FindTeam("team-B")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				for i := 0; i < 3; i++ {
					teamABuilds[i], err = caseInsensitiveTeamA.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())

					teamBBuilds[i], err = caseInsensitiveTeamB.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())
				}
			})

			Context("when other team builds are private", func() {
				It("returns only builds for requested team", func() {
					builds, _, err := caseInsensitiveTeamA.Builds(db.Page{Limit: 10})
					Expect(err).ToNot(HaveOccurred())

					Expect(len(builds)).To(Equal(3))
					Expect(builds).To(ConsistOf(teamABuilds))

					builds, _, err = caseInsensitiveTeamB.Builds(db.Page{Limit: 10})
					Expect(err).ToNot(HaveOccurred())

					Expect(len(builds)).To(Equal(3))
					Expect(builds).To(ConsistOf(teamBBuilds))
				})
			})

			Context("when other team builds are public", func() {
				BeforeEach(func() {
					err := pipeline.Expose()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns only builds for requested team", func() {
					builds, _, err := caseInsensitiveTeamA.Builds(db.Page{Limit: 10})
					Expect(err).ToNot(HaveOccurred())

					Expect(len(builds)).To(Equal(3))
					Expect(builds).To(ConsistOf(teamABuilds))

					builds, _, err = caseInsensitiveTeamB.Builds(db.Page{Limit: 10})
					Expect(err).ToNot(HaveOccurred())

					Expect(len(builds)).To(Equal(3))
					Expect(builds).To(ConsistOf(teamBBuilds))
				})
			})
		})
	})

	Describe("Pipeline", func() {
		Context("when the team has instanced pipelines configured", func() {
			var (
				instancedPipelineRef atc.PipelineRef
				instancedPipeline    db.Pipeline
			)

			BeforeEach(func() {
				var err error
				instancedPipelineRef = atc.PipelineRef{
					Name:         "fake-pipeline",
					InstanceVars: atc.InstanceVars{"branch": "feature"},
				}
				instancedPipeline, _, err = team.SavePipeline(instancedPipelineRef, atc.Config{}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the pipeline when requesting by name and instance vars", func() {
				pipeline, found, err := team.Pipeline(instancedPipelineRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline).To(Equal(instancedPipeline))
			})

			It("returns nothing when requesting by name only", func() {
				pipeline, found, err := team.Pipeline(atc.PipelineRef{Name: "fake-pipeline"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(pipeline).To(BeNil())
			})

			Context("when the team has instanced and named pipelines configured with the same name", func() {
				var (
					namedPipelineRef atc.PipelineRef
					namedPipeline    db.Pipeline
				)

				BeforeEach(func() {
					var err error
					namedPipelineRef = atc.PipelineRef{Name: "fake-pipeline"}
					namedPipeline, _, err = team.SavePipeline(namedPipelineRef, atc.Config{}, db.ConfigVersion(0), false)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the named pipeline when requesting with name only", func() {
					pipeline, found, err := team.Pipeline(namedPipelineRef)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(pipeline).To(Equal(namedPipeline))
				})
			})
		})
	})

	Describe("SavePipeline", func() {
		type SerialGroup struct {
			JobID int
			Name  string
		}

		var (
			config       atc.Config
			otherConfig  atc.Config
			pipelineRef  atc.PipelineRef
			pipelineName string
		)

		BeforeEach(func() {
			config = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"resource-1", "resource-2"},
					},
				},

				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
						Icon: "some-icon",
					},
				},

				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-resource-type",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Prototypes: atc.Prototypes{
					{
						Name: "some-prototype",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-job",

						Public: true,

						Serial:       true,
						SerialGroups: []string{"serial-group-1", "serial-group-2"},

						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
									Passed:  []string{"job-1", "job-2"},
									Trigger: true,
								},
							},
							{
								Config: &atc.TaskStep{
									Name:       "some-task",
									Privileged: true,
									ConfigPath: "some/config/path.yml",
									Config: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
								},
							},
							{
								Config: &atc.PutStep{
									Name: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
								},
							},
						},
					},
					{
						Name: "job-1",
					},
					{
						Name: "job-2",
					},
				},
			}

			otherConfig = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"some-other-job", "job-1", "job-2"},
						Resources: []string{"resource-1", "resource-2"},
					},
				},

				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-other-job",
					},
				},
			}

			pipelineName = "some-pipeline"
			pipelineRef = atc.PipelineRef{Name: pipelineName}
		})

		It("returns true for created", func() {
			_, created, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		It("caches the team id", func() {
			_, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.TeamID()).To(Equal(team.ID()))
		})

		It("can be saved as paused", func() {
			_, _, err := team.SavePipeline(pipelineRef, config, 0, true)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused()).To(BeTrue())
		})

		It("can be saved as unpaused", func() {
			_, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused()).To(BeFalse())
		})

		It("is not archived by default", func() {
			_, _, err := team.SavePipeline(pipelineRef, config, 0, true)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Archived()).To(BeFalse())
		})

		It("requests schedule on the pipeline", func() {
			requestedPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline, _, err := team.SavePipeline(atc.PipelineRef{Name: "other-pipeline"}, otherConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())

			requestedJob, found, err := requestedPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherJob, found, err := otherPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			requestedSchedule1 := requestedJob.ScheduleRequestedTime()
			requestedSchedule2 := otherJob.ScheduleRequestedTime()

			config.Resources[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			_, _, err = team.SavePipeline(pipelineRef, config, requestedPipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			found, err = requestedJob.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			found, err = otherJob.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(requestedJob.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
			Expect(otherJob.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
		})

		It("creates all of the resources from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := savedPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{
				"source-config": "some-value",
			}))
		})

		It("updates resource config source and resets config_id and config_scope_id", func() {
			scenario := dbtest.Setup(
				builder.WithPipeline(config),
				builder.WithBaseResourceType(dbConn, "some-type"),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
				),
			)
			Expect(scenario.Resource("some-resource").Source()).To(Equal(atc.Source{
				"source-config": "some-value",
			}))
			Expect(scenario.Resource("some-resource").ResourceConfigID()).ToNot(BeZero())
			Expect(scenario.Resource("some-resource").ResourceConfigScopeID()).ToNot(BeZero())

			config.Resources[0].Source = atc.Source{"new-source-config": "some-other-value"}
			scenario.Run(
				builder.WithPipeline(config),
			)

			Expect(scenario.Resource("some-resource").Source()).To(Equal(atc.Source{
				"new-source-config": "some-other-value",
			}))
			Expect(scenario.Resource("some-resource").ResourceConfigID()).To(BeZero())
			Expect(scenario.Resource("some-resource").ResourceConfigScopeID()).To(BeZero())
		})

		It("updates resource config type and resets config_id and config_scope_id", func() {
			scenario := dbtest.Setup(
				builder.WithPipeline(config),
				builder.WithBaseResourceType(dbConn, "some-type"),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
				),
			)
			Expect(scenario.Resource("some-resource").Type()).To(Equal("some-type"))
			Expect(scenario.Resource("some-resource").ResourceConfigID()).ToNot(BeZero())
			Expect(scenario.Resource("some-resource").ResourceConfigScopeID()).ToNot(BeZero())

			config.Resources[0].Type = "some-other-type"
			scenario.Run(
				builder.WithPipeline(config),
			)

			Expect(scenario.Resource("some-resource").Type()).To(Equal("some-other-type"))
			Expect(scenario.Resource("some-resource").ResourceConfigID()).To(BeZero())
			Expect(scenario.Resource("some-resource").ResourceConfigScopeID()).To(BeZero())
		})

		It("updates other resource fields and does not reset config_id and config_scope_id", func() {
			scenario := dbtest.Setup(
				builder.WithPipeline(config),
				builder.WithBaseResourceType(dbConn, "some-type"),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
				),
			)
			Expect(scenario.Resource("some-resource").Icon()).To(Equal("some-icon"))
			configID, configScopeID := scenario.Resource("some-resource").ResourceConfigID(), scenario.Resource("some-resource").ResourceConfigScopeID()
			Expect(configID).ToNot(BeZero())
			Expect(configScopeID).ToNot(BeZero())

			config.Resources[0].Icon = "some-other-icon"
			scenario.Run(
				builder.WithPipeline(config),
			)

			Expect(scenario.Resource("some-resource").Icon()).To(Equal("some-other-icon"))
			Expect(scenario.Resource("some-resource").ResourceConfigID()).To(Equal(configID))
			Expect(scenario.Resource("some-resource").ResourceConfigScopeID()).To(Equal(configScopeID))
		})

		It("clears out api pinned version when resaving a pinned version on the pipeline config", func() {
			scenario := dbtest.Setup(
				builder.WithPipeline(config),
				builder.WithBaseResourceType(dbConn, "some-type"),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
				),
				builder.WithPinnedVersion("some-resource", atc.Version{"version": "v1"}),
			)

			Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))

			config.Resources[0].Version = atc.Version{
				"version": "v2",
			}

			scenario.Run(
				builder.WithPipeline(config),
			)

			Expect(scenario.Resource("some-resource").ConfigPinnedVersion()).To(Equal(atc.Version{"version": "v2"}))
			Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(BeNil())
		})

		It("clears out config pinned version when it is removed", func() {
			config.Resources[0].Version = atc.Version{
				"version": "v1",
			}

			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.ConfigPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
			Expect(resource.APIPinnedVersion()).To(BeNil())

			config.Resources[0].Version = nil

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			resource, found, err = savedPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.ConfigPinnedVersion()).To(BeNil())
			Expect(resource.APIPinnedVersion()).To(BeNil())
		})

		It("does not clear the api pinned version when resaving pipeline config", func() {
			scenario := dbtest.Setup(
				builder.WithPipeline(config),
				builder.WithBaseResourceType(dbConn, "some-type"),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
				),
				builder.WithPinnedVersion("some-resource", atc.Version{"version": "v1"}),
			)

			Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))

			scenario.Run(
				builder.WithPipeline(config),
			)

			Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
		})

		It("marks resource as inactive if it is no longer in config", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.Resources = []atc.ResourceConfig{}
			config.Jobs = atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					Serial:       true,
					SerialGroups: []string{"serial-group-1", "serial-group-2"},

					PlanSequence: []atc.Step{
						{
							Config: &atc.TaskStep{
								Name:       "some-task",
								Privileged: true,
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									RootfsURI: "some-image",
								},
							},
						},
						{
							Config: &atc.PutStep{
								Name: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
						},
					},
				},
				{
					Name: "job-1",
				},
				{
					Name: "job-2",
				},
			}
			config.Resources = atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
				},
			}

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the resource types from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			resourceType, found, err := savedPipeline.ResourceType("some-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resourceType.Type()).To(Equal("some-type"))
			Expect(resourceType.Source()).To(Equal(atc.Source{
				"source-config": "some-value",
			}))
		})

		It("updates resource type config from the pipeline in the database", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.ResourceTypes[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			resourceType, found, err := savedPipeline.ResourceType("some-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resourceType.Type()).To(Equal("some-type"))
			Expect(resourceType.Source()).To(Equal(atc.Source{
				"source-other-config": "some-other-value",
			}))
		})

		It("marks resource type as inactive if it is no longer in config", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.ResourceTypes = []atc.ResourceType{}

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.ResourceType("some-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the prototypes from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			prototype, found, err := savedPipeline.Prototype("some-prototype")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(prototype.Type()).To(Equal("some-type"))
			Expect(prototype.Source()).To(Equal(atc.Source{
				"source-config": "some-value",
			}))
		})

		It("updates prototype config from the pipeline in the database", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.Prototypes[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			prototype, found, err := savedPipeline.Prototype("some-prototype")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(prototype.Type()).To(Equal("some-type"))
			Expect(prototype.Source()).To(Equal(atc.Source{
				"source-other-config": "some-other-value",
			}))
		})

		It("marks prototype as inactive if it is no longer in config", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.Prototypes = atc.Prototypes{}

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.Prototype("some-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the jobs from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := savedPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.Config()).To(Equal(config.Jobs[0]))
		})

		It("updates job config", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.Jobs[0].Public = false

			_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.Public()).To(BeFalse())
		})

		It("marks job inactive when it is no longer in pipeline", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			config.Jobs = []atc.JobConfig{}

			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		Context("update job names but keeps history", func() {
			BeforeEach(func() {
				newJobConfig := atc.JobConfig{
					Name: "new-job",

					Public: true,

					Serial:       true,
					SerialGroups: []string{"serial-group-1", "serial-group-2"},

					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
						},
						{
							Config: &atc.TaskStep{
								Name:       "some-task",
								ConfigPath: "some/config/path.yml",
							},
						},
						{
							Config: &atc.PutStep{
								Name: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
						},
						{
							Config: &atc.DoStep{
								Steps: []atc.Step{
									{
										Config: &atc.TaskStep{
											Name:       "some-nested-task",
											ConfigPath: "some/config/path.yml",
										},
									},
								},
							},
						},
					},
				}

				config.Jobs = append(config.Jobs, newJobConfig)
			})

			It("should handle when there are multiple name changes", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				job, _, _ := pipeline.Job("some-job")
				otherJob, _, _ := pipeline.Job("new-job")

				config.Jobs[0].Name = "new-job"
				config.Jobs[0].OldName = "some-job"

				config.Jobs[3].Name = "new-other-job"
				config.Jobs[3].OldName = "new-job"

				updatedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				updatedJob, _, _ := updatedPipeline.Job("new-job")
				Expect(updatedJob.ID()).To(Equal(job.ID()))

				otherUpdatedJob, _, _ := updatedPipeline.Job("new-other-job")
				Expect(otherUpdatedJob.ID()).To(Equal(otherJob.ID()))
			})

			It("should handle when old job has the same name as new job", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				job, _, _ := pipeline.Job("some-job")

				config.Jobs[0].Name = "some-job"
				config.Jobs[0].OldName = "some-job"

				updatedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				updatedJob, _, _ := updatedPipeline.Job("some-job")
				Expect(updatedJob.ID()).To(Equal(job.ID()))
			})

			It("should return an error when there is a swap with job name", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				config.Jobs[0].Name = "new-job"
				config.Jobs[0].OldName = "some-job"

				config.Jobs[1].Name = "some-job"
				config.Jobs[1].OldName = "new-job"

				_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).To(HaveOccurred())
			})

			Context("when new job name is in database but is inactive", func() {
				It("should successfully update job name", func() {
					pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
					Expect(err).ToNot(HaveOccurred())

					config.Jobs = config.Jobs[:len(config.Jobs)-1]

					_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
					Expect(err).ToNot(HaveOccurred())

					config.Jobs[0].Name = "new-job"
					config.Jobs[0].OldName = "some-job"

					_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion()+1, false)
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("update resource names but keeps data", func() {

			BeforeEach(func() {

				config.Resources = append(config.Resources, atc.ResourceConfig{
					Name: "new-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				})
			})

			It("should successfully update resource name", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				resource, _, _ := pipeline.Resource("some-resource")

				config.Resources[0].Name = "renamed-resource"
				config.Resources[0].OldName = "some-resource"

				config.Jobs[0].PlanSequence = []atc.Step{
					{
						Config: &atc.GetStep{
							Name:     "some-input",
							Resource: "renamed-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed:  []string{"job-1", "job-2"},
							Trigger: true,
						},
					},
				}

				updatedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				updatedResource, _, _ := updatedPipeline.Resource("renamed-resource")
				Expect(updatedResource.ID()).To(Equal(resource.ID()))
			})

			It("should handle when there are multiple name changes", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				resource, _, _ := pipeline.Resource("some-resource")
				otherResource, _, _ := pipeline.Resource("new-resource")

				config.Resources[0].Name = "new-resource"
				config.Resources[0].OldName = "some-resource"

				config.Resources[1].Name = "new-other-resource"
				config.Resources[1].OldName = "new-resource"

				config.Jobs[0].PlanSequence = []atc.Step{
					{
						Config: &atc.GetStep{
							Name:     "some-input",
							Resource: "new-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed:  []string{"job-1", "job-2"},
							Trigger: true,
						},
					},
				}

				updatedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				updatedResource, _, _ := updatedPipeline.Resource("new-resource")
				Expect(updatedResource.ID()).To(Equal(resource.ID()))

				otherUpdatedResource, _, _ := updatedPipeline.Resource("new-other-resource")
				Expect(otherUpdatedResource.ID()).To(Equal(otherResource.ID()))
			})

			It("should handle when old resource has the same name as new resource", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				resource, _, _ := pipeline.Resource("some-resource")

				config.Resources[0].Name = "some-resource"
				config.Resources[0].OldName = "some-resource"

				updatedPipeline, _, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				updatedResource, _, _ := updatedPipeline.Resource("some-resource")
				Expect(updatedResource.ID()).To(Equal(resource.ID()))
			})

			It("should return an error when there is a swap with resource name", func() {
				pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				config.Resources[0].Name = "new-resource"
				config.Resources[0].OldName = "some-resource"

				config.Resources[1].Name = "some-resource"
				config.Resources[1].OldName = "new-resource"

				_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).To(HaveOccurred())
			})

			Context("when resource is renamed but has a disabled version", func() {
				var scenario *dbtest.Scenario
				var resource db.Resource

				BeforeEach(func() {
					scenario = dbtest.Setup(
						builder.WithPipeline(config),
						builder.WithBaseResourceType(dbConn, "some-type"),
						builder.WithDisabledVersion("some-resource", atc.Version{"disabled": "version"}),
					)

					versions, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 3}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))
					Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
					Expect(versions[0].Enabled).To(BeFalse())

					resource = scenario.Resource("some-resource")
				})

				It("the disabled version should still be in the resource's version history", func() {
					config.Resources[0].Name = "disabled-resource"
					config.Resources[0].OldName = "some-resource"
					config.Jobs[0].PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "disabled-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
						},
					}

					scenario.Run(
						builder.WithPipeline(config),
						// Imitate a check run that found no new versions
						builder.WithResourceVersions("disabled-resource"),
					)

					updatedResource := scenario.Resource("disabled-resource")
					Expect(updatedResource.ID()).To(Equal(resource.ID()))

					versions, _, found, err := updatedResource.Versions(db.Page{Limit: 3}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))
					Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
					Expect(versions[0].Enabled).To(BeFalse())
				})
			})

			Context("when new resource exists but the version is pinned", func() {
				var scenario *dbtest.Scenario
				var resource db.Resource

				BeforeEach(func() {
					scenario = dbtest.Setup(
						builder.WithPipeline(config),
						builder.WithBaseResourceType(dbConn, "some-type"),
						builder.WithResourceVersions(
							"some-resource",
							atc.Version{"version": "v1"},
							atc.Version{"version": "v2"},
							atc.Version{"version": "v3"},
						),
						builder.WithPinnedVersion("some-resource", atc.Version{"version": "v1"}),
					)

					resource = scenario.Resource("some-resource")
				})

				It("should not change the pinned version", func() {
					config.Resources[0].Name = "pinned-resource"
					config.Resources[0].OldName = "some-resource"
					config.Jobs[0].PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "pinned-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
						},
					}

					scenario.Run(
						builder.WithPipeline(config),
					)

					updatedResource := scenario.Resource("pinned-resource")
					Expect(updatedResource.ID()).To(Equal(resource.ID()))
					Expect(updatedResource.APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
				})
			})
		})

		It("removes task caches for jobs that are no longer in pipeline", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = taskCacheFactory.FindOrCreate(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = taskCacheFactory.FindOrCreate(job.ID(), "some-nested-task", "some-path")
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-nested-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			config.Jobs = []atc.JobConfig{}

			_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-nested-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("removes task caches for tasks that are no longer exist", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = taskCacheFactory.FindOrCreate(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = taskCacheFactory.FindOrCreate(job.ID(), "some-nested-task", "some-path")
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-nested-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			config.Jobs = []atc.JobConfig{
				{
					Name: "some-job",
					PlanSequence: []atc.Step{
						{
							Config: &atc.TaskStep{
								Name:       "some-other-task",
								ConfigPath: "some/config/path.yml",
							},
						},
					},
				},
			}

			_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-nested-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("should not remove task caches in other pipeline", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline, _, err := team.SavePipeline(atc.PipelineRef{Name: "other-pipeline"}, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = taskCacheFactory.FindOrCreate(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherJob, found, err := otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = taskCacheFactory.FindOrCreate(otherJob.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(otherJob.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			config.Jobs = []atc.JobConfig{
				{
					Name: "some-job",
					PlanSequence: []atc.Step{
						{
							Config: &atc.TaskStep{
								Name:       "some-other-task",
								ConfigPath: "some/config/path.yml",
							},
						},
					},
				},
			}

			_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = taskCacheFactory.Find(job.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = taskCacheFactory.Find(otherJob.ID(), "some-task", "some-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("creates all of the serial groups from the jobs in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			serialGroups := []SerialGroup{}
			rows, err := dbConn.Query("SELECT job_id, serial_group FROM jobs_serial_groups")
			Expect(err).ToNot(HaveOccurred())

			for rows.Next() {
				var serialGroup SerialGroup
				err = rows.Scan(&serialGroup.JobID, &serialGroup.Name)
				Expect(err).ToNot(HaveOccurred())
				serialGroups = append(serialGroups, serialGroup)
			}

			job, found, err := savedPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(serialGroups).To(ConsistOf([]SerialGroup{
				{
					JobID: job.ID(),
					Name:  "serial-group-1",
				},
				{
					JobID: job.ID(),
					Name:  "serial-group-2",
				},
			}))
		})

		It("saves tags in the jobs table", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, otherConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := savedPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Tags()).To(Equal([]string{"some-group"}))
		})

		It("saves tags in the jobs table based on globs", func() {
			otherConfig.Groups[0].Jobs = []string{"*-other-job"}
			savedPipeline, _, err := team.SavePipeline(pipelineRef, otherConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := savedPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Tags()).To(Equal([]string{"some-group"}))
		})

		It("updates tags in the jobs table", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineRef, otherConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := savedPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Tags()).To(Equal([]string{"some-group"}))

			otherConfig.Groups = atc.GroupConfigs{
				{
					Name: "some-other-group",
					Jobs: []string{"job-1", "job-2", "some-other-job"},
				},
				{
					Name: "some-another-group",
					Jobs: []string{"*-other-job"},
				},
			}

			savedPipeline, _, err = team.SavePipeline(pipelineRef, otherConfig, savedPipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = savedPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Tags()).To(ConsistOf([]string{"some-another-group", "some-other-group"}))
		})

		It("it returns created as false when updated", func() {
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			_, created, err := team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeFalse())
		})

		It("deletes old job pipes and inserts new ones", func() {
			config = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"resource-1", "resource-2"},
					},
				},

				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-resource-type",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "job-1",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "job-2",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "some-job",

						Public: true,

						Serial:       true,
						SerialGroups: []string{"serial-group-1", "serial-group-2"},

						PlanSequence: []atc.Step{
							{
								Config: &atc.DoStep{
									Steps: []atc.Step{
										{
											Config: &atc.GetStep{
												Name:     "other-input",
												Resource: "some-resource",
											},
										},
									},
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
									Passed:  []string{"job-1", "job-2"},
									Trigger: true,
								},
							},
							{
								Config: &atc.TaskStep{
									Name:       "some-task",
									Privileged: true,
									ConfigPath: "some/config/path.yml",
									Config: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
								},
							},
							{
								Config: &atc.PutStep{
									Name: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
								},
							},
						},
					},
				},
			}

			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, true)
			Expect(err).ToNot(HaveOccurred())

			rows, err := psql.Select("name", "job_id", "resource_id", "passed_job_id").
				From("job_inputs").
				Where(sq.Expr(`job_id in (
					SELECT j.id
					FROM jobs j
					WHERE j.pipeline_id = $1
				)`, pipeline.ID())).
				RunWith(dbConn).
				Query()
			Expect(err).ToNot(HaveOccurred())

			type jobPipe struct {
				name        string
				jobID       int
				resourceID  int
				passedJobID int
			}

			var jobPipes []jobPipe
			for rows.Next() {
				var jp jobPipe
				var passedJob sql.NullInt64
				err = rows.Scan(&jp.name, &jp.jobID, &jp.resourceID, &passedJob)
				Expect(err).ToNot(HaveOccurred())

				if passedJob.Valid {
					jp.passedJobID = int(passedJob.Int64)
				}

				jobPipes = append(jobPipes, jp)
			}

			job1, found, err := pipeline.Job("job-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job2, found, err := pipeline.Job("job-2")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			someJob, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			someResource, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(jobPipes).To(ConsistOf(
				jobPipe{
					name:       "some-resource",
					jobID:      job1.ID(),
					resourceID: someResource.ID(),
				},
				jobPipe{
					name:       "some-resource",
					jobID:      job2.ID(),
					resourceID: someResource.ID(),
				},
				jobPipe{
					name:       "other-input",
					jobID:      someJob.ID(),
					resourceID: someResource.ID(),
				},
				jobPipe{
					name:        "some-input",
					jobID:       someJob.ID(),
					resourceID:  someResource.ID(),
					passedJobID: job1.ID(),
				},
				jobPipe{
					name:        "some-input",
					jobID:       someJob.ID(),
					resourceID:  someResource.ID(),
					passedJobID: job2.ID(),
				},
			))

			config = atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-resource-type",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "job-2",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "some-job",

						Public: true,

						Serial:       true,
						SerialGroups: []string{"serial-group-1", "serial-group-2"},

						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
									Passed:  []string{"job-2"},
									Trigger: true,
								},
							},
							{
								Config: &atc.TaskStep{
									Name:       "some-task",
									Privileged: true,
									ConfigPath: "some/config/path.yml",
									Config: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
								},
							},
							{
								Config: &atc.PutStep{
									Name: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
								},
							},
						},
					},
				},
			}

			_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			rows, err = psql.Select("name", "job_id", "resource_id", "passed_job_id").
				From("job_inputs").
				Where(sq.Expr(`job_id in (
					SELECT j.id
					FROM jobs j
					WHERE j.pipeline_id = $1
				)`, pipeline.ID())).
				RunWith(dbConn).
				Query()
			Expect(err).ToNot(HaveOccurred())

			var newJobPipes []jobPipe
			for rows.Next() {
				var jp jobPipe
				var passedJob sql.NullInt64
				err = rows.Scan(&jp.name, &jp.jobID, &jp.resourceID, &passedJob)
				Expect(err).ToNot(HaveOccurred())

				if passedJob.Valid {
					jp.passedJobID = int(passedJob.Int64)
				}

				newJobPipes = append(newJobPipes, jp)
			}

			Expect(newJobPipes).To(ConsistOf(
				jobPipe{
					name:       "some-resource",
					jobID:      job2.ID(),
					resourceID: someResource.ID(),
				},
				jobPipe{
					name:        "some-input",
					jobID:       someJob.ID(),
					resourceID:  someResource.ID(),
					passedJobID: job2.ID(),
				},
			))
		})

		Context("updating an existing pipeline", func() {
			It("maintains paused if the pipeline is paused", func() {
				_, _, err := team.SavePipeline(pipelineRef, config, 0, true)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err := team.Pipeline(pipelineRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeTrue())

				_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err = team.Pipeline(pipelineRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeTrue())
			})

			It("maintains unpaused if the pipeline is unpaused", func() {
				_, _, err := team.SavePipeline(pipelineRef, config, 0, false)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err := team.Pipeline(pipelineRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeFalse())

				_, _, err = team.SavePipeline(pipelineRef, config, pipeline.ConfigVersion(), true)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err = team.Pipeline(pipelineRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeFalse())
			})

			It("resets to unarchived", func() {
				team.SavePipeline(pipelineRef, config, 0, false)
				pipeline, _, _ := team.Pipeline(pipelineRef)
				pipeline.Archive()

				team.SavePipeline(pipelineRef, config, db.ConfigVersion(0), true)
				pipeline.Reload()
				Expect(pipeline.Archived()).To(BeFalse(), "the pipeline remained archived")
			})
		})

		It("can lookup a pipeline by name", func() {
			otherPipelineFilter := atc.PipelineRef{Name: "an-other-pipeline-name"}

			_, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())
			_, _, err = team.SavePipeline(otherPipelineFilter, otherConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Name()).To(Equal(pipelineName))
			Expect(pipeline.ID()).ToNot(Equal(0))
			resourceTypes, err := pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			resources, err := pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			jobs, err := pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			jobConfigs, err := jobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        pipeline.Groups(),
				Resources:     resources.Configs(),
				ResourceTypes: resourceTypes.Configs(),
				Jobs:          jobConfigs,
			}, config)

			otherPipeline, found, err := team.Pipeline(otherPipelineFilter)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(otherPipeline.Name()).To(Equal(otherPipelineFilter.Name))
			Expect(otherPipeline.ID()).ToNot(Equal(0))
			otherResourceTypes, err := otherPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			otherResources, err := otherPipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			otherJobs, err := otherPipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			otherJobConfigs, err := otherJobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        otherPipeline.Groups(),
				Resources:     otherResources.Configs(),
				ResourceTypes: otherResourceTypes.Configs(),
				Jobs:          otherJobConfigs,
			}, otherConfig)

		})

		It("can manage multiple pipeline configurations", func() {
			otherPipelineFilter := atc.PipelineRef{Name: "an-other-pipeline-name"}

			By("being able to save the config")
			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline, _, err := team.SavePipeline(otherPipelineFilter, otherConfig, 0, false)
			Expect(err).ToNot(HaveOccurred())

			By("returning the saved config to later gets")
			resourceTypes, err := pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			resources, err := pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			jobs, err := pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			jobConfigs, err := jobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        pipeline.Groups(),
				Resources:     resources.Configs(),
				ResourceTypes: resourceTypes.Configs(),
				Jobs:          jobConfigs,
			}, config)

			otherResourceTypes, err := otherPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			otherResources, err := otherPipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			otherJobs, err := otherPipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			otherJobConfigs, err := otherJobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        otherPipeline.Groups(),
				Resources:     otherResources.Configs(),
				ResourceTypes: otherResourceTypes.Configs(),
				Jobs:          otherJobConfigs,
			}, otherConfig)

			By("returning the saved groups")
			returnedGroups := pipeline.Groups()
			Expect(returnedGroups).To(Equal(config.Groups))

			otherReturnedGroups := otherPipeline.Groups()
			Expect(otherReturnedGroups).To(Equal(otherConfig.Groups))

			updatedConfig := config

			updatedConfig.Groups = append(config.Groups, atc.GroupConfig{
				Name: "new-group",
				Jobs: []string{"new-job-1", "new-job-2"},
			})

			updatedConfig.Resources = append(config.Resources, atc.ResourceConfig{
				Name: "new-resource",
				Type: "new-type",
				Source: atc.Source{
					"new-source-config": "new-value",
				},
			})

			updatedConfig.Jobs = append(config.Jobs, atc.JobConfig{
				Name: "new-job",
				PlanSequence: []atc.Step{
					{
						Config: &atc.GetStep{
							Name:     "new-input",
							Resource: "new-resource",
							Params: atc.Params{
								"new-param": "new-value",
							},
						},
					},
					{
						Config: &atc.TaskStep{
							Name:       "some-task",
							ConfigPath: "new/config/path.yml",
						},
					},
				},
			})

			By("not allowing non-sequential updates")
			_, _, err = team.SavePipeline(pipelineRef, updatedConfig, pipeline.ConfigVersion()-1, false)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			_, _, err = team.SavePipeline(pipelineRef, updatedConfig, pipeline.ConfigVersion()+10, false)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			_, _, err = team.SavePipeline(otherPipelineFilter, updatedConfig, otherPipeline.ConfigVersion()-1, false)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			_, _, err = team.SavePipeline(otherPipelineFilter, updatedConfig, otherPipeline.ConfigVersion()+10, false)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			By("being able to update the config with a valid con")
			pipeline, _, err = team.SavePipeline(pipelineRef, updatedConfig, pipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())
			otherPipeline, _, err = team.SavePipeline(otherPipelineFilter, updatedConfig, otherPipeline.ConfigVersion(), false)
			Expect(err).ToNot(HaveOccurred())

			By("returning the updated config")
			resourceTypes, err = pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			resources, err = pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			jobs, err = pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			jobConfigs, err = jobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        pipeline.Groups(),
				Resources:     resources.Configs(),
				ResourceTypes: resourceTypes.Configs(),
				Jobs:          jobConfigs,
			}, updatedConfig)

			otherResourceTypes, err = otherPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			otherResources, err = otherPipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			otherJobs, err = otherPipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			otherJobConfigs, err = otherJobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        otherPipeline.Groups(),
				Resources:     otherResources.Configs(),
				ResourceTypes: otherResourceTypes.Configs(),
				Jobs:          otherJobConfigs,
			}, updatedConfig)

			By("returning the saved groups")
			returnedGroups = pipeline.Groups()
			Expect(returnedGroups).To(Equal(updatedConfig.Groups))

			otherReturnedGroups = otherPipeline.Groups()
			Expect(otherReturnedGroups).To(Equal(updatedConfig.Groups))
		})

		It("should return sorted resources and resource_types", func() {
			config.ResourceTypes = append(config.ResourceTypes, atc.ResourceType{
				Name: "new-resource-type",
				Type: "new-type",
				Source: atc.Source{
					"new-source-config": "new-value",
				},
			})

			config.Resources = append(config.Resources, atc.ResourceConfig{
				Name: "new-resource",
				Type: "new-type",
				Source: atc.Source{
					"new-source-config": "new-value",
				},
			})

			pipeline, _, err := team.SavePipeline(pipelineRef, config, 0, false)
			Expect(err).ToNot(HaveOccurred())

			resourceTypes, err := pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			rtConfigs := resourceTypes.Configs()
			Expect(rtConfigs[0].Name).To(Equal(config.ResourceTypes[1].Name)) // "new-resource-type"
			Expect(rtConfigs[1].Name).To(Equal(config.ResourceTypes[0].Name)) // "some-resource-type"

			resources, err := pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			rConfigs := resources.Configs()
			Expect(rConfigs[0].Name).To(Equal(config.Resources[1].Name)) // "new-resource"
			Expect(rConfigs[1].Name).To(Equal(config.Resources[0].Name)) // "some-resource"
		})

		Context("when there are multiple teams", func() {
			It("can allow pipelines with the same name across teams", func() {
				pipelineRef := atc.PipelineRef{Name: "steve"}

				teamPipeline, _, err := team.SavePipeline(pipelineRef, config, 0, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(teamPipeline.Paused()).To(BeTrue())

				By("allowing you to save a pipeline with the same name in another team")
				otherTeamPipeline, _, err := otherTeam.SavePipeline(pipelineRef, otherConfig, 0, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(otherTeamPipeline.Paused()).To(BeTrue())

				By("updating the pipeline config for the correct team's pipeline")
				_, _, err = team.SavePipeline(pipelineRef, otherConfig, teamPipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				_, _, err = otherTeam.SavePipeline(pipelineRef, config, otherTeamPipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				By("cannot cross update configs")
				_, _, err = team.SavePipeline(pipelineRef, otherConfig, otherTeamPipeline.ConfigVersion(), false)
				Expect(err).To(HaveOccurred())

				_, _, err = team.SavePipeline(pipelineRef, otherConfig, otherTeamPipeline.ConfigVersion(), true)
				Expect(err).To(HaveOccurred())
			})
		})

		It("does not deadlock when concurrently setting pipelines and running checks/gets", func() {
			// enable concurrent use of database. this is set to 1 by default to
			// ensure methods don't require more than one in a single connection,
			// which can cause deadlocking as the pool is limited.
			dbConn.SetMaxOpenConns(4)

			config := atc.Config{
				ResourceTypes: atc.ResourceTypes{
					{
						Name:   "some-resource-type",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"foo": "bar"},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-resource-type",
						Source: atc.Source{"foo": "baz"},
					},
				},
			}
			scenario := dbtest.Setup(
				builder.WithBaseWorker(),
				builder.WithPipeline(config),
			)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			errChan := make(chan error, 1)

			var wg sync.WaitGroup

			loopUntilTimeoutOrPanic := func(name string, run func(i int)) func() {
				wg.Add(1)
				return func() {
					defer wg.Done()
					defer func() {
						if err := recover(); err != nil {
							errChan <- fmt.Errorf("%s error: %v", name, err)
						}
					}()

					i := 0
					for {
						select {
						case <-ctx.Done():
							return
						default:
							run(i)
							i++
						}
					}
				}
			}

			pipeline := scenario.Pipeline
			go loopUntilTimeoutOrPanic("set pipeline", func(_ int) {
				var err error
				pipeline, _, err = scenario.Team.SavePipeline(
					atc.PipelineRef{Name: pipeline.Name()},
					config,
					pipeline.ConfigVersion(),
					false,
				)
				if err != nil {
					panic(err)
				}
			})()

			resource := scenario.Resource("some-resource")
			go loopUntilTimeoutOrPanic("check resource", func(i int) {
				scenario.Run(
					builder.WithResourceVersions(resource.Name(), atc.Version{"v": strconv.Itoa(i / 10)}),
				)
			})()

			build, err := defaultJob.CreateBuild("some-user")
			Expect(err).ToNot(HaveOccurred())

			go loopUntilTimeoutOrPanic("check resource type", func(i int) {
				scenario.Run(
					builder.WithResourceTypeVersions("some-resource-type", atc.Version{"foo": strconv.Itoa(i / 10)}),
				)
			})()

			go loopUntilTimeoutOrPanic("get resource", func(i int) {
				customResourceTypeCache, err := resourceCacheFactory.FindOrCreateResourceCache(
					db.ForBuild(build.ID()),
					dbtest.BaseResourceType,
					atc.Version{"foo": strconv.Itoa(i / 10)},
					atc.Source{"foo": "bar"},
					atc.Params{},
					nil,
				)
				if err != nil {
					panic(err)
				}

				_, err = resourceCacheFactory.FindOrCreateResourceCache(
					db.ForBuild(build.ID()),
					resource.Type(),
					atc.Version{"v": strconv.Itoa(i / 10)},
					resource.Source(),
					atc.Params{},
					customResourceTypeCache,
				)
				if err != nil {
					panic(err)
				}
			})()

			go func() {
				wg.Wait()
				close(errChan)
			}()

			Expect(<-errChan).ToNot(HaveOccurred())
		})
	})

	Describe("RenamePipeline", func() {
		It("renames individual pipelines", func() {
			found, err := defaultTeam.RenamePipeline("default-pipeline", "new-pipeline")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = defaultPipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(defaultPipeline.Name()).To(Equal("new-pipeline"))
		})

		Context("when multiple pipeline instances have the same name", func() {
			var (
				p1 db.Pipeline
				p2 db.Pipeline
				p3 db.Pipeline
			)

			BeforeEach(func() {
				var err error
				p1, _, err = defaultTeam.SavePipeline(atc.PipelineRef{
					Name:         "release",
					InstanceVars: atc.InstanceVars{"version": "6.7.x"},
				}, defaultPipelineConfig, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				p2, _, err = defaultTeam.SavePipeline(atc.PipelineRef{
					Name:         "release",
					InstanceVars: atc.InstanceVars{"version": "7.0.x"},
				}, defaultPipelineConfig, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				p3, _, err = defaultTeam.SavePipeline(atc.PipelineRef{
					Name:         "release",
					InstanceVars: nil,
				}, defaultPipelineConfig, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("renames every instance", func() {
				found, err := defaultTeam.RenamePipeline("release", "new-pipeline")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = p1.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(p1.Name()).To(Equal("new-pipeline"))

				_, err = p2.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(p2.Name()).To(Equal("new-pipeline"))

				_, err = p3.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(p3.Name()).To(Equal("new-pipeline"))
			})
		})

		Context("when there are no pipelines with the old name", func() {
			It("returns not found", func() {
				found, err := defaultTeam.RenamePipeline("blah-blah-blah", "new-pipeline")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("FindCheckContainers", func() {
		var (
			logger lager.Logger
		)

		expiries := db.ContainerOwnerExpiries{
			Min: 5 * time.Minute,
			Max: 1 * time.Hour,
		}

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("db-test")
		})

		Context("when pipeline exists", func() {
			Context("when resource exists", func() {
				Context("when check container for resource exists", func() {
					var resourceContainer db.CreatingContainer
					var resourceConfig db.ResourceConfig

					BeforeEach(func() {
						var err error
						resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
							defaultResource.Type(),
							defaultResource.Source(),
							nil,
						)
						Expect(err).ToNot(HaveOccurred())

						scope, err := resourceConfig.FindOrCreateScope(intptr(defaultResource.ID()))
						Expect(err).ToNot(HaveOccurred())

						err = defaultResource.SetResourceConfigScope(scope)
						Expect(err).ToNot(HaveOccurred())

						resourceContainer, err = defaultWorker.CreateContainer(
							db.NewResourceConfigCheckSessionContainerOwner(
								resourceConfig.ID(),
								resourceConfig.OriginBaseResourceType().ID,
								expiries,
							),
							db.ContainerMetadata{},
						)
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns check container for resource", func() {
						containers, checkContainersExpiresAt, err := defaultTeam.FindCheckContainers(logger, defaultPipelineRef, "some-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(containers).To(HaveLen(1))
						Expect(containers[0].ID()).To(Equal(resourceContainer.ID()))
						Expect(checkContainersExpiresAt).To(HaveLen(1))
						Expect(checkContainersExpiresAt[resourceContainer.ID()]).ToNot(BeNil())
					})

					Context("when there are multiple resources with the same resource config", func() {
						var (
							otherPipeline          db.Pipeline
							otherResource          db.Resource
							otherResourceContainer db.CreatingContainer
							otherPipelineRef       atc.PipelineRef
							found                  bool
							err                    error
						)

						BeforeEach(func() {
							otherPipelineRef = atc.PipelineRef{Name: "other-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
							otherPipeline, _, err = defaultTeam.SavePipeline(otherPipelineRef, atc.Config{
								Resources: atc.ResourceConfigs{
									{
										Name: "some-resource",
										Type: "some-base-resource-type",
										Source: atc.Source{
											"some": "source",
										},
									},
								},
							}, db.ConfigVersion(0), false)
							Expect(err).NotTo(HaveOccurred())

							otherResource, found, err = otherPipeline.Resource("some-resource")
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())

							resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
								otherResource.Type(),
								otherResource.Source(),
								nil,
							)
							Expect(err).ToNot(HaveOccurred())

							scope, err := resourceConfig.FindOrCreateScope(intptr(otherResource.ID()))
							Expect(err).ToNot(HaveOccurred())

							err = otherResource.SetResourceConfigScope(scope)
							Expect(err).ToNot(HaveOccurred())

							otherResourceContainer, _, err = defaultWorker.FindContainer(
								db.NewResourceConfigCheckSessionContainerOwner(
									resourceConfig.ID(),
									resourceConfig.OriginBaseResourceType().ID,
									expiries,
								),
							)
							Expect(err).ToNot(HaveOccurred())
						})

						It("returns the same check container", func() {
							containers, checkContainersExpiresAt, err := defaultTeam.FindCheckContainers(logger, otherPipelineRef, "some-resource")
							Expect(err).ToNot(HaveOccurred())
							Expect(containers).To(HaveLen(1))
							Expect(containers[0].ID()).To(Equal(otherResourceContainer.ID()))
							Expect(otherResourceContainer.ID()).To(Equal(resourceContainer.ID()))
							Expect(checkContainersExpiresAt).To(HaveLen(1))
							Expect(checkContainersExpiresAt[resourceContainer.ID()]).ToNot(BeNil())
						})
					})
				})

				Context("when check container does not exist", func() {
					It("returns empty list", func() {
						containers, checkContainersExpiresAt, err := defaultTeam.FindCheckContainers(logger, defaultPipelineRef, "some-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(containers).To(BeEmpty())
						Expect(checkContainersExpiresAt).To(BeEmpty())
					})
				})
			})

			Context("when resource does not exist", func() {
				It("returns empty list", func() {
					containers, checkContainersExpiresAt, err := defaultTeam.FindCheckContainers(logger, defaultPipelineRef, "non-existent-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(containers).To(BeEmpty())
					Expect(checkContainersExpiresAt).To(BeEmpty())
				})
			})
		})

		Context("when pipeline does not exist", func() {
			It("returns empty list", func() {
				containers, checkContainersExpiresAt, err := defaultTeam.FindCheckContainers(logger, atc.PipelineRef{Name: "non-existent-pipeline"}, "some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(BeEmpty())
				Expect(checkContainersExpiresAt).To(BeEmpty())
			})
		})
	})

	Describe("IsCheckContainer", func() {
		Context("when the container is associated with a resource config check session", func() {
			var resourceContainer db.Container
			var scenario *dbtest.Scenario

			expiries := db.ContainerOwnerExpiries{
				Min: 5 * time.Minute,
				Max: 1 * time.Hour,
			}

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some": "source",
								},
							},
						},
					}),
					builder.WithWorker(atc.Worker{
						ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
						Name:            "some-default-worker",
						GardenAddr:      "3.4.5.6:7777",
						BaggageclaimURL: "7.8.9.10:7878",
					}),
					builder.WithResourceVersions("some-resource"),
				)

				rc, found, err := resourceConfigFactory.FindResourceConfigByID(scenario.Resource("some-resource").ResourceConfigID())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceContainer, err = scenario.Workers[0].CreateContainer(
					db.NewResourceConfigCheckSessionContainerOwner(
						rc.ID(),
						rc.OriginBaseResourceType().ID,
						expiries,
					),
					db.ContainerMetadata{},
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns true", func() {
				is, err := team.IsCheckContainer(resourceContainer.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(is).To(BeTrue())
			})
		})

		Context("when the container is owned by a team", func() {
			var createdContainer db.Container
			var scenario *dbtest.Scenario

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
					}),
					builder.WithBaseWorker(),
				)

				build, err := scenario.Job("some-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				creatingContainer, err := scenario.Workers[0].CreateContainer(
					db.NewBuildStepContainerOwner(
						build.ID(),
						atc.PlanID("some-job"),
						scenario.Team.ID(),
					),
					db.ContainerMetadata{Type: "task", StepName: "some-task"},
				)
				Expect(err).ToNot(HaveOccurred())

				createdContainer, err = creatingContainer.Created()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				is, err := team.IsCheckContainer(createdContainer.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(is).To(BeFalse())
			})
		})
	})

	Describe("IsContainerWithinTeam", func() {
		Context("when the container is a check container", func() {
			var resourceContainer db.Container
			var scenario *dbtest.Scenario

			expiries := db.ContainerOwnerExpiries{
				Min: 5 * time.Minute,
				Max: 1 * time.Hour,
			}

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some": "source",
								},
							},
						},
					}),
					builder.WithWorker(atc.Worker{
						ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
						Name:            "some-default-worker",
						GardenAddr:      "3.4.5.6:7777",
						BaggageclaimURL: "7.8.9.10:7878",
					}),
					builder.WithResourceVersions("some-resource"),
				)

				rc, found, err := resourceConfigFactory.FindResourceConfigByID(scenario.Resource("some-resource").ResourceConfigID())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceContainer, err = scenario.Workers[0].CreateContainer(
					db.NewResourceConfigCheckSessionContainerOwner(
						rc.ID(),
						rc.OriginBaseResourceType().ID,
						expiries,
					),
					db.ContainerMetadata{},
				)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the container does belong on the team", func() {
				var ok bool

				BeforeEach(func() {
					var err error
					ok, err = scenario.Team.IsContainerWithinTeam(resourceContainer.Handle(), true)
					Expect(err).ToNot(HaveOccurred())
				})

				It("finds the container for the team", func() {
					Expect(ok).To(BeTrue())
				})
			})

			Context("when the container does not belong on the team", func() {
				var ok bool

				BeforeEach(func() {
					var err error
					ok, err = team.IsContainerWithinTeam(resourceContainer.Handle(), true)
					Expect(err).ToNot(HaveOccurred())
				})

				It("finds the container for the team", func() {
					Expect(ok).To(BeFalse())
				})
			})
		})

		Context("when the container is owned by a team", func() {
			var createdContainer db.Container
			var scenario *dbtest.Scenario

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
					}),
					builder.WithBaseWorker(),
				)

				build, err := scenario.Job("some-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				creatingContainer, err := scenario.Workers[0].CreateContainer(db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job"), scenario.Team.ID()), db.ContainerMetadata{Type: "task", StepName: "some-task"})
				Expect(err).ToNot(HaveOccurred())

				createdContainer, err = creatingContainer.Created()
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the container does belong on the team", func() {
				var ok bool

				BeforeEach(func() {
					var err error
					ok, err = scenario.Team.IsContainerWithinTeam(createdContainer.Handle(), false)
					Expect(err).ToNot(HaveOccurred())
				})

				It("finds the container for the team", func() {
					Expect(ok).To(BeTrue())
				})
			})

			Context("when the container does not belong on the team", func() {
				var ok bool

				BeforeEach(func() {
					var err error
					ok, err = team.IsContainerWithinTeam(createdContainer.Handle(), false)
					Expect(err).ToNot(HaveOccurred())
				})

				It("finds the container for the team", func() {
					Expect(ok).To(BeFalse())
				})
			})
		})
	})
})
