package db_test

import (
	"database/sql"
	"os"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
)

var _ = Describe("SQL DB", func() {
	var postgresRunner postgresrunner.Runner

	var dbConn *sql.DB
	var listener *pq.Listener
	var dbProcess ifrit.Process

	var sqlDB *db.SQLDB

	var dbSharedBehaviorInput = dbSharedBehaviorInput{}

	BeforeSuite(func() {
		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Envoke(postgresRunner)
	})

	AfterSuite(func() {
		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
	})

	BeforeEach(func() {
		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, listener)

		dbSharedBehaviorInput.DB = sqlDB
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	Describe("is a DB", dbSharedBehavior(&dbSharedBehaviorInput))

	Describe("config", func() {
		yep := true

		config := atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"job-1", "job-2"},
					Resources: []string{"resource-1", "resource-2"},
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
			},

			Jobs: atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					BuildConfigPath: "some/config/path.yml",
					BuildConfig: &atc.BuildConfig{
						Image: "some-image",
					},

					Privileged: true,

					Serial: true,

					Inputs: []atc.JobInputConfig{
						{
							RawName:  "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed:     []string{"job-1", "job-2"},
							RawTrigger: &yep,
						},
					},

					Outputs: []atc.JobOutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							RawPerformOn: []atc.OutputCondition{"success", "failure"},
						},
					},
				},
			},
		}

		It("can manage pipeline configuration", func() {
			By("initially being empty")
			Ω(sqlDB.GetConfig()).Should(BeZero())

			By("being able to save the config")
			err := sqlDB.SaveConfig(config)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning the saved config to later gets")
			Ω(sqlDB.GetConfig()).Should(Equal(config))

			updatedConfig := config

			updatedConfig.Groups = append(config.Groups, atc.GroupConfig{
				Name: "new-group",
				Jobs: []string{"new-job-1", "new-job-2"},
			})

			updatedConfig.Resources = append(config.Resources, atc.ResourceConfig{
				Name: "new-resource",
				Type: "new-type",
				Source: atc.Source{
					"new-source-config": "new-value",
				},
			})

			updatedConfig.Jobs = append(config.Jobs, atc.JobConfig{
				Name:            "some-resource",
				BuildConfigPath: "new/config/path.yml",
				Inputs: []atc.JobInputConfig{
					{
						RawName:  "new-input",
						Resource: "new-resource",
						Params: atc.Params{
							"new-param": "new-value",
						},
					},
				},
			})

			By("being able to update the config")
			err = sqlDB.SaveConfig(updatedConfig)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning the updated config")
			Ω(sqlDB.GetConfig()).Should(Equal(updatedConfig))
		})
	})
})
