package db_test

import (
	"database/sql"

	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource History", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB
	var otherPipelineDB db.PipelineDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)

		_, err := sqlDB.SaveConfig("a-pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB, err = pipelineDBFactory.BuildWithName("a-pipeline-name")
		Expect(err).NotTo(HaveOccurred())

		_, err = sqlDB.SaveConfig("another-pipeline", atc.Config{}, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		otherPipelineDB, err = pipelineDBFactory.BuildWithName("another-pipeline")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("GetResourceVersions", func() {
		var resource atc.ResourceConfig
		var versions []atc.Version
		var expectedVersions []db.SavedVersionedResource

		BeforeEach(func() {
			resource = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}

			versions = nil
			expectedVersions = nil
			for i := 0; i < 10; i++ {
				version := atc.Version{"version": fmt.Sprintf("%d", i+1)}
				versions = append(versions, version)
				expectedVersions = append(expectedVersions,
					db.SavedVersionedResource{
						ID:      i + 1,
						Enabled: true,
						VersionedResource: db.VersionedResource{
							Resource:     resource.Name,
							Type:         resource.Type,
							Version:      db.Version(version),
							Metadata:     nil,
							PipelineName: pipelineDB.GetPipelineName(),
						},
					})
			}

			err := pipelineDB.SaveResourceVersions(resource, versions)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are no versions to be found", func() {
			It("returns the versions, with previous/next pages", func() {
				historyPage, pagination, err := pipelineDB.GetResourceVersions("nope", db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{}))
				Expect(pagination).To(Equal(db.Pagination{}))
			})
		})

		Context("with no since/until", func() {
			It("returns the first page, with the given limit, and a next page", func() {
				historyPage, pagination, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[8].ID, Limit: 2}))
			})
		})

		Context("with a since that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Since: expectedVersions[6].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[5], expectedVersions[4]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[5].ID, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[4].ID, Limit: 2}))
			})
		})

		Context("with a since that places it at the end of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Since: expectedVersions[2].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[1], expectedVersions[0]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[1].ID, Limit: 2}))
				Expect(pagination.Next).To(BeNil())
			})
		})

		Context("with an until that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Until: expectedVersions[6].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[8], expectedVersions[7]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[8].ID, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[7].ID, Limit: 2}))
			})
		})

		Context("with a until that places it at the beginning of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Until: expectedVersions[7].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[8].ID, Limit: 2}))
			})
		})

		Context("when the version has metadata", func() {
			BeforeEach(func() {
				metadata := []db.MetadataField{{Name: "name1", Value: "value1"}}

				expectedVersions[9].Metadata = metadata

				build, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).ToNot(HaveOccurred())

				pipelineDB.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-input",
					VersionedResource: expectedVersions[9].VersionedResource,
					FirstOccurrence:   true,
				})
			})

			It("returns the metadata in the version history", func() {
				historyPage, _, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 1})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9]}))
			})
		})

		Context("when a version is disabled", func() {
			BeforeEach(func() {
				pipelineDB.DisableVersionedResource(10)

				expectedVersions[9].Enabled = false
			})

			It("returns a disabled version", func() {
				historyPage, _, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 1})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9]}))
			})
		})
	})
})
