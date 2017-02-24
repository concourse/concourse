package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = XDescribe("TeamDbContainers", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		database      db.DB
		teamDBFactory db.TeamDBFactory

		savedPipeline      db.SavedPipeline
		savedOtherPipeline db.SavedPipeline

		teamID      int
		otherTeamID int
		teamDB      db.TeamDB
		pipelineDB  db.PipelineDB
		build       db.Build
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(lockfakes.FakeConnector)
		retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := lock.NewLockFactory(retryableConn)
		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		database = db.NewSQL(dbConn, bus, lockFactory)

		team := db.Team{Name: "team-name"}
		savedTeam, err := database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())
		teamID = savedTeam.ID

		otherTeam := db.Team{Name: "other-team-name"}
		savedOtherTeam, err := database.CreateTeam(otherTeam)
		Expect(err).NotTo(HaveOccurred())
		otherTeamID = savedOtherTeam.ID

		teamDB = teamDBFactory.GetTeamDB("team-name")

		build, err = teamDB.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveWorker(db.WorkerInfo{
			Name:       "some-worker",
			GardenAddr: "1.2.3.4:7777",
		}, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveWorker(db.WorkerInfo{
			Name:       "some-other-worker",
			GardenAddr: "1.2.3.5:7777",
		}, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		config := atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
				{
					Name: "some-other-job",
				},
				{
					Name: "some-random-job",
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
				},
				{
					Name: "some-other-resource",
					Type: "some-other-type",
				},
			},
			ResourceTypes: atc.ResourceTypes{
				{
					Name: "some-custom-type",
					Type: "git",
				},
			},
		}

		savedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("some-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		savedOtherPipeline, _, err = teamDB.SaveConfigToBeDeprecated("some-other-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
	})

	type findContainersByDescriptorsExample struct {
		containersToCreate     []db.Container
		descriptorsToFilterFor db.Container
		expectedHandles        []string
	}

	getResourceID := func(name string) int {
		savedResource, _, err := pipelineDB.GetResource(name)
		Expect(err).NotTo(HaveOccurred())
		return savedResource.ID
	}

	getJobBuildID := func(jobName string) int {
		savedBuild, err := pipelineDB.CreateJobBuild(jobName)
		Expect(err).NotTo(HaveOccurred())
		return savedBuild.ID()
	}

	getOneOffBuildID := func() int {
		savedBuild, err := teamDB.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		return savedBuild.ID()
	}

	DescribeTable("filtering containers by descriptors",
		func(exampleGenerator func() findContainersByDescriptorsExample) {
			var results []db.SavedContainer
			var handles []string
			var err error

			example := exampleGenerator()

			for _, containerToCreate := range example.containersToCreate {
				if containerToCreate.Type.String() == "" {
					containerToCreate.Type = db.ContainerTypeTask
				}

				_, err := database.CreateContainerToBeRemoved(containerToCreate, time.Duration(0), []string{})
				Expect(err).NotTo(HaveOccurred())
			}

			results, err = teamDB.FindContainersByDescriptors(example.descriptorsToFilterFor)
			Expect(err).NotTo(HaveOccurred())

			for _, result := range results {
				handles = append(handles, result.Handle)
			}

			Expect(handles).To(ConsistOf(example.expectedHandles))

			for _, containerToDelete := range example.containersToCreate {
				err = database.DeleteContainer(containerToDelete.Handle)
				Expect(err).NotTo(HaveOccurred())
			}
		},

		Entry("returns all containers belonging to the team when no filters are passed", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "a",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: 0,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "b",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "c",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     otherTeamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("does not return things that the filter doesn't match", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "a",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "b",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedOtherPipeline.ID,
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{ResourceName: "some-resource"}},
				expectedHandles:        nil,
			}
		}),

		Entry("returns containers where the step name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "a",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							StepName:   "some-step",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "b",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedOtherPipeline.ID,
							StepName:   "some-other-step",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "c",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedOtherPipeline.ID,
							StepName:   "some-step",
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{StepName: "some-step"}},
				expectedHandles:        []string{"a", "c"},
			}
		}),

		Entry("returns containers where the resource name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceID:  getResourceID("some-resource"),
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineID:   savedPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceID:  getResourceID("some-resource"),
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineID:   savedOtherPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceID:  getResourceID("some-other-resource"),
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineID:   savedOtherPipeline.ID,
							ResourceName: "some-other-resource",
							TeamID:       teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{ResourceName: "some-resource"}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("returns containers where the pipeline matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "a",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "b",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedOtherPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "c",
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{PipelineName: "some-pipeline"}},
				expectedHandles:        []string{"a", "c"},
			}
		}),

		Entry("returns containers where the type matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "a",
							Type:       db.ContainerTypePut,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "b",
							Type:       db.ContainerTypePut,
							WorkerName: "some-other-worker",
							PipelineID: savedOtherPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "c",
							Type:       db.ContainerTypeGet,
							WorkerName: "some-other-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{Type: db.ContainerTypePut}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("returns containers where the worker name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "a",
							Type:       db.ContainerTypePut,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "b",
							Type:       db.ContainerTypePut,
							WorkerName: "some-worker",
							PipelineID: savedOtherPipeline.ID,
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "c",
							Type:       db.ContainerTypeGet,
							WorkerName: "some-other-worker",
							PipelineID: savedPipeline.ID,
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{WorkerName: "some-worker"}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("returns containers where the check type matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
							ResourceID:  1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineID:   savedPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:       db.ContainerStageRun,
							CheckType:   "nope",
							CheckSource: atc.Source{"some": "source"},
							ResourceID:  1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineID:   savedOtherPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:       db.ContainerStageRun,
							CheckType:   "some-type",
							CheckSource: atc.Source{"some": "source"},
							ResourceID:  1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineID:   savedPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerIdentifier: db.ContainerIdentifier{CheckType: "some-type"}},
				expectedHandles:        []string{"c"},
			}
		}),

		Entry("returns containers where the check source matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage: db.ContainerStageRun,
							CheckSource: atc.Source{
								"some": "other-source",
							},
							CheckType:  "git",
							ResourceID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineID:   savedPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineID:   savedOtherPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage: db.ContainerStageRun,
							CheckSource: atc.Source{
								"some": "source",
							},
							CheckType:  "git",
							ResourceID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineID:   savedPipeline.ID,
							ResourceName: "some-resource",
							TeamID:       teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}}},
				expectedHandles:        []string{"c"},
			}
		}),

		Entry("returns containers where the job name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{{
					ContainerIdentifier: db.ContainerIdentifier{
						Stage:   db.ContainerStageRun,
						BuildID: getJobBuildID("some-other-job"),
						PlanID:  "plan-id",
					},
					ContainerMetadata: db.ContainerMetadata{
						Type:       db.ContainerTypeTask,
						WorkerName: "some-worker",
						PipelineID: savedPipeline.ID,
						JobName:    "some-other-job",
						Handle:     "a",
						TeamID:     teamID,
					},
				},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: getJobBuildID("some-job"),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							JobName:    "some-job",
							Handle:     "b",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: getOneOffBuildID(),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: 0,
							JobName:    "",
							Handle:     "c",
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{JobName: "some-job"}},
				expectedHandles:        []string{"b"},
			}
		}),

		Entry("returns containers where the build ID matches", func() findContainersByDescriptorsExample {
			someBuildID := getJobBuildID("some-job")
			someOtherBuildID := getJobBuildID("some-job")
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: someBuildID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							JobName:    "some-job",
							Handle:     "a",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: someOtherBuildID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							JobName:    "some-other-job",
							Handle:     "b",
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerIdentifier: db.ContainerIdentifier{BuildID: someBuildID}},
				expectedHandles:        []string{"a"},
			}
		}),

		Entry("returns containers where the build name matches", func() findContainersByDescriptorsExample {
			savedBuild1, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			savedBuild2, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			savedBuild3, err := pipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: savedBuild1.ID(),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							JobName:    "some-job",
							Handle:     "a",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: savedBuild2.ID(),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							JobName:    "some-job",
							BuildName:  savedBuild2.Name(),
							Handle:     "b",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: savedBuild3.ID(),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							JobName:    "some-other-job",
							// purposefully re-use the original build name to test that it
							// can return multiple containers
							BuildName: savedBuild1.Name(),
							Handle:    "c",
							TeamID:    teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{
					ContainerMetadata: db.ContainerMetadata{
						BuildName: savedBuild1.Name(),
						TeamID:    teamID,
					},
				},
				expectedHandles: []string{"a", "c"},
			}
		}),

		Entry("returns containers where the attempts numbers match", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{{
					ContainerIdentifier: db.ContainerIdentifier{
						Stage:   db.ContainerStageRun,
						PlanID:  "plan-id",
						BuildID: build.ID(),
					},
					ContainerMetadata: db.ContainerMetadata{
						Type:       db.ContainerTypeTask,
						WorkerName: "some-worker",
						PipelineID: savedPipeline.ID,
						Attempts:   []int{1, 2, 5},
						Handle:     "a",
						TeamID:     teamID,
					},
				},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							Attempts:   []int{1, 2},
							Handle:     "b",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:       db.ContainerTypeTask,
							WorkerName: "some-other-worker",
							PipelineID: savedPipeline.ID,
							Attempts:   []int{1},
							Handle:     "c",
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{Attempts: []int{1, 2}}},
				expectedHandles:        []string{"b"},
			}
		}),

		Entry("returns containers where all fields match", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							StepName:   "some-name",
							PipelineID: savedPipeline.ID,
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							Handle:     "a",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							StepName:   "WROONG",
							PipelineID: savedPipeline.ID,
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							Handle:     "b",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							StepName:   "some-name",
							PipelineID: savedPipeline.ID,
							Type:       db.ContainerTypeTask,
							WorkerName: "some-worker",
							Handle:     "c",
							TeamID:     teamID,
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: build.ID(),
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							PipelineID: savedPipeline.ID,
							Type:       db.ContainerTypeTask,
							Handle:     "d",
							TeamID:     teamID,
						},
					},
				},
				descriptorsToFilterFor: db.Container{
					ContainerMetadata: db.ContainerMetadata{
						StepName:   "some-name",
						PipelineID: savedPipeline.ID,
						Type:       db.ContainerTypeTask,
						WorkerName: "some-worker",
						TeamID:     teamID,
					},
				},
				expectedHandles: []string{"a", "c"},
			}
		}),
	)

	Describe("GetContainer", func() {
		var savedContainer db.SavedContainer
		var otherSavedContainer db.SavedContainer
		var resourceTypeVersion atc.Version
		var container db.Container

		BeforeEach(func() {
			resourceTypeVersion = atc.Version{
				"some-resource-type": "some-version",
			}

			container = db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					ResourceID:          getResourceID("some-resource"),
					CheckType:           "some-resource-type",
					CheckSource:         atc.Source{"some": "source"},
					ResourceTypeVersion: resourceTypeVersion,
					Stage:               db.ContainerStageRun,
				},
				ContainerMetadata: db.ContainerMetadata{
					Handle:               "some-handle",
					WorkerName:           "some-worker",
					PipelineID:           savedPipeline.ID,
					Type:                 db.ContainerTypeCheck,
					WorkingDirectory:     "tmp/build/some-guid",
					EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
					TeamID:               teamID,
				},
			}

			otherTeamContainer := db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					Stage:   db.ContainerStageRun,
					PlanID:  "plan-id",
					BuildID: build.ID(),
				},
				ContainerMetadata: db.ContainerMetadata{
					Handle:     "b",
					Type:       db.ContainerTypeTask,
					WorkerName: "some-worker",
					PipelineID: 0,
					TeamID:     otherTeamID,
				},
			}
			var err error
			savedContainer, err = database.CreateContainerToBeRemoved(container, 0, []string{})
			Expect(err).NotTo(HaveOccurred())

			otherSavedContainer, err = database.CreateContainerToBeRemoved(otherTeamContainer, 0, []string{})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container belongs to the team", func() {
			It("returns the container", func() {
				actualContainer, found, err := teamDB.GetContainer(savedContainer.Handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(actualContainer.Handle).To(Equal(savedContainer.Handle))
				Expect(actualContainer.WorkerName).To(Equal(container.WorkerName))
				Expect(actualContainer.ResourceID).To(Equal(container.ResourceID))

				Expect(actualContainer.Handle).To(Equal(container.Handle))
				Expect(actualContainer.StepName).To(Equal(""))
				Expect(actualContainer.ResourceName).To(Equal("some-resource"))
				Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
				Expect(actualContainer.PipelineName).To(Equal(savedPipeline.Name))
				Expect(actualContainer.BuildID).To(Equal(0))
				Expect(actualContainer.BuildName).To(Equal(""))
				Expect(actualContainer.Type).To(Equal(db.ContainerTypeCheck))
				Expect(actualContainer.ContainerMetadata.WorkerName).To(Equal(container.WorkerName))
				Expect(actualContainer.WorkingDirectory).To(Equal(container.WorkingDirectory))
				Expect(actualContainer.CheckType).To(Equal(container.CheckType))
				Expect(actualContainer.CheckSource).To(Equal(container.CheckSource))
				Expect(actualContainer.EnvironmentVariables).To(Equal(container.EnvironmentVariables))
				Expect(actualContainer.ResourceTypeVersion).To(Equal(resourceTypeVersion))
				Expect(actualContainer.TeamID).To(Equal(teamID))
			})
		})

		Context("when the container belongs to another team", func() {
			It("does not return the container and returns and error", func() {
				_, found, err := teamDB.GetContainer(otherSavedContainer.Handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
