package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
)

var _ = Describe("Keeping track of containers", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		database      *db.SQLDB
		teamDB        db.TeamDB
		savedPipeline db.SavedPipeline
		pipelineDB    db.PipelineDB
		teamID        int
	)

	BeforeEach(func() {
		var err error

		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(dbfakes.FakeConnector)
		retryableConn := &db.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := db.NewLockFactory(retryableConn)

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

		savedPipeline, _, err = teamDB.SaveConfig("some-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("some-other-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, lockFactory)
		pipelineDB = pipelineDBFactory.Build(savedPipeline)

		workerInfo := db.WorkerInfo{
			Name: "updated-resource-type-worker",
			ResourceTypes: []atc.WorkerResourceType{
				atc.WorkerResourceType{
					Type:    "some-type",
					Version: "some-version",
				},
			},
		}
		_, err = database.SaveWorker(workerInfo, 10*time.Minute)
		Expect(err).NotTo(HaveOccurred())
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
				Handle: "handle-2",
				Type:   db.ContainerTypeTask,
				TeamID: teamID,
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
				PipelineID: savedPipeline.ID,
				JobName:    savedBuild4.JobName(),
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		_, err = database.CreateContainer(containerInfo0, 5*time.Minute, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainer(containerInfo1, 0, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainer(containerInfo2, 0, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainer(containerInfo3, 0, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.CreateContainer(containerInfo4, 0, 0, []string{})
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
		_, err := database.CreateContainer(containerToCreate, 42*time.Minute, time.Duration(0), []string{})
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
		_, err = database.CreateContainer(matchingHandleContainer, time.Second, time.Duration(0), []string{})
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
		Expect(actualContainer.TTL).To(Equal(42 * time.Minute))
		Expect(actualContainer.ResourceTypeVersion).To(Equal(resourceTypeVersion))
		Expect(actualContainer.TeamID).To(Equal(teamID))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("not returning expired container")
		err = database.UpdateExpiresAtOnContainer("some-handle", -time.Minute)
		Expect(err).NotTo(HaveOccurred())

		_, found, err = database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can create and get a step container info object", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: 1111,
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
		_, err := database.CreateContainer(containerToCreate, time.Minute, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		duplicateHandleContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: 1112,
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
		_, err = database.CreateContainer(duplicateHandleContainer, time.Second, time.Duration(0), []string{})
		Expect(err).To(HaveOccurred())

		By("trying to create a container with an insufficient step identifier")
		insufficientStepContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: 1113,
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
		_, err = database.CreateContainer(insufficientStepContainer, time.Second, time.Duration(0), []string{})
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
		_, err = database.CreateContainer(insufficientCheckContainer, time.Second, time.Duration(0), []string{})
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
		Expect(actualContainer.BuildName).To(Equal(""))
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
		_, err = database.CreateContainer(containerToCreate, time.Minute, time.Duration(0), []string{})
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

	Describe("UpdateExpiresAtOnContainer", func() {
		BeforeEach(func() {
			containerToCreate := db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					Stage:   db.ContainerStageRun,
					PlanID:  "update-ttl-plan",
					BuildID: 2000,
				},
				ContainerMetadata: db.ContainerMetadata{
					Handle:     "some-handle",
					Type:       db.ContainerTypeTask,
					WorkerName: "some-worker",
					PipelineID: savedPipeline.ID,
					TeamID:     teamID,
				},
			}
			savedContainer, err := database.CreateContainer(containerToCreate, 5*time.Minute, time.Duration(0), []string{})
			Expect(err).NotTo(HaveOccurred())

			Expect(savedContainer.TTL).To(Equal(5 * time.Minute))
			Expect(savedContainer.ExpiresIn).To(BeNumerically("<=", 5*time.Minute, 5*time.Second))
		})

		It("can update the time to live for a container info object", func() {
			timeBefore := time.Now()

			err := database.UpdateExpiresAtOnContainer("some-handle", 3*time.Second)
			Expect(err).NotTo(HaveOccurred())

			updatedContainer, found, err := database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(updatedContainer.TTL).To(Equal(3 * time.Second))
			Expect(updatedContainer.ExpiresIn).To(BeNumerically("<=", 3*time.Second, 2*time.Second))

			Eventually(func() bool {
				_, found, err := database.GetContainer("some-handle")
				Expect(err).NotTo(HaveOccurred())
				return found
			}, 10*time.Second).Should(BeFalse())

			timeAfter := time.Now()
			Expect(timeAfter.Sub(timeBefore)).To(BeNumerically("<=", 5*time.Second))
			Expect(timeAfter.Sub(timeBefore)).To(BeNumerically("<", 10*time.Second))
		})

		It("can set ttl to infinite", func() {
			err := database.UpdateExpiresAtOnContainer("some-handle", 0)
			Expect(err).NotTo(HaveOccurred())

			updatedContainer, found, err := database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(updatedContainer.TTL).To(BeZero())
			Expect(updatedContainer.ExpiresIn).To(BeZero())
		})
	})

	It("can reap a container", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  "to-be-reaped-plan",
				BuildID: 1000,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "some-reaped-handle",
				Type:       db.ContainerTypeTask,
				WorkerName: "some-worker",
				PipelineID: savedPipeline.ID,
				TeamID:     teamID,
			},
		}
		_, err := database.CreateContainer(containerToCreate, time.Minute, time.Duration(0), []string{})
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

		checkContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-a-handle",
				Type:   db.ContainerTypeCheck,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		getContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-a-handle",
				Type:   db.ContainerTypeGet,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		checkContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-b-handle",
				Type:   db.ContainerTypeCheck,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		getContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-b-handle",
				Type:   db.ContainerTypeGet,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		runContainer, err := database.CreateContainer(db.Container{
			ContainerIdentifier: runStageContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "run-handle",
				Type:   db.ContainerTypeTask,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
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

		getStageAContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-a",
			Stage:               db.ContainerStageGet,
		}

		checkStageBContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-b",
			Stage:               db.ContainerStageCheck,
		}

		getStageBContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-b",
			Stage:               db.ContainerStageGet,
		}

		runStageContainerID := db.ContainerIdentifier{
			ResourceID:  1,
			CheckSource: atc.Source{"some": "source"},
			CheckType:   "some-type",
			Stage:       db.ContainerStageRun,
		}

		checkContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-a-handle",
				Type:   db.ContainerTypeCheck,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		getContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-a-handle",
				Type:   db.ContainerTypeGet,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		checkContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-b-handle",
				Type:   db.ContainerTypeCheck,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		getContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-b-handle",
				Type:   db.ContainerTypeGet,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
		Expect(err).ToNot(HaveOccurred())

		runContainer, err := database.CreateContainer(db.Container{
			ContainerIdentifier: runStageContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "run-handle",
				Type:   db.ContainerTypeTask,
				TeamID: teamID,
			},
		}, time.Minute, time.Duration(0), []string{})
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
		stepContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("plan-id"),
				BuildID: 555,
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
		otherStepContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("other-plan-id"),
				BuildID: 666,
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
			},
		}
		expiredContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("plan-id"),
				BuildID: 789,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:     "expired",
				WorkerName: "some-worker",
				StepName:   "other-container",
				Type:       db.ContainerTypeTask,
				TeamID:     teamID,
			},
		}

		_, err := database.CreateContainer(containerToCreate, time.Minute, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())
		_, err = database.CreateContainer(stepContainerToCreate, time.Minute, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())
		_, err = database.CreateContainer(otherStepContainer, time.Minute, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())
		_, err = database.CreateContainer(expiredContainer, -time.Minute, time.Duration(0), []string{})
		Expect(err).NotTo(HaveOccurred())

		allContainers := getAllContainers(dbConn)
		Expect(allContainers).To(HaveLen(4))

		By("not returning expired container")
		_, found, err := database.FindContainerByIdentifier(
			expiredContainer.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("returning a single matching resource container info")
		actualContainer, found, err := database.FindContainerByIdentifier(
			containerToCreate.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.Handle).To(Equal("some-handle"))
		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(containerToCreate.ResourceID))
		Expect(actualContainer.TeamID).To(Equal(teamID))

		By("returning a single matching step container info")
		actualStepContainer, found, err := database.FindContainerByIdentifier(
			stepContainerToCreate.ContainerIdentifier,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(actualStepContainer.Handle).To(Equal("other-handle"))
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
		_, err = database.CreateContainer(invalidCheckContainerToCreate, time.Minute, time.Duration(0), []string{})
		Expect(err).To(HaveOccurred())

		By("validating pipeline container has pipeline ID")
		_, err = database.CreateContainer(invalidMetadataContainerToCreate, time.Minute, time.Duration(0), []string{})
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
				Handle:       "new-source-handle",
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		_, err = database.CreateContainer(newSourceContainerToCreate, time.Minute, time.Duration(0), []string{})
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

		_, err = database.CreateContainer(newCheckTypeContainerToCreate, time.Minute, time.Duration(0), []string{})
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
				Handle:       "matching-handle",
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		createdMatchingContainer, err := database.CreateContainer(matchingContainerToCreate, time.Minute, time.Duration(0), []string{})
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

		By("removing it if the TTL has expired")
		ttl := 1 * time.Second

		err = database.UpdateExpiresAtOnContainer(otherHandle, -ttl)
		Expect(err).NotTo(HaveOccurred())
		_, found, err = database.FindContainerByIdentifier(
			stepContainerToCreate.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

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
				WorkerName: "updated-resource-type-worker",
				Type:       db.ContainerTypeCheck,
				PipelineID: savedPipeline.ID,
				TeamID:     teamID,
			},
		}

		_, err = database.CreateContainer(customContainer, 10*time.Minute, 0, []string{})
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
				Handle:     "updated-resource-type-container",
				WorkerName: "updated-resource-type-worker",
				Type:       db.ContainerTypeCheck,
				TeamID:     teamID,
			},
		}

		_, err = database.CreateContainer(containerWithCorrectVersion, 10*time.Minute, 0, []string{})
		Expect(err).NotTo(HaveOccurred())

		foundContainer, found, err = database.FindContainerByIdentifier(containerWithCorrectVersion.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundContainer.Handle).To(Equal(containerWithCorrectVersion.Handle))

		By("not finding a check container whose ttl has not expired, but whose best_used_by_time has elapsed")
		sourContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-sour-new-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "sour-check-type-handle",
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		_, err = database.CreateContainer(sourContainer, time.Minute, 1*time.Nanosecond, []string{})
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(2 * time.Nanosecond)
		_, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-sour-new-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("finding a non-check container whose ttl has not expired, but whose best_used_by_time has elapsed")
		nonSourContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				BuildID: 42,
				PlanID:  atc.PlanID("plan-id"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "non-sour-type-handle",
				PipelineID:   savedPipeline.ID,
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				TeamID:       teamID,
			},
		}

		_, err = database.CreateContainer(nonSourContainer, time.Minute, 1*time.Nanosecond, []string{})
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(2 * time.Nanosecond)
		_, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				BuildID: 42,
				PlanID:  atc.PlanID("plan-id"),
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
