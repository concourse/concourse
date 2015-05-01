package db_test

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("SQL DB", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB
	var pipelineDBFactory db.PipelineDBFactory
	var dbSharedBehaviorInput = dbSharedBehaviorInput{}

	BeforeEach(func() {
		var err error
		postgresRunner.CreateTestDB()
		dbConn = postgresRunner.Open()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)

		sqlDB.SaveConfig("some-pipeline", atc.Config{}, db.ConfigVersion(1))
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)

		pipelineDB, err = pipelineDBFactory.BuildWithName("some-pipeline")
		Ω(err).ShouldNot(HaveOccurred())

		dbSharedBehaviorInput.DB = sqlDB
		dbSharedBehaviorInput.PipelineDB = pipelineDB
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	Describe("is a DB", dbSharedBehavior(&dbSharedBehaviorInput))
	Describe("has a job service", jobService(&dbSharedBehaviorInput))
	Describe("Can schedule builds with serial groups", serialGroupsBehavior(&dbSharedBehaviorInput))

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

					TaskConfigPath: "some/config/path.yml",
					TaskConfig: &atc.TaskConfig{
						Image: "some-image",
					},

					Privileged: true,

					Serial: true,

					InputConfigs: []atc.JobInputConfig{
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

					OutputConfigs: []atc.JobOutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							RawPerformOn: []atc.Condition{"success", "failure"},
						},
					},
				},
			},
		}

		otherConfig := atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"job-1", "job-2"},
					Resources: []string{"resource-1", "resource-2"},
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name: "some-other-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
			},

			Jobs: atc.JobConfigs{
				{
					Name: "some-other-job",
				},
			},
		}

		It("can lookup a pipeline by name", func() {
			pipelineName := "a-pipeline-name"
			otherPipelineName := "an-other-pipeline-name"

			err := sqlDB.SaveConfig(pipelineName, config, 0)
			Ω(err).ShouldNot(HaveOccurred())
			err = sqlDB.SaveConfig(otherPipelineName, otherConfig, 0)
			Ω(err).ShouldNot(HaveOccurred())

			pipeline, err := sqlDB.GetPipelineByName(pipelineName)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(pipeline.Name).Should(Equal(pipelineName))
			Ω(pipeline.Config).Should(Equal(config))
			Ω(pipeline.ID).ShouldNot(Equal(0))

			otherPipeline, err := sqlDB.GetPipelineByName(otherPipelineName)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherPipeline.Name).Should(Equal(otherPipelineName))
			Ω(otherPipeline.Config).Should(Equal(otherConfig))
			Ω(otherPipeline.ID).ShouldNot(Equal(0))
		})

		It("can get a list of all active pipelines", func() {
			pipelineName := "a-pipeline-name"
			otherPipelineName := "an-other-pipeline-name"

			err := sqlDB.SaveConfig(pipelineName, config, 0)
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.SaveConfig(otherPipelineName, otherConfig, 0)
			Ω(err).ShouldNot(HaveOccurred())

			pipelines, err := sqlDB.GetAllActivePipelines()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(pipelines).Should(ConsistOf(
				db.SavedPipeline{
					ID: 1,
					Pipeline: db.Pipeline{
						Name:    "some-pipeline",
						Version: db.ConfigVersion(1),
					},
				},
				db.SavedPipeline{
					ID: 2,
					Pipeline: db.Pipeline{
						Name:    pipelineName,
						Config:  config,
						Version: db.ConfigVersion(2),
					},
				},
				db.SavedPipeline{
					ID: 3,
					Pipeline: db.Pipeline{
						Name:    otherPipelineName,
						Config:  otherConfig,
						Version: db.ConfigVersion(3),
					},
				},
			))
		})

		It("can lookup configs by build id", func() {
			err := sqlDB.SaveConfig("my-pipeline", config, 0)

			myPipelineDB, err := pipelineDBFactory.BuildWithName("my-pipeline")
			Ω(err).ShouldNot(HaveOccurred())

			build, err := myPipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			gottenConfig, _, err := sqlDB.GetConfigByBuildID(build.ID)
			Ω(gottenConfig).Should(Equal(config))
		})

		It("can manage multiple pipeline configurations", func() {
			pipelineName := "a-pipeline-name"
			otherPipelineName := "an-other-pipeline-name"

			By("initially being empty")
			Ω(sqlDB.GetConfig(pipelineName)).Should(BeZero())
			Ω(sqlDB.GetConfig(otherPipelineName)).Should(BeZero())

			By("being able to save the config")
			err := sqlDB.SaveConfig(pipelineName, config, 0)
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.SaveConfig(otherPipelineName, otherConfig, 0)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning the saved config to later gets")
			returnedConfig, configVersion, err := sqlDB.GetConfig(pipelineName)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(returnedConfig).Should(Equal(config))
			Ω(configVersion).ShouldNot(Equal(db.ConfigVersion(0)))

			otherReturnedConfig, otherConfigVersion, err := sqlDB.GetConfig(otherPipelineName)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherReturnedConfig).Should(Equal(otherConfig))
			Ω(otherConfigVersion).ShouldNot(Equal(db.ConfigVersion(0)))

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
				Name:           "some-resource",
				TaskConfigPath: "new/config/path.yml",
				InputConfigs: []atc.JobInputConfig{
					{
						RawName:  "new-input",
						Resource: "new-resource",
						Params: atc.Params{
							"new-param": "new-value",
						},
					},
				},
			})

			By("not allowing non-sequential updates")
			err = sqlDB.SaveConfig(pipelineName, updatedConfig, configVersion-1)
			Ω(err).Should(Equal(db.ErrConfigComparisonFailed))

			err = sqlDB.SaveConfig(pipelineName, updatedConfig, configVersion+10)
			Ω(err).Should(Equal(db.ErrConfigComparisonFailed))

			err = sqlDB.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion-1)
			Ω(err).Should(Equal(db.ErrConfigComparisonFailed))

			err = sqlDB.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion+10)
			Ω(err).Should(Equal(db.ErrConfigComparisonFailed))

			By("being able to update the config with a valid con")
			err = sqlDB.SaveConfig(pipelineName, updatedConfig, configVersion)
			Ω(err).ShouldNot(HaveOccurred())
			err = sqlDB.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning the updated config")
			returnedConfig, newConfigVersion, err := sqlDB.GetConfig(pipelineName)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(returnedConfig).Should(Equal(updatedConfig))
			Ω(newConfigVersion).ShouldNot(Equal(configVersion))

			otherReturnedConfig, newOtherConfigVersion, err := sqlDB.GetConfig(otherPipelineName)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherReturnedConfig).Should(Equal(updatedConfig))
			Ω(newOtherConfigVersion).ShouldNot(Equal(otherConfigVersion))
		})
	})
})
