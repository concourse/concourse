package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Jobs Builds", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB

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
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		_, err := sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB := teamDBFactory.GetTeamDB("some-team")

		config := atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "some-job"},
				{Name: "some-other-job"},
			},
		}

		savedPipeline, _, err := teamDB.SaveConfigToBeDeprecated("a-pipeline-name", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfigToBeDeprecated("another-pipeline", atc.Config{}, 0, db.PipelineUnpaused)
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
				build, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, err = pipelineDB.CreateJobBuild("some-other-job")
				Expect(err).NotTo(HaveOccurred())

				builds[i] = build
			}
		})

		Context("when there are no builds to be found", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("some-other-job", db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{}))
				Expect(pagination).To(Equal(db.Pagination{}))
			})
		})

		Context("with no since/until", func() {
			It("returns the first page, with the given limit, and a next page", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("some-job", db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[8].ID(), Limit: 2}))
			})
		})

		Context("with a since that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("some-job", db.Page{Since: builds[6].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[5], builds[4]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[5].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[4].ID(), Limit: 2}))
			})
		})

		Context("with a since that places it at the end of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("some-job", db.Page{Since: builds[2].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[1], builds[0]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[1].ID(), Limit: 2}))
				Expect(pagination.Next).To(BeNil())
			})
		})

		Context("with an until that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("some-job", db.Page{Until: builds[6].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[8], builds[7]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[8].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[7].ID(), Limit: 2}))
			})
		})

		Context("with a until that places it at the beginning of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := pipelineDB.GetJobBuilds("some-job", db.Page{Until: builds[7].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: builds[8].ID(), Limit: 2}))
			})
		})
	})
})
