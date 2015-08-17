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
		bus := db.NewNotificationsBus(listener)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)

		_, err := sqlDB.SaveConfig("a-pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
		Ω(err).ShouldNot(HaveOccurred())

		pipelineDB, err = pipelineDBFactory.BuildWithName("a-pipeline-name")
		Ω(err).ShouldNot(HaveOccurred())

		_, err = sqlDB.SaveConfig("another-pipeline", atc.Config{}, 0, db.PipelineUnpaused)
		Ω(err).ShouldNot(HaveOccurred())

		otherPipelineDB, err = pipelineDBFactory.BuildWithName("another-pipeline")
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	Context("GetJobBuildsMaxID", func() {
		var (
			build1 db.Build
			build2 db.Build
			build3 db.Build
			err    error
		)

		BeforeEach(func() {
			build1, err = pipelineDB.CreateJobBuild("job-name")
			build2, err = pipelineDB.CreateJobBuild("job-name")
			build3, err = pipelineDB.CreateJobBuild("other-job-name")
		})

		It("returns the max id from the builds table by job name, scoped to the pipeline", func() {
			maxID, err := pipelineDB.GetJobBuildsMaxID("job-name")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(maxID).Should(Equal(build2.ID))

			maxID, err = otherPipelineDB.GetJobBuildsMaxID("job-name")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(maxID).Should(BeZero())
		})
	})

	Context("GetJobBuildsCursor", func() {
		var (
			build1 db.Build
			build2 db.Build
			build3 db.Build
			err    error
		)

		BeforeEach(func() {
			build1, err = pipelineDB.CreateJobBuild("job-name")
			Ω(err).ShouldNot(HaveOccurred())
			build2, err = pipelineDB.CreateJobBuild("job-name")
			Ω(err).ShouldNot(HaveOccurred())
			_, err = pipelineDB.CreateJobBuild("other-name") // add in another test verifying this record doesn't fuck shit up
			Ω(err).ShouldNot(HaveOccurred())
			build3, err = pipelineDB.CreateJobBuild("job-name")
			Ω(err).ShouldNot(HaveOccurred())
			_, err = pipelineDB.CreateJobBuild("job-name")
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("returns a slice of builds limitied by the passed in limit, ordered by id desc", func() {
			builds, _, err := pipelineDB.GetJobBuildsCursor("job-name", 0, true, 2)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(len(builds)).Should(Equal(2))

			Ω(builds[0].ID).Should(BeNumerically(">", builds[1].ID))
		})

		Context("when resultsGreaterThanStartingID is true", func() {
			It("returns a slice of builds with ID's equal to and less than the starting ID", func() {
				builds, _, err := pipelineDB.GetJobBuildsCursor("job-name", build2.ID, true, 2)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(builds).Should(ConsistOf([]db.Build{
					build3,
					build2,
				}))
			})

			Context("when there are more results that are greater than the given starting id", func() {
				It("returns true for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build2.ID, true, 2)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(moreResultsInGivenDirection).Should(BeTrue())
				})
			})

			Context("when there are not more results that are greater than the given starting id", func() {
				It("returns false for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build2.ID, true, 1000)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(moreResultsInGivenDirection).Should(BeFalse())
				})
			})
		})

		Context("when resultsGreaterThanStartingID is false", func() {
			It("returns a slice of builds with ID's equal to and less than the starting ID", func() {
				builds, _, err := pipelineDB.GetJobBuildsCursor("job-name", build3.ID, false, 2)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(builds).Should(ConsistOf([]db.Build{
					build3,
					build2,
				}))
			})

			Context("when there are more results that are less than the given starting id", func() {
				It("returns true for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build3.ID, false, 2)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(moreResultsInGivenDirection).Should(BeTrue())
				})
			})

			Context("when there are not more results that are less than the given starting id", func() {
				It("returns false for moreResultsInGivenDirection", func() {
					_, moreResultsInGivenDirection, err := pipelineDB.GetJobBuildsCursor("job-name", build3.ID, false, 1000)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(moreResultsInGivenDirection).Should(BeFalse())
				})
			})
		})

	})
})
