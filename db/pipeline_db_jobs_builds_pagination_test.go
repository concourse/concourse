package db_test

import (
	"database/sql"

	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Jobs Builds", func() {
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

	Context("GetJobBuilds", func() {
		var builds [10]db.Build

		BeforeEach(func() {
			for i := 0; i < 10; i++ {
				build, err := pipelineDB.CreateJobBuild("job-name")
				Expect(err).NotTo(HaveOccurred())

				_, err = pipelineDB.CreateJobBuild("other-name")
				Expect(err).NotTo(HaveOccurred())

				builds[i] = build
			}
		})

		Context("when there are no builds to be found", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("nope", db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{}))
				Expect(pagination).To(Equal(db.Pagination{}))
			})
		})

		Context("with no since/until", func() {
			It("returns the first page, with the given limit, and a next page", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[8].ID, Limit: 2}))
			})
		})

		Context("with a since that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Since: builds[6].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[5], builds[4]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[5].ID, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[4].ID, Limit: 2}))
			})
		})

		Context("with a since that places it at the end of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Since: builds[2].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[1], builds[0]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[1].ID, Limit: 2}))
				Expect(pagination.Next).To(BeNil())
			})
		})

		Context("with an until that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Until: builds[6].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[8], builds[7]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[8].ID, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[7].ID, Limit: 2}))
			})
		})

		Context("with a until that places it at the beginning of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Until: builds[7].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[8].ID, Limit: 2}))
			})
		})
	})
})
