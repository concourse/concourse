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
		postgresRunner.CreateTestDB()

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

		postgresRunner.DropTestDB()
	})

	Context("GetJobBuildsMaxID", func() {
		var (
			build2 db.Build
			err    error
		)

		BeforeEach(func() {
			_, err = pipelineDB.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())

			build2, err = pipelineDB.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())

			_, err = pipelineDB.CreateJobBuild("other-job-name")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the max id from the builds table by job name, scoped to the pipeline", func() {
			maxID, err := pipelineDB.GetJobBuildsMaxID("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(maxID).To(Equal(build2.ID))

			maxID, err = otherPipelineDB.GetJobBuildsMaxID("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(maxID).To(BeZero())
		})
	})

	Context("GetJobBuildsCursor", func() {
		var (
			build2 db.Build
			build3 db.Build
			err    error
		)

		BeforeEach(func() {
			_, err = pipelineDB.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())

			build2, err = pipelineDB.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())

			_, err = pipelineDB.CreateJobBuild("other-name") // add in another test verifying this record doesn't fuck shit up
			Expect(err).NotTo(HaveOccurred())

			build3, err = pipelineDB.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())

			_, err = pipelineDB.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a slice of builds limitied by the passed in limit, ordered by id desc", func() {
			builds, _, err := pipelineDB.GetJobBuildsCursor("job-name", 0, true, 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(builds)).To(Equal(2))

			Expect(builds[0].ID).To(BeNumerically(">", builds[1].ID))
		})

		Context("when resultsGreaterThanStartingID is true", func() {
			It("returns a slice of builds with ID's equal to and less than the starting ID", func() {
				builds, _, err := pipelineDB.GetJobBuildsCursor("job-name", build2.ID, true, 2)
				Expect(err).NotTo(HaveOccurred())

				Expect(builds).To(ConsistOf([]db.Build{
					build3,
					build2,
				}))

			})

			Context("when there are more results that are greater than the given starting id", func() {
				It("returns true for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build2.ID, true, 2)
					Expect(err).NotTo(HaveOccurred())
					Expect(moreResultsInGivenDirection).To(BeTrue())
				})
			})

			Context("when there are not more results that are greater than the given starting id", func() {
				It("returns false for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build2.ID, true, 1000)
					Expect(err).NotTo(HaveOccurred())
					Expect(moreResultsInGivenDirection).To(BeFalse())
				})
			})
		})

		Context("when resultsGreaterThanStartingID is false", func() {
			It("returns a slice of builds with ID's equal to and less than the starting ID", func() {
				builds, _, err := pipelineDB.GetJobBuildsCursor("job-name", build3.ID, false, 2)
				Expect(err).NotTo(HaveOccurred())

				Expect(builds).To(ConsistOf([]db.Build{
					build3,
					build2,
				}))

			})

			Context("when there are more results that are less than the given starting id", func() {
				It("returns true for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build3.ID, false, 2)
					Expect(err).NotTo(HaveOccurred())
					Expect(moreResultsInGivenDirection).To(BeTrue())
				})
			})

			Context("when there are not more results that are less than the given starting id", func() {
				It("returns false for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build3.ID, false, 1000)
					Expect(err).NotTo(HaveOccurred())
					Expect(moreResultsInGivenDirection).To(BeFalse())
				})
			})
		})

	})
})
