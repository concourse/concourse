package db_test

import (
	"database/sql"
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
	var dbConn *sql.DB
	var listener *pq.Listener

	var database *db.SQLDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can create and get a container info object", func() {
		expectedContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Name:                 "some-container",
				PipelineName:         "some-pipeline",
				BuildID:              123,
				Type:                 db.ContainerTypeTask,
				WorkerName:           "some-worker",
				WorkingDirectory:     "tmp/build/some-guid",
				CheckType:            "some-type",
				CheckSource:          atc.Source{"uri": "http://example.com"},
				PlanID:               "some-plan-id",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
			},
			Handle: "some-handle",
		}

		By("creating a container")
		err := database.CreateContainer(expectedContainer, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		err = database.CreateContainer(db.Container{Handle: "some-handle"}, time.Second)
		Expect(err).To(HaveOccurred())

		By("getting the saved info object by h andle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.Handle).To(Equal("some-handle"))
		Expect(actualContainer.Name).To(Equal("some-container"))
		Expect(actualContainer.PipelineName).To(Equal("some-pipeline"))
		Expect(actualContainer.BuildID).To(Equal(123))
		Expect(actualContainer.Type).To(Equal(db.ContainerTypeTask))
		Expect(actualContainer.WorkerName).To(Equal("some-worker"))
		Expect(actualContainer.WorkingDirectory).To(Equal("tmp/build/some-guid"))
		Expect(actualContainer.CheckType).To(Equal("some-type"))
		Expect(actualContainer.CheckSource).To(Equal(atc.Source{"uri": "http://example.com"}))
		Expect(actualContainer.PlanID).To(Equal(atc.PlanID("some-plan-id")))
		Expect(actualContainer.EnvironmentVariables).To(Equal([]string{"VAR1=val1", "VAR2=val2"}))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can update the time to live for a container info object", func() {
		updatedTTL := 5 * time.Minute

		originalContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Type: db.ContainerTypeTask,
			},
			Handle: "some-handle",
		}
		err := database.CreateContainer(originalContainer, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		// comparisonContainer is used to get the expected expiration time in the
		// database timezone to avoid timezone errors
		comparisonContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Type: db.ContainerTypeTask,
			},
			Handle: "comparison-handle",
		}
		err = database.CreateContainer(comparisonContainer, updatedTTL)
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
		info := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Type: db.ContainerTypeTask,
			},
			Handle: "some-handle",
		}

		err := database.CreateContainer(info, time.Minute)
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

	type findContainersByIdentifierExample struct {
		containersToCreate   []db.Container
		identifierToFilerFor db.ContainerIdentifier
		expectedHandles      []string
	}

	DescribeTable("filtering containers by identifier",
		func(example findContainersByIdentifierExample) {
			var results []db.Container
			var handles []string
			var err error

			for _, containerToCreate := range example.containersToCreate {
				if containerToCreate.Type.String() == "" {
					containerToCreate.Type = db.ContainerTypeTask
				}

				err = database.CreateContainer(containerToCreate, 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			}

			results, err = database.FindContainersByIdentifier(example.identifierToFilerFor)
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

		Entry("returns everything when no filters are passed", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a"},
				{Handle: "b"},
			},
			identifierToFilerFor: db.ContainerIdentifier{},
			expectedHandles:      []string{"a", "b"},
		}),

		Entry("does not return things that the filter doesn't match", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a"},
				{Handle: "b"},
			},
			identifierToFilerFor: db.ContainerIdentifier{Name: "some-name"},
			expectedHandles:      nil,
		}),

		Entry("returns containers where the name matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{Name: "some-container"}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{Name: "some-container"}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{Name: "some-other"}},
			},
			identifierToFilerFor: db.ContainerIdentifier{Name: "some-container"},
			expectedHandles:      []string{"a", "b"},
		}),

		Entry("returns containers where the pipeline matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{PipelineName: "some-pipeline"}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{PipelineName: "some-other"}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{PipelineName: "some-pipeline"}},
			},
			identifierToFilerFor: db.ContainerIdentifier{PipelineName: "some-pipeline"},
			expectedHandles:      []string{"a", "c"},
		}),

		Entry("returns containers where the build id matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{BuildID: 1}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{BuildID: 2}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{BuildID: 2}},
			},
			identifierToFilerFor: db.ContainerIdentifier{BuildID: 2},
			expectedHandles:      []string{"b", "c"},
		}),

		Entry("returns containers where the type matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{Type: db.ContainerTypePut}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{Type: db.ContainerTypePut}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{Type: db.ContainerTypeGet}},
			},
			identifierToFilerFor: db.ContainerIdentifier{Type: db.ContainerTypePut},
			expectedHandles:      []string{"a", "b"},
		}),

		Entry("returns containers where the worker name matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{WorkerName: "some-worker"}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{WorkerName: "some-worker"}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{WorkerName: "other"}},
			},
			identifierToFilerFor: db.ContainerIdentifier{WorkerName: "some-worker"},
			expectedHandles:      []string{"a", "b"},
		}),

		Entry("returns containers where the check type matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{CheckType: "some-type"}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{CheckType: "nope"}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{CheckType: "some-type"}},
			},
			identifierToFilerFor: db.ContainerIdentifier{CheckType: "some-type"},
			expectedHandles:      []string{"a", "c"},
		}),

		Entry("returns containers where the check source matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "other-source"}}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}}},
			},
			identifierToFilerFor: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}},
			expectedHandles:      []string{"b", "c"},
		}),

		Entry("returns containers where the step location matches", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{PlanID: "some-id"}},
				{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{PlanID: "some-id"}},
				{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{PlanID: "some-other-id"}},
			},
			identifierToFilerFor: db.ContainerIdentifier{PlanID: "some-id"},
			expectedHandles:      []string{"a", "b"},
		}),

		Entry("returns containers where all fields match", findContainersByIdentifierExample{
			containersToCreate: []db.Container{
				{
					ContainerIdentifier: db.ContainerIdentifier{
						Name:         "some-name",
						PipelineName: "some-pipeline",
						BuildID:      123,
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
					},
					Handle: "a",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{
						Name:         "WROONG",
						PipelineName: "some-pipeline",
						BuildID:      123,
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
					},
					Handle: "b",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{
						Name:         "some-name",
						PipelineName: "some-pipeline",
						BuildID:      123,
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
					},
					Handle: "c",
				},
				{
					ContainerIdentifier: db.ContainerIdentifier{
						WorkerName: "Wat",
					},
					Handle: "d",
				},
			},
			identifierToFilerFor: db.ContainerIdentifier{
				Name:         "some-name",
				PipelineName: "some-pipeline",
				BuildID:      123,
				Type:         db.ContainerTypeCheck,
				WorkerName:   "some-worker",
			},
			expectedHandles: []string{"a", "c"},
		}),
	)

	It("can find a single container info by identifier", func() {
		expectedContainer := db.Container{
			Handle: "some-handle",
			ContainerIdentifier: db.ContainerIdentifier{
				PipelineName: "some-pipeline",
				BuildID:      123,
				Name:         "some-container",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeTask,
				CheckType:    "some-type",
				CheckSource:  atc.Source{"some": "other-source"},
			},
		}
		otherContainer := db.Container{
			Handle: "other-handle",
			ContainerIdentifier: db.ContainerIdentifier{
				Name: "other-container",
				Type: db.ContainerTypeTask,
			},
		}

		err := database.CreateContainer(expectedContainer, time.Minute)
		Expect(err).NotTo(HaveOccurred())
		err = database.CreateContainer(otherContainer, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		By("returning a single matching container info")
		actualContainer, found, err := database.FindContainerByIdentifier(db.ContainerIdentifier{Name: "some-container"})

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(actualContainer.Handle).To(Equal("some-handle"))
		Expect(actualContainer.Name).To(Equal("some-container"))
		Expect(actualContainer.PipelineName).To(Equal("some-pipeline"))
		Expect(actualContainer.BuildID).To(Equal(123))
		Expect(actualContainer.Type).To(Equal(db.ContainerTypeTask))
		Expect(actualContainer.WorkerName).To(Equal("some-worker"))
		Expect(actualContainer.CheckType).To(Equal("some-type"))
		Expect(actualContainer.CheckSource).To(Equal(atc.Source{"some": "other-source"}))
		Expect(actualContainer.ExpiresAt.String()).NotTo(BeEmpty())

		By("erroring if more than one container matches the filter")
		actualContainer, found, err = database.FindContainerByIdentifier(db.ContainerIdentifier{Type: db.ContainerTypeTask})
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(db.ErrMultipleContainersFound))
		Expect(found).To(BeFalse())
		Expect(actualContainer.Handle).To(BeEmpty())

		By("returning found of false if no containers match the filter")
		actualContainer, found, err = database.FindContainerByIdentifier(db.ContainerIdentifier{Name: "nope"})
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(actualContainer.Handle).To(BeEmpty())

		By("removing it if the TTL has expired")
		ttl := 1 * time.Second
		ttlContainer := db.Container{
			Handle: "some-ttl-handle",
			ContainerIdentifier: db.ContainerIdentifier{
				Name: "some-ttl-name",
				Type: db.ContainerTypeTask,
			},
		}

		err = database.CreateContainer(ttlContainer, -ttl)
		Expect(err).NotTo(HaveOccurred())
		_, found, err = database.FindContainerByIdentifier(db.ContainerIdentifier{Name: "some-ttl-name"})
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})
})
