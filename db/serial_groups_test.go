package db_test

import (
	"database/sql"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Scheduling multiple builds within the same serial groups", func() {
	var jobOneConfig atc.JobConfig
	var jobOneTwoConfig atc.JobConfig
	var jobTwoConfig atc.JobConfig
	var dbConn *sql.DB
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)

		team, err := sqlDB.SaveTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		sqlDB.SaveConfig(team.Name, "some-pipeline", atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		pipelineDB, err = pipelineDBFactory.BuildWithName("some-pipeline")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	//TODO: Combine
	BeforeEach(func() {
		jobOneConfig = atc.JobConfig{
			Name:         "job-one",
			SerialGroups: []string{"one"},
		}
		jobOneTwoConfig = atc.JobConfig{
			Name:         "job-one-two",
			SerialGroups: []string{"one", "two"},
		}
		jobTwoConfig = atc.JobConfig{
			Name:         "job-two",
			SerialGroups: []string{"two"},
		}
	})

	Context("When a job is not the next most pending job within a serial group", func() {
		It("should not be scheduled", func() {
			buildOne, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
			Expect(err).NotTo(HaveOccurred())

			buildTwo, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
			Expect(err).NotTo(HaveOccurred())

			buildThree, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
			Expect(err).NotTo(HaveOccurred())

			scheduled, err := pipelineDB.ScheduleBuild(buildOne.ID, jobOneConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeTrue())

			scheduled, err = pipelineDB.ScheduleBuild(buildTwo.ID, jobOneConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeFalse())
			scheduled, err = pipelineDB.ScheduleBuild(buildThree.ID, jobOneTwoConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeFalse())

			Expect(sqlDB.FinishBuild(buildOne.ID, db.StatusSucceeded)).To(Succeed())

			scheduled, err = pipelineDB.ScheduleBuild(buildThree.ID, jobOneTwoConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeFalse())

			scheduled, err = pipelineDB.ScheduleBuild(buildTwo.ID, jobOneConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeTrue())
		})
	})

	Context("when a build is running under job-one", func() {
		BeforeEach(func() {
			var err error
			build, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
			Expect(err).NotTo(HaveOccurred())

			scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeTrue())
		})

		Context("and we schedule a build for job-one", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})

		Context("and we schedule a build for job-one-two", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})

		Context("and we schedule a build for job-two", func() {
			It("succeeds", func() {
				build, err := pipelineDB.CreateJobBuild(jobTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())
			})
		})
	})

	Context("When a build is running in job-one-two", func() {
		BeforeEach(func() {
			var err error
			build, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
			Expect(err).NotTo(HaveOccurred())

			scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeTrue())
		})

		Context("and we schedule a build for job-one", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})

		Context("and we schedule a build for job-one-two", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})

		Context("and we schedule a build for job-two", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})
	})

	Context("When a build is running in job two", func() {
		BeforeEach(func() {
			var err error
			build, err := pipelineDB.CreateJobBuild(jobTwoConfig.Name)
			Expect(err).NotTo(HaveOccurred())

			scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduled).To(BeTrue())
		})

		Context("and we schedule a build for job-one", func() {
			It("succeeds", func() {
				build, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())
			})
		})

		Context("and we schedule a build for job-one-two", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})

		Context("and we schedule a build for job-two", func() {
			It("fails", func() {
				build, err := pipelineDB.CreateJobBuild(jobTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeFalse())
			})
		})
	})
})
