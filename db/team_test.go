package db_test

import (
	"encoding/json"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/creds/credsfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

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

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			metaContainers = make(map[db.ContainerMetadata][]db.Container)
			for _, meta := range sampleMetadata {
				firstContainerCreating, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job")), meta)
				Expect(err).ToNot(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], firstContainerCreating)

				secondContainerCreating, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job")), meta)
				Expect(err).ToNot(HaveOccurred())

				secondContainerCreated, err := secondContainerCreating.Created()
				Expect(err).ToNot(HaveOccurred())

				metaContainers[meta] = append(metaContainers[meta], secondContainerCreated)

				thirdContainerCreating, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job")), meta)
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

	Describe("FindCheckContainers", func() {
		var (
			fakeVariablesFactory *credsfakes.FakeVariablesFactory
			variables            creds.Variables
		)

		expiries := db.ContainerOwnerExpiries{
			GraceTime: 2 * time.Minute,
			Min:       5 * time.Minute,
			Max:       1 * time.Hour,
		}

		BeforeEach(func() {
			fakeVariablesFactory = new(credsfakes.FakeVariablesFactory)
			variables = template.StaticVariables{}
			fakeVariablesFactory.NewVariablesReturns(variables)
		})

		Context("when pipeline exists", func() {
			Context("when resource exists", func() {
				Context("when check container for resource exists", func() {
					var resourceContainer db.CreatingContainer
					var resourceConfigCheckSession db.ResourceConfigCheckSession

					BeforeEach(func() {
						pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
						Expect(err).ToNot(HaveOccurred())

						resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
							logger,
							defaultResource.Type(),
							defaultResource.Source(),
							creds.NewVersionedResourceTypes(variables, pipelineResourceTypes.Deserialize()),
							expiries,
						)
						Expect(err).ToNot(HaveOccurred())

						resourceContainer, err = defaultTeam.CreateContainer(
							"default-worker",
							db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, defaultTeam.ID()),
							db.ContainerMetadata{},
						)
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns check container for resource", func() {
						containers, err := defaultTeam.FindCheckContainers(logger, "default-pipeline", "some-resource", fakeVariablesFactory)
						Expect(err).ToNot(HaveOccurred())
						Expect(containers).To(ContainElement(resourceContainer))
					})

					Context("when another team has a container with the same resource config", func() {
						BeforeEach(func() {
							_, err := otherTeam.CreateContainer(
								"default-worker",
								db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, defaultTeam.ID()),
								db.ContainerMetadata{},
							)
							Expect(err).ToNot(HaveOccurred())
						})

						It("only returns container for current team", func() {
							containers, err := defaultTeam.FindCheckContainers(logger, "default-pipeline", "some-resource", fakeVariablesFactory)
							Expect(err).ToNot(HaveOccurred())
							Expect(containers).To(HaveLen(1))
							Expect(containers).To(ContainElement(resourceContainer))
						})
					})
				})

				Context("when check container does not exist", func() {
					It("returns empty list", func() {
						containers, err := defaultTeam.FindCheckContainers(logger, "default-pipeline", "some-resource", fakeVariablesFactory)
						Expect(err).ToNot(HaveOccurred())
						Expect(containers).To(BeEmpty())
					})
				})
			})

			Context("when resource does not exist", func() {
				It("returns empty list", func() {
					containers, err := defaultTeam.FindCheckContainers(logger, "default-pipeline", "non-existent-resource", fakeVariablesFactory)
					Expect(err).ToNot(HaveOccurred())
					Expect(containers).To(BeEmpty())
				})
			})
		})

		Context("when pipeline does not exist", func() {
			It("returns empty list", func() {
				containers, err := defaultTeam.FindCheckContainers(logger, "non-existent-pipeline", "some-resource", fakeVariablesFactory)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(BeEmpty())
			})
		})
	})

	Describe("FindContainerByHandle", func() {
		var createdContainer db.CreatedContainer

		BeforeEach(func() {
			job, found, err := defaultPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), atc.PlanID("some-job")), db.ContainerMetadata{Type: "task", StepName: "some-task"})
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

		It("does not find container for another team", func() {
			_, found, err := otherTeam.FindContainerByHandle(createdContainer.Handle())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
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
				container, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan"), containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainer(container.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).ToNot(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForContainer(container.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			var container db.CreatedContainer

			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan"), containerMetadata)
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

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForContainer(container.Handle())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
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

	Describe("FindWorkerForContainerByOwner", func() {
		var containerMetadata db.ContainerMetadata
		var build db.Build
		var fakeOwner *dbfakes.FakeContainerOwner

		BeforeEach(func() {
			var err error
			containerMetadata = db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			fakeOwner = new(dbfakes.FakeContainerOwner)
			fakeOwner.FindReturns(sq.Eq{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
			}, true, nil)
			fakeOwner.CreateReturns(map[string]interface{}{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
			}, nil)
		})

		Context("when there is a creating container", func() {
			BeforeEach(func() {
				_, err := defaultTeam.CreateContainer(defaultWorker.Name(), fakeOwner, containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainerByOwner(fakeOwner)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).ToNot(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForContainerByOwner(fakeOwner)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is a created container", func() {
			BeforeEach(func() {
				creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), fakeOwner, containerMetadata)
				Expect(err).ToNot(HaveOccurred())

				_, err = creatingContainer.Created()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				worker, found, err := defaultTeam.FindWorkerForContainerByOwner(fakeOwner)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(worker).ToNot(BeNil())
				Expect(worker.Name()).To(Equal(defaultWorker.Name()))
			})

			It("does not find container for another team", func() {
				worker, found, err := otherTeam.FindWorkerForContainerByOwner(fakeOwner)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				bogusOwner := new(dbfakes.FakeContainerOwner)
				bogusOwner.FindReturns(sq.Eq{
					"build_id": build.ID() + 1,
					"plan_id":  "how-could-this-happen-to-me",
				}, true, nil)
				bogusOwner.CreateReturns(map[string]interface{}{
					"build_id": build.ID() + 1,
					"plan_id":  "how-could-this-happen-to-me",
				}, nil)

				worker, found, err := defaultTeam.FindWorkerForContainerByOwner(bogusOwner)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(worker).To(BeNil())
			})
		})
	})

	Describe("Updating Auth", func() {
		var (
			authProvider map[string]*json.RawMessage
		)

		BeforeEach(func() {
			data := []byte(`{"credit_card":"please"}`)
			authProvider = map[string]*json.RawMessage{
				"fake-provider": (*json.RawMessage)(&data),
			}
		})

		Describe("UpdateProviderAuth", func() {
			It("saves auth team info to the existing team", func() {
				err := team.UpdateProviderAuth(authProvider)
				Expect(err).ToNot(HaveOccurred())

				Expect(team.Auth()).To(Equal(authProvider))
			})

			It("saves github auth team info without over writing the basic auth", func() {
				err := team.UpdateProviderAuth(authProvider)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Pipelines", func() {
		var (
			pipelines []db.Pipeline
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
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
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = team.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the pipelines", func() {
				Expect(pipelines).To(Equal([]db.Pipeline{pipeline1, pipeline2}))
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
				_, _, err = team.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = team.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
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

	Describe("VisiblePipelines", func() {
		var (
			pipelines []db.Pipeline
			pipeline1 db.Pipeline
			pipeline2 db.Pipeline
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
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline2, _, err = otherTeam.SavePipeline("fake-pipeline-two", atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-fake"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				Expect(pipeline2.Expose()).To(Succeed())
				Expect(pipeline2.Reload()).To(BeTrue())
			})

			It("returns the pipelines", func() {
				Expect(pipelines).To(Equal([]db.Pipeline{pipeline1, pipeline2}))
			})

			Context("when the other team has a private pipeline", func() {
				BeforeEach(func() {
					var err error
					_, _, err = otherTeam.SavePipeline("fake-pipeline-three", atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "job-fake-again"},
						},
					}, db.ConfigVersion(1), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not return the other team private pipeline", func() {
					Expect(pipelines).To(Equal([]db.Pipeline{pipeline1, pipeline2}))
				})
			})
		})

		Context("when the team has no configured pipelines", func() {
			It("returns no pipelines", func() {
				Expect(pipelines).To(Equal([]db.Pipeline{}))
			})
		})
	})

	Describe("OrderPipelines", func() {
		var pipeline1 db.Pipeline
		var pipeline2 db.Pipeline
		var otherPipeline1 db.Pipeline
		var otherPipeline2 db.Pipeline

		BeforeEach(func() {
			var err error
			pipeline1, _, err = team.SavePipeline("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			pipeline2, _, err = team.SavePipeline("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline1, _, err = otherTeam.SavePipeline("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			otherPipeline2, _, err = otherTeam.SavePipeline("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		It("orders pipelines that belong to team (case insensitive)", func() {
			err := team.OrderPipelines([]string{"pipeline-name-b", "pipeline-name-a"})
			Expect(err).ToNot(HaveOccurred())

			err = otherTeam.OrderPipelines([]string{"pipeline-name-a", "pipeline-name-b"})
			Expect(err).ToNot(HaveOccurred())

			orderedPipelines, err := team.Pipelines()

			Expect(err).ToNot(HaveOccurred())
			Expect(orderedPipelines).To(HaveLen(2))
			Expect(orderedPipelines[0].ID()).To(Equal(pipeline2.ID()))
			Expect(orderedPipelines[1].ID()).To(Equal(pipeline1.ID()))

			otherTeamOrderedPipelines, err := otherTeam.Pipelines()
			Expect(err).ToNot(HaveOccurred())
			Expect(otherTeamOrderedPipelines).To(HaveLen(2))
			Expect(otherTeamOrderedPipelines[0].ID()).To(Equal(otherPipeline1.ID()))
			Expect(otherTeamOrderedPipelines[1].ID()).To(Equal(otherPipeline2.ID()))
		})

		Context("when pipeline does not exist", func() {
			It("returns error ", func() {
				err := otherTeam.OrderPipelines([]string{"pipeline-name-a", "pipeline-does-not-exist"})
				Expect(err).To(HaveOccurred())
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

	Describe("PrivateAndPublicBuilds", func() {
		Context("when there are no builds", func() {
			It("returns an empty list of builds", func() {
				builds, pagination, err := team.PrivateAndPublicBuilds(db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Next).To(BeNil())
				Expect(pagination.Previous).To(BeNil())
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
				pipeline, _, err = team.SavePipeline("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				job, found, err := pipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				for i := 3; i < 5; i++ {
					build, err := job.CreateBuild()
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

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[3].ID(), Limit: 2}))

				builds, pagination, err = team.PrivateAndPublicBuilds(*pagination.Next)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(2))

				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID(), Limit: 2}))

				builds, pagination, err = team.PrivateAndPublicBuilds(*pagination.Next)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0]).To(Equal(allBuilds[0]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[0].ID(), Limit: 2}))
				Expect(pagination.Next).To(BeNil())

				builds, pagination, err = team.PrivateAndPublicBuilds(*pagination.Previous)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID(), Limit: 2}))
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

	Describe("Builds", func() {
		var (
			expectedBuilds []db.Build
			pipeline       db.Pipeline
		)

		BeforeEach(func() {
			oneOfAKind, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, oneOfAKind)

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
			pipeline, _, err = team.SavePipeline("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			thirdBuild, err := someOtherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, thirdBuild)
		})

		It("returns builds for the current team", func() {
			builds, _, err := team.Builds(db.Page{Limit: 10})
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
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

	Describe("SavePipeline", func() {
		type SerialGroup struct {
			JobID int
			Name  string
		}

		var (
			config       atc.Config
			otherConfig  atc.Config
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
						Name: "some-job",

						Public: true,

						Serial:       true,
						SerialGroups: []string{"serial-group-1", "serial-group-2"},

						Plan: atc.PlanSequence{
							{
								Get:      "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
							{
								Task:           "some-task",
								Privileged:     true,
								TaskConfigPath: "some/config/path.yml",
								TaskConfig: &atc.TaskConfig{
									RootfsURI: "some-image",
								},
							},
							{
								Put: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
						},
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
		})

		It("returns true for created", func() {
			_, created, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		It("caches the team id", func() {
			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.TeamID()).To(Equal(team.ID()))
		})

		It("can be saved as paused", func() {
			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelinePaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused()).To(BeTrue())
		})

		It("can be saved as unpaused", func() {
			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused()).To(BeFalse())
		})

		It("defaults to paused", func() {
			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused()).To(BeTrue())
		})

		It("creates all of the resources from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := savedPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{
				"source-config": "some-value",
			}))
		})

		It("updates resource config", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			config.Resources[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			savedPipeline, _, err := team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := savedPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{
				"source-other-config": "some-other-value",
			}))
		})

		It("marks resource as inactive if it is no longer in config", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			config.Resources = []atc.ResourceConfig{}

			savedPipeline, _, err := team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the resource types from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
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
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			config.ResourceTypes[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			savedPipeline, _, err := team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
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
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			config.ResourceTypes = []atc.ResourceType{}

			savedPipeline, _, err := team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.ResourceType("some-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the jobs from the pipeline in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := savedPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.Config()).To(Equal(config.Jobs[0]))
		})

		It("updates job config", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			config.Jobs[0].Public = false

			_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.Config().Public).To(BeFalse())
		})

		It("marks job inactive when it is no longer in pipeline", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			config.Jobs = []atc.JobConfig{}

			savedPipeline, _, err := team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := savedPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("removes worker task caches for jobs that are no longer in pipeline", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = workerTaskCacheFactory.FindOrCreate(job.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())

			_, found, err = workerTaskCacheFactory.Find(job.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			config.Jobs = []atc.JobConfig{}

			_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = workerTaskCacheFactory.Find(job.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("removes worker task caches for tasks that are no longer exist", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = workerTaskCacheFactory.FindOrCreate(job.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())

			_, found, err = workerTaskCacheFactory.Find(job.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			config.Jobs = []atc.JobConfig{
				{
					Name: "some-job",
					Plan: atc.PlanSequence{
						{
							Task:           "some-other-task",
							TaskConfigPath: "some/config/path.yml",
						},
					},
				},
			}

			_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = workerTaskCacheFactory.Find(job.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the serial groups from the jobs in the database", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
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
			savedPipeline, _, err := team.SavePipeline(pipelineName, otherConfig, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := savedPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Tags()).To(Equal([]string{"some-group"}))
		})

		It("updates tags in the jobs table", func() {
			savedPipeline, _, err := team.SavePipeline(pipelineName, otherConfig, 0, db.PipelineNoChange)
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
					Jobs: []string{"some-other-job"},
				},
			}

			savedPipeline, _, err = team.SavePipeline(pipelineName, otherConfig, savedPipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = savedPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Tags()).To(ConsistOf([]string{"some-another-group", "some-other-group"}))
		})

		It("it returns created as false when updated", func() {
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())

			_, created, err := team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeFalse())
		})

		It("updating from paused to unpaused", func() {
			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelinePaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused()).To(BeTrue())

			_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err = team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused()).To(BeFalse())
		})

		It("updating from unpaused to paused", func() {
			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused()).To(BeFalse())

			_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelinePaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err = team.Pipeline(pipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused()).To(BeTrue())
		})

		Context("updating with no change", func() {
			It("maintains paused if the pipeline is paused", func() {
				_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelinePaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err := team.Pipeline(pipelineName)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeTrue())

				_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err = team.Pipeline(pipelineName)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeTrue())
			})

			It("maintains unpaused if the pipeline is unpaused", func() {
				_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err := team.Pipeline(pipelineName)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeFalse())

				_, _, err = team.SavePipeline(pipelineName, config, pipeline.ConfigVersion(), db.PipelineNoChange)
				Expect(err).ToNot(HaveOccurred())

				pipeline, found, err = team.Pipeline(pipelineName)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused()).To(BeFalse())
			})
		})

		It("can lookup a pipeline by name", func() {
			pipelineName := "a-pipeline-name"
			otherPipelineName := "an-other-pipeline-name"

			_, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			_, _, err = team.SavePipeline(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			pipeline, found, err := team.Pipeline(pipelineName)
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
			expectConfigsEqual(atc.Config{
				Groups:        pipeline.Groups(),
				Resources:     resources.Configs(),
				ResourceTypes: resourceTypes.Configs(),
				Jobs:          jobs.Configs(),
			}, config)

			otherPipeline, found, err := team.Pipeline(otherPipelineName)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(otherPipeline.Name()).To(Equal(otherPipelineName))
			Expect(otherPipeline.ID()).ToNot(Equal(0))
			otherResourceTypes, err := otherPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			otherResources, err := otherPipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			otherJobs, err := otherPipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        otherPipeline.Groups(),
				Resources:     otherResources.Configs(),
				ResourceTypes: otherResourceTypes.Configs(),
				Jobs:          otherJobs.Configs(),
			}, otherConfig)

		})

		It("can manage multiple pipeline configurations", func() {
			pipelineName := "a-pipeline-name"
			otherPipelineName := "an-other-pipeline-name"

			By("being able to save the config")
			pipeline, _, err := team.SavePipeline(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline, _, err := team.SavePipeline(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			By("returning the saved config to later gets")
			resourceTypes, err := pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			resources, err := pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			jobs, err := pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        pipeline.Groups(),
				Resources:     resources.Configs(),
				ResourceTypes: resourceTypes.Configs(),
				Jobs:          jobs.Configs(),
			}, config)

			otherResourceTypes, err := otherPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			otherResources, err := otherPipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			otherJobs, err := otherPipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        otherPipeline.Groups(),
				Resources:     otherResources.Configs(),
				ResourceTypes: otherResourceTypes.Configs(),
				Jobs:          otherJobs.Configs(),
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
				Plan: atc.PlanSequence{
					{
						Get:      "new-input",
						Resource: "new-resource",
						Params: atc.Params{
							"new-param": "new-value",
						},
					},
					{
						Task:           "some-task",
						TaskConfigPath: "new/config/path.yml",
					},
				},
			})

			By("not allowing non-sequential updates")
			_, _, err = team.SavePipeline(pipelineName, updatedConfig, pipeline.ConfigVersion()-1, db.PipelineUnpaused)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			_, _, err = team.SavePipeline(pipelineName, updatedConfig, pipeline.ConfigVersion()+10, db.PipelineUnpaused)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			_, _, err = team.SavePipeline(otherPipelineName, updatedConfig, otherPipeline.ConfigVersion()-1, db.PipelineUnpaused)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			_, _, err = team.SavePipeline(otherPipelineName, updatedConfig, otherPipeline.ConfigVersion()+10, db.PipelineUnpaused)
			Expect(err).To(Equal(db.ErrConfigComparisonFailed))

			By("being able to update the config with a valid con")
			pipeline, _, err = team.SavePipeline(pipelineName, updatedConfig, pipeline.ConfigVersion(), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			otherPipeline, _, err = team.SavePipeline(otherPipelineName, updatedConfig, otherPipeline.ConfigVersion(), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			By("returning the updated config")
			resourceTypes, err = pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			resources, err = pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			jobs, err = pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        pipeline.Groups(),
				Resources:     resources.Configs(),
				ResourceTypes: resourceTypes.Configs(),
				Jobs:          jobs.Configs(),
			}, updatedConfig)

			otherResourceTypes, err = otherPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
			otherResources, err = otherPipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
			otherJobs, err = otherPipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
			expectConfigsEqual(atc.Config{
				Groups:        otherPipeline.Groups(),
				Resources:     otherResources.Configs(),
				ResourceTypes: otherResourceTypes.Configs(),
				Jobs:          otherJobs.Configs(),
			}, updatedConfig)

			By("returning the saved groups")
			returnedGroups = pipeline.Groups()
			Expect(returnedGroups).To(Equal(updatedConfig.Groups))

			otherReturnedGroups = otherPipeline.Groups()
			Expect(otherReturnedGroups).To(Equal(updatedConfig.Groups))
		})

		Context("when there are multiple teams", func() {
			It("can allow pipelines with the same name across teams", func() {
				teamPipeline, _, err := team.SavePipeline("steve", config, 0, db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				By("allowing you to save a pipeline with the same name in another team")
				otherTeamPipeline, _, err := otherTeam.SavePipeline("steve", otherConfig, 0, db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				By("updating the pipeline config for the correct team's pipeline")
				teamPipeline, _, err = team.SavePipeline("steve", otherConfig, teamPipeline.ConfigVersion(), db.PipelineNoChange)
				Expect(err).ToNot(HaveOccurred())

				_, _, err = otherTeam.SavePipeline("steve", config, otherTeamPipeline.ConfigVersion(), db.PipelineNoChange)
				Expect(err).ToNot(HaveOccurred())

				By("pausing the correct team's pipeline")
				_, _, err = team.SavePipeline("steve", otherConfig, teamPipeline.ConfigVersion(), db.PipelinePaused)
				Expect(err).ToNot(HaveOccurred())

				pausedPipeline, found, err := team.Pipeline("steve")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				unpausedPipeline, found, err := otherTeam.Pipeline("steve")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(pausedPipeline.Paused()).To(BeTrue())
				Expect(unpausedPipeline.Paused()).To(BeFalse())

				By("cannot cross update configs")
				_, _, err = team.SavePipeline("steve", otherConfig, otherTeamPipeline.ConfigVersion(), db.PipelineNoChange)
				Expect(err).To(HaveOccurred())

				_, _, err = team.SavePipeline("steve", otherConfig, otherTeamPipeline.ConfigVersion(), db.PipelinePaused)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("FindContainerOnWorker/CreateContainer", func() {
		var (
			containerMetadata db.ContainerMetadata
			team              db.Team
			fakeOwner         *dbfakes.FakeContainerOwner
			build             db.Build

			foundCreatingContainer db.CreatingContainer
			foundCreatedContainer  db.CreatedContainer
		)

		expiries := db.ContainerOwnerExpiries{
			GraceTime: 2 * time.Minute,
			Min:       5 * time.Minute,
			Max:       1 * time.Hour,
		}

		BeforeEach(func() {
			containerMetadata = db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}

			var err error
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			fakeOwner = new(dbfakes.FakeContainerOwner)
			fakeOwner.FindReturns(sq.Eq{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
			}, true, nil)
			fakeOwner.CreateReturns(map[string]interface{}{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
			}, nil)

			team = defaultTeam
			_, err = team.SaveWorker(atc.Worker{
				Name:      "fake-worker",
				Team:      "default-team",
				StartTime: 1501703719,
				ResourceTypes: []atc.WorkerResourceType{
					{
						Type:       "fake-resource-type",
						Image:      "fake-image",
						Version:    "fake-version",
						Privileged: false,
					},
				},
			}, 1*time.Hour)
			Expect(err).ToNot(HaveOccurred())

			resourceConfigCheckSession, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
				logger,
				defaultResource.Type(),
				defaultResource.Source(),
				creds.VersionedResourceTypes{},
				expiries,
			)
			Expect(err).ToNot(HaveOccurred())

			_ = db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, team.ID())
		})

		JustBeforeEach(func() {
			var err error
			foundCreatingContainer, foundCreatedContainer, err = team.FindContainerOnWorker(defaultWorker.Name(), fakeOwner)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			var creatingContainer db.CreatingContainer

			BeforeEach(func() {
				var err error
				creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), fakeOwner, containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				Expect(foundCreatedContainer).To(BeNil())
				Expect(foundCreatingContainer).ToNot(BeNil())
			})

			Context("when finding on another team", func() {
				BeforeEach(func() {
					team = otherTeam
				})

				It("does not find it", func() {
					Expect(foundCreatingContainer).To(BeNil())
					Expect(foundCreatedContainer).To(BeNil())
				})
			})

			Context("when there is a created container", func() {
				BeforeEach(func() {
					_, err := creatingContainer.Created()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					Expect(foundCreatedContainer).ToNot(BeNil())
					Expect(foundCreatingContainer).To(BeNil())
				})

				Context("when finding on another team", func() {
					BeforeEach(func() {
						team = otherTeam
					})

					It("does not find it", func() {
						Expect(foundCreatingContainer).To(BeNil())
						Expect(foundCreatedContainer).To(BeNil())
					})
				})
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				Expect(foundCreatedContainer).To(BeNil())
				Expect(foundCreatingContainer).To(BeNil())
			})
		})
	})
})
