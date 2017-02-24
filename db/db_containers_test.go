package db_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
)

var _ = Describe("Keeping track of containers", func() {
	var (
		dbConn   db.Conn
		dbngConn dbng.Conn
		listener *pq.Listener

		database      *db.SQLDB
		teamDB        db.TeamDB
		savedPipeline db.SavedPipeline
		pipelineDB    db.PipelineDB
		teamID        int
		build         db.Build

		dbngWorkerFactory         dbng.WorkerFactory
		dbngResourceConfigFactory dbng.ResourceConfigFactory
		dbngTeam                  dbng.Team
		dbngWorker                *dbng.Worker
		dbngResourceCacheFactory  dbng.ResourceCacheFactory

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error

		postgresRunner.Truncate()

		pqConn := postgresRunner.Open()
		dbConn = db.Wrap(pqConn)
		dbngConn = dbng.Wrap(pqConn)

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(lockfakes.FakeConnector)
		retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := lock.NewLockFactory(retryableConn)
		logger = lagertest.NewTestLogger("test")

		database = db.NewSQL(dbConn, bus, lockFactory)

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

		savedTeam, err := database.CreateTeam(db.Team{Name: "team-name"})
		Expect(err).NotTo(HaveOccurred())
		teamID = savedTeam.ID

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB = teamDBFactory.GetTeamDB("team-name")

		build, err = teamDB.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())

		savedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("some-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfigToBeDeprecated("some-other-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, lockFactory)
		pipelineDB = pipelineDBFactory.Build(savedPipeline)

		wf := dbng.NewWorkerFactory(dbngConn)
		_, err = wf.SaveWorker(atc.Worker{
			Name:       "some-worker",
			GardenAddr: "1.2.3.4:7777",
		}, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		workerInfo := atc.Worker{
			Name: "updated-resource-type-worker",
			ResourceTypes: []atc.WorkerResourceType{
				atc.WorkerResourceType{
					Type:    "some-type",
					Version: "some-version",
				},
			},
		}
		_, err = wf.SaveWorker(workerInfo, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		dbngWorkerFactory = dbng.NewWorkerFactory(dbngConn)
		dbngWorker, err = dbngWorkerFactory.SaveWorker(atc.Worker{
			Name:            "some-worker",
			GardenAddr:      "1.2.3.4:7777",
			BaggageclaimURL: "1.2.3.4:7788",
			ResourceTypes: []atc.WorkerResourceType{
				atc.WorkerResourceType{
					Type:    "some-type",
					Version: "some-version",
				},
				atc.WorkerResourceType{
					Type:    "some-new-type",
					Version: "some-new-version",
				},
			},
		}, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		dbngTeamFactory := dbng.NewTeamFactory(dbngConn, lockFactory)
		var found bool
		dbngTeam, found, err = dbngTeamFactory.FindTeam("team-name")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		dbngResourceConfigFactory = dbng.NewResourceConfigFactory(dbngConn, lockFactory)
		dbngResourceCacheFactory = dbng.NewResourceCacheFactory(dbngConn, lockFactory)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	getResourceID := func(name string) int {
		savedResource, _, err := pipelineDB.GetResource(name)
		Expect(err).NotTo(HaveOccurred())
		return savedResource.ID
	}

	It("can find non-one-off containers from unsuccessful builds", func() {
		savedBuild0, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		savedBuild1, err := pipelineDB.CreateJobBuild("some-other-job")
		Expect(err).NotTo(HaveOccurred())

		savedBuild2, err := teamDB.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())

		savedBuild3, err := pipelineDB.CreateJobBuild("some-random-job")
		Expect(err).NotTo(HaveOccurred())

		savedBuild4, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		err = savedBuild0.Finish(db.StatusErrored)
		Expect(err).NotTo(HaveOccurred())
		err = savedBuild1.Finish(db.StatusFailed)
		Expect(err).NotTo(HaveOccurred())
		err = savedBuild2.Finish(db.StatusFailed)
		Expect(err).NotTo(HaveOccurred())
		err = savedBuild3.Finish(db.StatusSucceeded)
		Expect(err).NotTo(HaveOccurred())
		err = savedBuild4.Finish(db.StatusAborted)
		Expect(err).NotTo(HaveOccurred())

		containerInfo0 := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild0.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "handle-0",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				JobName:    savedBuild0.JobName(),
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		containerInfo1 := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild1.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "handle-1",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				JobName:    savedBuild1.JobName(),
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		containerInfo2 := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild2.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "handle-2",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		containerInfo3 := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild3.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "handle-3",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				JobName:    savedBuild3.JobName(),
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		containerInfo4 := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild4.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "handle-4",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				JobName:    savedBuild4.JobName(),
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		_, err = database.CreateContainerToBeRemoved(containerInfo0, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainerToBeRemoved(containerInfo1, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainerToBeRemoved(containerInfo2, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainerToBeRemoved(containerInfo3, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainerToBeRemoved(containerInfo4, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		savedContainers, err := database.FindJobContainersFromUnsuccessfulBuilds()
		Expect(err).NotTo(HaveOccurred())

		Expect(savedContainers).To(HaveLen(2))
		handle0 := savedContainers[0].Handle
		handle1 := savedContainers[1].Handle
		Expect([]string{handle0, handle1}).To(ConsistOf("handle-0", "handle-1"))
	})

	It("can create and get a resource container object", func() {
		resourceTypeVersion := atc.Version{
			"some-resource-type": "some-version",
		}

		containerToCreate := db.Container{
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

		By("creating a container")
		_, err := database.CreateContainerToBeRemoved(containerToCreate, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		matchingHandleContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage: db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle: "some-handle",
				TeamID: teamID,
			},
		}
		_, err = database.CreateContainerToBeRemoved(matchingHandleContainer, time.Duration(0), []string{})
		Expect(err).To(HaveOccurred())

		By("getting the saved info object by handle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(containerToCreate.ResourceID))

		Expect(actualContainer.Handle).To(Equal(containerToCreate.Handle))
		Expect(actualContainer.StepName).To(Equal(""))
		Expect(actualContainer.ResourceName).To(Equal("some-resource"))
		Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
		Expect(actualContainer.PipelineName).To(Equal(savedPipeline.Name))
		Expect(actualContainer.BuildID).To(Equal(0))
		Expect(actualContainer.BuildName).To(Equal(""))
		Expect(actualContainer.Type).To(Equal(db.ContainerTypeCheck))
		Expect(actualContainer.ContainerMetadata.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.WorkingDirectory).To(Equal(containerToCreate.WorkingDirectory))
		Expect(actualContainer.CheckType).To(Equal(containerToCreate.CheckType))
		Expect(actualContainer.CheckSource).To(Equal(containerToCreate.CheckSource))
		Expect(actualContainer.EnvironmentVariables).To(Equal(containerToCreate.EnvironmentVariables))
		Expect(actualContainer.ResourceTypeVersion).To(Equal(resourceTypeVersion))
		Expect(actualContainer.TeamID).To(Equal(teamID))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can create and get a step container info object", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: build.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:               "some-handle",
				WorkerName:           "some-worker",
				PipelineID:           savedPipeline.ID,
				StepName:             "some-step-container",
				Type:                 db.ContainerTypeTask,
				WorkingDirectory:     "tmp/build/some-guid",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
				User:                 "test-user",
				Attempts:             []int{1, 2, 4},
				TeamID:               teamID,
			},
		}

		By("creating a container")
		_, err := database.CreateContainerToBeRemoved(containerToCreate, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		duplicateHandleContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: build.ID(),
				PlanID:  "some-other-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "some-handle",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				Type:       db.ContainerTypeTask,
			},
		}
		_, err = database.CreateContainerToBeRemoved(duplicateHandleContainer, time.Duration(0), []string{})
		Expect(err).To(HaveOccurred())

		By("trying to create a container with an insufficient step identifier")
		insufficientStepContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: build.ID(),
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "some-handle-2",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}
		_, err = database.CreateContainerToBeRemoved(insufficientStepContainer, time.Duration(0), []string{})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))

		By("trying to create a container with an insufficient check identifier")
		insufficientCheckContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				ResourceID: 72,
				CheckType:  "git",
				Stage:      db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "some-handle-3",
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}
		_, err = database.CreateContainerToBeRemoved(insufficientCheckContainer, time.Duration(0), []string{})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))

		By("getting the saved info object by handle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.BuildID).To(Equal(containerToCreate.BuildID))
		Expect(actualContainer.PlanID).To(Equal(containerToCreate.PlanID))

		Expect(actualContainer.Handle).To(Equal(containerToCreate.Handle))
		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
		Expect(actualContainer.PipelineName).To(Equal(savedPipeline.Name))
		Expect(actualContainer.StepName).To(Equal(containerToCreate.StepName))
		Expect(actualContainer.BuildName).To(Equal(build.Name()))
		Expect(actualContainer.Type).To(Equal(containerToCreate.Type))
		Expect(actualContainer.WorkingDirectory).To(Equal(containerToCreate.WorkingDirectory))
		Expect(actualContainer.EnvironmentVariables).To(Equal(containerToCreate.EnvironmentVariables))
		Expect(actualContainer.User).To(Equal(containerToCreate.User))
		Expect(actualContainer.Attempts).To(Equal(containerToCreate.Attempts))

		Expect(actualContainer.ResourceID).To(Equal(0))
		Expect(actualContainer.ResourceName).To(Equal(""))
		Expect(actualContainer.CheckType).To(BeEmpty())
		Expect(actualContainer.CheckSource).To(BeEmpty())
		Expect(actualContainer.TeamID).To(Equal(teamID))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can populate metadata that was omitted when creating the container", func() {
		savedBuild, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild.ID(),
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:               "some-handle",
				WorkerName:           "some-worker",
				PipelineID:           savedPipeline.ID,
				StepName:             "some-step-container",
				Type:                 db.ContainerTypeTask,
				WorkingDirectory:     "tmp/build/some-guid",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
				Attempts:             []int{1, 2, 4},
				TeamID:               teamID,
			},
		}

		By("creating a container with optional metadata fields omitted")
		_, err = database.CreateContainerToBeRemoved(containerToCreate, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())

		By("populating those fields when retrieving the container")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.BuildName).To(Equal(savedBuild.Name()))
		Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
		Expect(actualContainer.PipelineName).To(Equal(savedPipeline.Name))
		Expect(actualContainer.JobName).To(Equal("some-job"))
		Expect(actualContainer.User).To(Equal("root"))
		Expect(actualContainer.TeamID).To(Equal(teamID))
	})

	It("can reap a container", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  "to-be-reaped-plan",
				BuildID: build.ID(),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "some-reaped-handle",
				Type:       db.ContainerTypeTask,
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				TeamID:     teamID,
			},
		}
		_, err := database.CreateContainerToBeRemoved(containerToCreate, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())

		_, found, err := database.GetContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		By("reaping an existing container")
		err = database.ReapContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())

		_, found, err = database.GetContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("not failing if the container's already been reaped")
		err = database.ReapContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())
	})

	It("differentiates between a single step's containers with different stages", func() {
		someBuild, err := teamDB.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())

		checkStageAContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID(),
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-a",
			Stage:               db.ContainerStageCheck,
		}

		getStageAContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID(),
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-a",
			Stage:               db.ContainerStageGet,
		}

		checkStageBContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID(),
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-b",
			Stage:               db.ContainerStageCheck,
		}

		getStageBContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID(),
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-b",
			Stage:               db.ContainerStageGet,
		}

		runStageContainerID := db.ContainerIdentifier{
			BuildID: someBuild.ID(),
			PlanID:  atc.PlanID("some-task"),
			Stage:   db.ContainerStageRun,
		}

		checkContainerA, err := database.CreateContainerToBeRemoved(db.Container{
			ContainerIdentifier: checkStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "check-a-handle",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		getContainerA, err := database.CreateContainerToBeRemoved(db.Container{
			ContainerIdentifier: getStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "get-a-handle",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeGet,
				TeamID:     teamID,
			},
		}, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		checkContainerB, err := database.CreateContainerToBeRemoved(db.Container{
			ContainerIdentifier: checkStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "check-b-handle",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		getContainerB, err := database.CreateContainerToBeRemoved(db.Container{
			ContainerIdentifier: getStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "get-b-handle",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeGet,
				TeamID:     teamID,
			},
		}, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		runContainer, err := database.CreateContainerToBeRemoved(db.Container{
			ContainerIdentifier: runStageContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "run-handle",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		container, found, err := database.FindContainerByIdentifier(checkStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(checkStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(runStageContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(runContainer.ContainerIdentifier))
	})

	It("differentiates between a single resource's checking containers with different stages", func() {
		checkStageAContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-a",
			Stage:               db.ContainerStageCheck,
		}

		resourceConfig, err := dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some": "source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err := dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		checkContainerA, err := database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: checkStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		getStageAContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-a",
			Stage:               db.ContainerStageGet,
		}

		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err = creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		getContainerA, err := database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: getStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-worker",
				StepName:   "some-step",
				Type:       db.ContainerTypeGet,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		checkStageBContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-b",
			Stage:               db.ContainerStageCheck,
		}

		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err = creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		checkContainerB, err := database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: checkStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		getStageBContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-b",
			Stage:               db.ContainerStageGet,
		}

		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err = creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		getContainerB, err := database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: getStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-worker",
				StepName:   "some-step",
				Type:       db.ContainerTypeGet,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		runStageContainerID := db.ContainerIdentifier{
			ResourceID:  1,
			CheckSource: atc.Source{"some": "source"},
			CheckType:   "some-type",
			Stage:       db.ContainerStageRun,
		}

		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err = creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		runContainer, err := database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: runStageContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-worker",
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		container, found, err := database.FindContainerByIdentifier(checkStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(checkStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(runStageContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(runContainer.ContainerIdentifier))
	})

	It("picks one check container when there are several for different worker base resource types", func() {
		containerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-a",
			Stage:               db.ContainerStageCheck,
		}

		resourceConfig, err := dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some": "source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err := dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		_, err = database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: containerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		otherWorker, err := dbngWorkerFactory.SaveWorker(atc.Worker{
			Name:            "some-other-worker",
			GardenAddr:      "5.6.7.8:7777",
			BaggageclaimURL: "5.6.7.8:7788",
			ResourceTypes: []atc.WorkerResourceType{
				atc.WorkerResourceType{
					Type:    "some-type",
					Version: "some-updated-version",
				},
			},
		}, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some": "source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(otherWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		createdContainer, err = creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())

		checkContainerB, err := database.UpdateContainerTTLToBeRemoved(db.Container{
			ContainerIdentifier: containerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle:     createdContainer.Handle(),
				WorkerName: "some-other-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		container, found, err := database.FindContainerByIdentifier(containerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerB.ContainerIdentifier))
	})

	It("can find a single container info by identifier", func() {
		handle := "some-handle"
		otherHandle := "other-handle"

		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       handle,
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		resourceConfig, err := dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some": "other-source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())

		creatingContainer, err := dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		containerToCreateCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		containerToCreate.Handle = containerToCreateCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(containerToCreate, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		stepContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("plan-id"),
				BuildID: build.ID(),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     otherHandle,
				PipelineID: savedPipeline.ID,
				WorkerName: "some-worker",
				StepName:   "other-container",
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		creatingContainer, err = dbngTeam.CreateBuildContainer(dbngWorker, build.ID(), atc.PlanID("plan-id"), dbng.ContainerMetadata{
			Type: string(db.ContainerTypeTask),
			Name: "other-container",
		})
		Expect(err).NotTo(HaveOccurred())
		stepContainerToCreateCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		stepContainerToCreate.Handle = stepContainerToCreateCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(stepContainerToCreate, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		otherStepContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("other-plan-id"),
				BuildID: build.ID(),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "very-other-handle",
				PipelineID: savedPipeline.ID,
				WorkerName: "some-worker",
				StepName:   "other-container",
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		creatingContainer, err = dbngTeam.CreateBuildContainer(dbngWorker, build.ID(), atc.PlanID("other-plan-id"), dbng.ContainerMetadata{
			Type: string(db.ContainerTypeTask),
			Name: "other-container",
		})
		Expect(err).NotTo(HaveOccurred())
		otherStepContainerCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		otherStepContainer.Handle = otherStepContainerCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(otherStepContainer, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		resourceTypeContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:               db.ContainerStageRun,
				CheckType:           "some-type",
				CheckSource:         atc.Source{"some": "other-source"},
				ResourceTypeVersion: atc.Version{"foo": "bar"},
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineID: savedPipeline.ID,
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
				WorkerName: "some-worker",
			},
		}
		invalidCheckContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "other-source"},
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineID: savedPipeline.ID,
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
				WorkerName: "some-worker",
			},
		}
		invalidMetadataContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:               db.ContainerStageRun,
				CheckType:           "some-type",
				CheckSource:         atc.Source{"some": "other-source"},
				ResourceTypeVersion: atc.Version{"foo": "bar"},
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineName: "some-pipeline-name",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
				WorkerName:   "some-worker",
			},
		}

		allContainers := getAllContainers(dbConn)
		Expect(allContainers).To(HaveLen(3))

		By("returning a single matching resource container info")
		actualContainer, found, err := database.FindContainerByIdentifier(
			containerToCreate.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.Handle).To(Equal(containerToCreateCreated.Handle()))
		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(containerToCreate.ResourceID))
		Expect(actualContainer.TeamID).To(Equal(teamID))

		By("returning a single matching step container info")
		actualStepContainer, found, err := database.FindContainerByIdentifier(
			stepContainerToCreate.ContainerIdentifier,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(actualStepContainer.Handle).To(Equal(stepContainerToCreateCreated.Handle()))
		Expect(actualStepContainer.WorkerName).To(Equal(stepContainerToCreate.WorkerName))
		Expect(actualStepContainer.ResourceID).To(Equal(stepContainerToCreate.ResourceID))
		Expect(actualStepContainer.TeamID).To(Equal(teamID))

		By("returning a single matching resource type container info")
		actualResourceTypeContainer, found, err := database.FindContainerByIdentifier(
			resourceTypeContainerToCreate.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualResourceTypeContainer.ResourceTypeVersion).To(Equal(containerToCreate.ContainerIdentifier.ResourceTypeVersion))

		By("validating check container has either resource id or resource type version")
		_, err = database.CreateContainerToBeRemoved(invalidCheckContainerToCreate, time.Duration(0), []string{})
		Expect(err).To(HaveOccurred())

		By("validating pipeline container has pipeline ID")
		_, err = database.CreateContainerToBeRemoved(invalidMetadataContainerToCreate, time.Duration(0), []string{})
		Expect(err).To(HaveOccurred())

		By("differentiating check containers based on their check source")
		newSourceContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "new-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some": "new-source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		newSourceContainerToCreateCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		newSourceContainerToCreate.Handle = newSourceContainerToCreateCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(newSourceContainerToCreate, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		foundNewSourceContainer, found, err := database.FindContainerByIdentifier(newSourceContainerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundNewSourceContainer.Handle).To(Equal(newSourceContainerToCreate.Handle))

		foundOldSourceContainer, found, err := database.FindContainerByIdentifier(containerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundOldSourceContainer.Handle).To(Equal(containerToCreate.Handle))

		By("differentiating check containers based on their check type")
		newCheckTypeContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-new-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "new-check-type-handle",
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-new-type",
			atc.Source{"some": "new-source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		newCheckTypeContainerToCreateCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		newCheckTypeContainerToCreate.Handle = newCheckTypeContainerToCreateCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(newCheckTypeContainerToCreate, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		foundNewCheckTypeContainer, found, err := database.FindContainerByIdentifier(newCheckTypeContainerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundNewCheckTypeContainer.Handle).To(Equal(newCheckTypeContainerToCreate.Handle))

		foundOldCheckTypeContainer, found, err := database.FindContainerByIdentifier(containerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundOldCheckTypeContainer.Handle).To(Equal(containerToCreate.Handle))

		By("erroring if more than one container matches the filter")
		matchingContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some": "other-source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		matchingContainerToCreateCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		matchingContainerToCreate.Handle = matchingContainerToCreateCreated.Handle()
		createdMatchingContainer, err := database.UpdateContainerTTLToBeRemoved(matchingContainerToCreate, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		foundContainer, found, err := database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID:  createdMatchingContainer.ResourceID,
				CheckType:   createdMatchingContainer.CheckType,
				CheckSource: createdMatchingContainer.CheckSource,
				Stage:       createdMatchingContainer.Stage,
			})
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(db.ErrMultipleContainersFound))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				BuildID: createdMatchingContainer.BuildID,
			})
		Expect(err).To(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("still erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				PlanID: createdMatchingContainer.PlanID,
			})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("still erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID: createdMatchingContainer.ResourceID,
				CheckType:  createdMatchingContainer.CheckType,
			})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("still erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID:  createdMatchingContainer.ResourceID,
				CheckSource: createdMatchingContainer.CheckSource,
			})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("returning found of false if no containers match the filter")
		actualContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				BuildID: 404,
				PlanID:  atc.PlanID("plan-id"),
				Stage:   db.ContainerStageRun,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(actualContainer.Handle).To(BeEmpty())

		By("finding a check container has a custom resource type")
		customContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:               db.ContainerStageRun,
				CheckType:           "some-custom-type",
				CheckSource:         atc.Source{},
				ResourceTypeVersion: atc.Version{},
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "custom-handle",
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				PipelineID: savedPipeline.ID,
				TeamID:     teamID,
			},
		}

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-custom-type",
			atc.Source{"some": "other-source"},
			savedPipeline.ID,
			atc.ResourceTypes{
				atc.ResourceType{
					Name: "some-custom-type",
					Type: "some-type",
				},
			},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		customContainerCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		customContainer.Handle = customContainerCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(customContainer, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		foundContainer, found, err = database.FindContainerByIdentifier(customContainer.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundContainer.Handle).To(Equal(customContainer.Handle))

		By("finding a check container when its resource type version does not match worker's")
		containerWithCorrectVersion := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:               db.ContainerStageRun,
				CheckType:           "some-type",
				CheckSource:         atc.Source{"some-type": "some-source"},
				ResourceTypeVersion: atc.Version{"some-type": "some-version"},
			},
			ContainerMetadata: db.ContainerMetadata{
				WorkerName: "some-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some-type": "some-source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		containerWithCorrectVersionCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		containerWithCorrectVersion.Handle = containerWithCorrectVersionCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(containerWithCorrectVersion, time.Duration(0))
		Expect(err).NotTo(HaveOccurred())

		foundContainer, found, err = database.FindContainerByIdentifier(containerWithCorrectVersion.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundContainer.Handle).To(Equal(containerWithCorrectVersion.Handle))

		By("not finding a container if its worker is not running")
		_, err = dbConn.Exec(`update workers set state=$1, addr=NULL, baggageclaim_url=NULL where name=$2`, "stalled", "some-worker")
		Expect(err).NotTo(HaveOccurred())
		foundContainer, found, err = database.FindContainerByIdentifier(containerWithCorrectVersion.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		_, err = dbConn.Exec(`update workers set state=$1, addr=$2, baggageclaim_url=$3 where name=$4`, "running", "1.2.3.4:7777", "1.2.3.4:7788", "some-worker")
		Expect(err).NotTo(HaveOccurred())

		By("not finding a container if its in creating state")

		By("not finding a check container if its worker base resource type version is updated")

		By("not finding a check container whose best_used_by_time has elapsed")
		sourContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "sour-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		resourceConfig, err = dbngResourceConfigFactory.FindOrCreateResourceConfigForResource(
			logger,
			getResourceID("some-resource"),
			"some-type",
			atc.Source{"some-type": "sour-source"},
			savedPipeline.ID,
			atc.ResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
		creatingContainer, err = dbngTeam.CreateResourceCheckContainer(dbngWorker, resourceConfig)
		Expect(err).NotTo(HaveOccurred())
		sourContainerCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		sourContainer.Handle = sourContainerCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(sourContainer, 1*time.Nanosecond)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(2 * time.Nanosecond)
		_, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "sour-source"},
				ResourceID:  getResourceID("some-resource"),
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("finding a non-check container whose best_used_by_time has elapsed")
		nonSourContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				BuildID: build.ID(),
				PlanID:  atc.PlanID("non-sour-plan-id"),
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}
		creatingContainer, err = dbngTeam.CreateBuildContainer(dbngWorker, build.ID(), atc.PlanID("non-sour-plan-id"), dbng.ContainerMetadata{
			Type: string(db.ContainerTypeTask),
			Name: "non-sour-container",
		})
		Expect(err).NotTo(HaveOccurred())
		nonSourContainerCreated, err := creatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
		nonSourContainer.Handle = nonSourContainerCreated.Handle()
		_, err = database.UpdateContainerTTLToBeRemoved(nonSourContainer, 1*time.Nanosecond)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(2 * time.Nanosecond)
		_, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				BuildID: build.ID(),
				PlanID:  atc.PlanID("non-sour-plan-id"),
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
	})
})

func getAllContainers(sqldb db.Conn) []db.Container {
	var container_slice []db.Container
	query := `SELECT worker_name, pipeline_id, resource_id, build_id, plan_id
	          FROM containers
						`
	rows, err := sqldb.Query(query)
	Expect(err).NotTo(HaveOccurred())
	defer rows.Close()

	for rows.Next() {
		var container db.Container
		rows.Scan(&container.WorkerName, &container.ResourceID, &container.BuildID, &container.PlanID)
		container_slice = append(container_slice, container)
	}
	return container_slice
}
