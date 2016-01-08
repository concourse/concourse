package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Keeping track of containers", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database *db.SQLDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)

		_, err := dbConn.Query(`DELETE FROM teams WHERE name = 'main'`)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveTeam(db.Team{Name: atc.DefaultTeamName})
		Expect(err).NotTo(HaveOccurred())

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
		}

		_, err = database.SaveConfig(atc.DefaultTeamName, "some-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig(atc.DefaultTeamName, "some-other-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can create and get a resource container info object", func() {
		expectedContainer := db.Container{
			ContainerMetadata: db.ContainerMetadata{
				ResourceName:         "some-resource",
				PipelineName:         "some-pipeline",
				WorkerName:           "some-worker",
				Type:                 db.ContainerTypeCheck,
				WorkingDirectory:     "tmp/build/some-guid",
				CheckSource:          atc.Source{"uri": "http://example.com"},
				CheckType:            "some-type",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
			},
			Handle: "some-handle",
		}

		By("creating a container")
		createdContainer, pipelineID, err := CreateContainerHelper(expectedContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		_, err = database.CreateContainer(db.Container{Handle: "some-handle"}, time.Second)
		Expect(err).To(HaveOccurred())

		By("getting the saved info object by handle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.ContainerIdentifier.WorkerName).To(Equal(createdContainer.ContainerIdentifier.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(createdContainer.ResourceID))

		Expect(actualContainer.Handle).To(Equal("some-handle"))
		Expect(actualContainer.StepName).To(Equal(""))
		Expect(actualContainer.ResourceName).To(Equal("some-resource"))
		Expect(actualContainer.PipelineID).To(Equal(pipelineID))
		Expect(actualContainer.PipelineName).To(Equal("some-pipeline"))
		Expect(actualContainer.ContainerMetadata.BuildID).To(Equal(0))
		Expect(actualContainer.Type).To(Equal(db.ContainerTypeCheck))
		Expect(actualContainer.ContainerMetadata.WorkerName).To(Equal("some-worker"))
		Expect(actualContainer.WorkingDirectory).To(Equal("tmp/build/some-guid"))
		Expect(actualContainer.CheckType).To(Equal("some-type"))
		Expect(actualContainer.CheckSource).To(Equal(atc.Source{"uri": "http://example.com"}))
		Expect(actualContainer.EnvironmentVariables).To(Equal([]string{"VAR1=val1", "VAR2=val2"}))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	FIt("can create and get a step container info object", func() {
		expectedContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				PlanID: "some-plan-id",
			},
			ContainerMetadata: db.ContainerMetadata{
				StepName:             "some-step-container",
				PipelineName:         "some-pipeline",
				Type:                 db.ContainerTypeTask,
				WorkerName:           "some-worker",
				WorkingDirectory:     "tmp/build/some-guid",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
				Attempts:             []int{1, 2, 4},
			},
			Handle: "some-handle",
		}

		By("creating a container")
		createdContainer, pipelineID, err := CreateContainerHelper(expectedContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		_, err = database.CreateContainer(db.Container{Handle: "some-handle"}, time.Second)
		Expect(err).To(HaveOccurred())

		By("getting the saved info object by handle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.ContainerIdentifier.WorkerName).To(Equal(createdContainer.ContainerIdentifier.WorkerName))
		Expect(actualContainer.ContainerIdentifier.BuildID).To(Equal(createdContainer.ContainerIdentifier.BuildID))
		Expect(actualContainer.PlanID).To(Equal(createdContainer.PlanID))

		Expect(actualContainer.Handle).To(Equal(expectedContainer.Handle))
		Expect(actualContainer.StepName).To(Equal(expectedContainer.StepName))
		Expect(actualContainer.ResourceName).To(Equal(""))
		Expect(actualContainer.PipelineID).To(Equal(pipelineID))
		Expect(actualContainer.PipelineName).To(Equal(expectedContainer.PipelineName))
		Expect(actualContainer.ContainerMetadata.BuildID).To(BeNumerically(">", 0))
		Expect(actualContainer.Type).To(Equal(db.ContainerTypeTask))
		Expect(actualContainer.WorkerName).To(Equal(expectedContainer.WorkerName))
		Expect(actualContainer.WorkingDirectory).To(Equal(expectedContainer.WorkingDirectory))
		Expect(actualContainer.CheckType).To(BeEmpty())
		Expect(actualContainer.CheckSource).To(BeEmpty())
		Expect(actualContainer.EnvironmentVariables).To(Equal(expectedContainer.EnvironmentVariables))
		Expect(actualContainer.Attempts).To(Equal(expectedContainer.Attempts))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can update the time to live for a container info object", func() {
		updatedTTL := 5 * time.Minute

		expectedContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{},
			ContainerMetadata: db.ContainerMetadata{
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
			},
			Handle: "some-handle",
		}
		_, _, err := CreateContainerHelper(expectedContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		// comparisonContainer is used to get the expected expiration time in the
		// database timezone to avoid timezone errors
		comparisonContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{},
			ContainerMetadata: db.ContainerMetadata{
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-other-worker",
				PipelineName: "some-other-pipeline",
			},
			Handle: "comparison-handle",
		}
		_, _, err = CreateContainerHelper(comparisonContainer, updatedTTL, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		comparisonContainer, found, err := database.GetContainer("comparison-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		err = database.UpdateExpiresAtOnContainer("some-handle", updatedTTL)
		Expect(err).NotTo(HaveOccurred())

		updatedContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(updatedContainer.ExpiresAt).To(BeTemporally("~", comparisonContainer.ExpiresAt, time.Second))
	})

	It("can reap a container", func() {
		expectedContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{},
			ContainerMetadata: db.ContainerMetadata{
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
			},
			Handle: "some-handle",
		}

		_, _, err := CreateContainerHelper(expectedContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		_, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		By("reaping an existing container")
		err = database.ReapContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())

		_, found, err = database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("not failing if the container's already been reaped")
		err = database.ReapContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
	})

	type findContainersByMetadataExample struct {
		containersToCreate  []db.Container
		metadataToFilterFor db.ContainerMetadata
		expectedHandles     []string
	}

	DescribeTable("filtering containers by metadata",
		func(example findContainersByMetadataExample) {
			var results []db.Container
			var handles []string
			var err error

			for _, containerToCreate := range example.containersToCreate {
				if containerToCreate.Type.String() == "" {
					containerToCreate.Type = db.ContainerTypeTask
				}

				_, _, err := CreateContainerHelper(containerToCreate, time.Minute, dbConn, database)
				Expect(err).NotTo(HaveOccurred())
			}

			results, err = database.FindContainersByMetadata(example.metadataToFilterFor)
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

		Entry("returns everything when no filters are passed", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
					},
					Handle: "b",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{},
			expectedHandles:     []string{"a", "b"},
		}),

		Entry("does not return things that the filter doesn't match", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
					},
					Handle: "b",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{ResourceName: "some-resource"},
			expectedHandles:     nil,
		}),

		Entry("returns containers where the step name matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						StepName:     "some-step",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
						StepName:     "some-other-step",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
						StepName:     "some-step",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{StepName: "some-step"},
			expectedHandles:     []string{"a", "c"},
		}),

		Entry("returns containers where the resource name matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
						ResourceName: "some-other-resource",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{ResourceName: "some-resource"},
			expectedHandles:     []string{"a", "b"},
		}),

		Entry("returns containers where the pipeline matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-Oother-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{PipelineName: "some-pipeline"},
			expectedHandles:     []string{"a", "c"},
		}),

		Entry("returns containers where the type matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						WorkerName:   "some-other-worker",
						PipelineName: "some-other-pipeline",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeGet,
						WorkerName:   "some-Oother-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{Type: db.ContainerTypePut},
			expectedHandles:     []string{"a", "b"},
		}),

		Entry("returns containers where the worker name matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						WorkerName:   "some-worker",
						PipelineName: "some-other-pipeline",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeGet,
						WorkerName:   "some-other-worker",
						PipelineName: "some-pipeline",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{WorkerName: "some-worker"},
			expectedHandles:     []string{"a", "b"},
		}),

		Entry("returns containers where the check type matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						CheckType:    "",
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						CheckType:    "nope",
						WorkerName:   "some-worker",
						PipelineName: "some-other-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						CheckType:    "some-type",
						WorkerName:   "some-other-worker",
						PipelineName: "some-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{CheckType: "some-type"},
			expectedHandles:     []string{"c"},
		}),

		Entry("returns containers where the check source matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerMetadata: db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
						CheckSource: atc.Source{
							"some": "other-source",
						},
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "a",
				},
				{
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
						PipelineName: "some-other-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "b",
				},
				{
					ContainerMetadata: db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
						CheckSource: atc.Source{
							"some": "source",
						},
						WorkerName:   "some-other-worker",
						PipelineName: "some-pipeline",
						ResourceName: "some-resource",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{CheckSource: atc.Source{"some": "source"}},
			expectedHandles:     []string{"c"},
		}),

		Entry("returns containers where the job name matches", findContainersByMetadataExample{
			containersToCreate: []db.Container{{
				ContainerMetadata: db.ContainerMetadata{
					Type:         db.ContainerTypeTask,
					WorkerName:   "some-worker",
					PipelineName: "some-pipeline",
					JobName:      "some-other-job",
				},
				Handle: "a",
			},
				{
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						JobName:      "some-job",
					},
					Handle: "b",
				},
				{
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-other-worker",
						PipelineName: "some-pipeline",
						JobName:      "",
					},
					Handle: "c",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{JobName: "some-job"},
			expectedHandles:     []string{"b"},
		}),

		Entry("returns containers where all fields match", findContainersByMetadataExample{
			containersToCreate: []db.Container{
				{
					ContainerMetadata: db.ContainerMetadata{
						StepName:     "some-name",
						PipelineName: "some-pipeline",
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
					},
					Handle: "a",
				},
				{
					ContainerMetadata: db.ContainerMetadata{
						StepName:     "WROONG",
						PipelineName: "some-pipeline",
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
					},
					Handle: "b",
				},
				{
					ContainerMetadata: db.ContainerMetadata{
						StepName:     "some-name",
						PipelineName: "some-pipeline",
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
					},
					Handle: "c",
				},
				{
					ContainerMetadata: db.ContainerMetadata{
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						Type:         db.ContainerTypeTask,
					},
					Handle: "d",
				},
			},
			metadataToFilterFor: db.ContainerMetadata{
				StepName:     "some-name",
				PipelineName: "some-pipeline",
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-worker",
			},
			expectedHandles: []string{"a", "c"},
		}),
	)

	It("can find a single container info by identifier", func() {
		handle := "some-handle"
		otherHandle := "other-handle"

		expectedContainer := db.Container{
			Handle: handle,
			ContainerMetadata: db.ContainerMetadata{
				PipelineName: "some-pipeline",
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				CheckType:    "some-type",
				CheckSource:  atc.Source{"some": "other-source"},
			},
		}
		stepContainer := db.Container{
			Handle: otherHandle,
			ContainerIdentifier: db.ContainerIdentifier{
				PlanID: atc.PlanID("plan-id"),
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineName: "some-pipeline",
				WorkerName:   "some-worker",
				StepName:     "other-container",
				Type:         db.ContainerTypeTask,
			},
		}
		otherStepContainer := db.Container{
			Handle: "very-other-handle",
			ContainerIdentifier: db.ContainerIdentifier{
				PlanID: atc.PlanID("other-plan-id"),
			},
			ContainerMetadata: db.ContainerMetadata{
				PipelineName: "some-pipeline",
				WorkerName:   "some-worker",
				StepName:     "other-container",
				Type:         db.ContainerTypeTask,
			},
		}

		newContainer, _, err := CreateContainerHelper(expectedContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())
		newStepContainer, _, err := CreateContainerHelper(stepContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = CreateContainerHelper(otherStepContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		all_containers := getAllContainers(dbConn)
		Expect(all_containers).To(HaveLen(3))

		By("returning a single matching resource container info")
		actualContainer, found, err := database.FindContainerByIdentifier(
			newContainer.ContainerIdentifier,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(actualContainer.Handle).To(Equal("some-handle"))
		Expect(actualContainer.ContainerIdentifier.WorkerName).To(Equal(newContainer.ContainerIdentifier.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(newContainer.ResourceID))
		Expect(actualContainer.ExpiresAt.String()).NotTo(BeEmpty())

		By("returning a single matching step container info")
		actualStepContainer, found, err := database.FindContainerByIdentifier(
			newStepContainer.ContainerIdentifier,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(actualStepContainer.Handle).To(Equal("other-handle"))
		Expect(actualStepContainer.ContainerIdentifier.WorkerName).To(Equal(newStepContainer.ContainerIdentifier.WorkerName))
		Expect(actualStepContainer.ResourceID).To(Equal(newStepContainer.ResourceID))
		Expect(actualStepContainer.ExpiresAt.String()).NotTo(BeEmpty())

		By("erroring if more than one container matches the filter")
		matchingContainer := db.Container{
			Handle: "matching-handle",
			ContainerMetadata: db.ContainerMetadata{
				PipelineName: "some-pipeline",
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
				CheckType:    "some-type",
				CheckSource:  atc.Source{"some": "other-source"},
				BuildID:      1234,
			},
		}

		actualMatchingContainer, _, err := CreateContainerHelper(matchingContainer, time.Minute, dbConn, database)
		Expect(err).NotTo(HaveOccurred())

		foundContainer, found, err := database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID: actualMatchingContainer.ResourceID,
				WorkerName: actualMatchingContainer.ContainerIdentifier.WorkerName,
			})
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(db.ErrMultipleContainersFound))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				BuildID: actualMatchingContainer.ContainerIdentifier.BuildID,
			})
		Expect(err).To(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("returning found of false if no containers match the filter")
		actualContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				BuildID:    -1,
				WorkerName: "some-worker",
				PlanID:     atc.PlanID("plan-id"),
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(actualContainer.Handle).To(BeEmpty())

		By("removing it if the TTL has expired")
		ttl := 1 * time.Second

		err = database.UpdateExpiresAtOnContainer(otherHandle, -ttl)
		Expect(err).NotTo(HaveOccurred())
		_, found, err = database.FindContainerByIdentifier(
			newStepContainer.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
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
		rows.Scan(&container.ContainerIdentifier.WorkerName, &container.ResourceID, &container.ContainerIdentifier.BuildID, &container.PlanID)
		container_slice = append(container_slice, container)
	}
	return container_slice
}

func CreateContainerHelper(container db.Container, ttl time.Duration, sqlDB db.Conn, dbSQL *db.SQLDB) (db.Container, int, error) {
	pipeline, err := dbSQL.GetPipelineByTeamNameAndName(atc.DefaultTeamName, "some-pipeline")
	Expect(err).NotTo(HaveOccurred())

	var worker db.WorkerInfo
	worker.Name = container.ContainerIdentifier.WorkerName
	// hacky way to generate unique addresses in the case of multiple workers to
	// avoid matching on empty string
	worker.GardenAddr = time.Now().String()
	insertedWorker, err := dbSQL.SaveWorker(worker, 0)
	Expect(err).NotTo(HaveOccurred())

	pipelineDBFactory := db.NewPipelineDBFactory(nil, sqlDB, nil, dbSQL)
	pipelineDB := pipelineDBFactory.Build(pipeline)

	if container.Type != db.ContainerTypeCheck {
		jobName := container.JobName
		if jobName == "" {
			jobName = "some-random-job"
		}

		build, err := pipelineDB.CreateJobBuild(jobName)
		Expect(err).NotTo(HaveOccurred())

		container.ContainerIdentifier.BuildID = build.ID

		if container.ResourceName != "" {
			input := db.BuildInput{
				Name: container.ResourceName,
				VersionedResource: db.VersionedResource{
					Resource:     container.ResourceName,
					Type:         container.CheckType,
					Metadata:     []db.MetadataField{},
					PipelineName: container.PipelineName,
				},
				FirstOccurrence: false,
			}
			dbSQL.SaveBuildInput(atc.DefaultTeamName, build.ID, input)
		}
	} else {
		err = pipelineDB.SaveResourceVersions(
			atc.ResourceConfig{
				Name:       container.ResourceName,
				Type:       container.CheckType,
				Source:     container.CheckSource,
				CheckEvery: "minute",
			},
			[]atc.Version{atc.Version{"some": "version"}},
		)
		Expect(err).NotTo(HaveOccurred())

		resource, err := pipelineDB.GetResource(container.ResourceName)
		Expect(err).NotTo(HaveOccurred())

		container.ResourceID = resource.ID
	}

	container.ContainerIdentifier.WorkerName = insertedWorker.Name
	createdContainer, err := dbSQL.CreateContainer(container, ttl)
	Expect(err).NotTo(HaveOccurred())

	return createdContainer, pipeline.ID, nil
}
