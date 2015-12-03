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

var _ = Describe("Keeping track of pipeline configs", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var database *db.SQLDB
	var pipelineDBFactory db.PipelineDBFactory

	var config atc.Config
	var otherConfig atc.Config

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, database)

		config = atc.Config{
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

					Serial: true,

					Plan: atc.PlanSequence{
						{
							Get:      "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed:  []string{"job-1", "job-2"},
							Trigger: true,
						},
						{
							Task:           "some-task",
							Privileged:     true,
							TaskConfigPath: "some/config/path.yml",
							TaskConfig: &atc.TaskConfig{
								Image: "some-image",
							},
						},
						{
							Put: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
						},
					},
				},
			},
		}

		otherConfig = atc.Config{
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
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("on initial create", func() {
		var pipelineName string
		BeforeEach(func() {
			pipelineName = "a-pipeline-name"
		})

		It("returns true for created", func() {
			created, err := database.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		It("can be saved as paused", func() {
			_, err := database.SaveConfig(pipelineName, config, 0, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err := database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			Expect(pipeline.Paused).To(BeTrue())
		})

		It("can be saved as unpaused", func() {
			_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err := database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			Expect(pipeline.Paused).To(BeFalse())
		})

		It("defaults to paused", func() {
			_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err := database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			Expect(pipeline.Paused).To(BeTrue())
		})
	})

	Context("on updates", func() {
		var pipelineName string

		BeforeEach(func() {
			pipelineName = "a-pipeline-name"
		})

		It("it returns created as false", func() {
			_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			_, configVersion, err := database.GetConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			created, err := database.SaveConfig(pipelineName, config, configVersion, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeFalse())
		})

		It("updating from paused to unpaused", func() {
			_, err := database.SaveConfig(pipelineName, config, 0, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err := database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipeline.Paused).To(BeTrue())

			_, configVersion, err := database.GetConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.SaveConfig(pipelineName, config, configVersion, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err = database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipeline.Paused).To(BeFalse())
		})

		It("updating from unpaused to paused", func() {
			_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err := database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipeline.Paused).To(BeFalse())

			_, configVersion, err := database.GetConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.SaveConfig(pipelineName, config, configVersion, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, err = database.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipeline.Paused).To(BeTrue())
		})

		Context("updating with no change", func() {
			It("maintains paused if the pipeline is paused", func() {
				_, err := database.SaveConfig(pipelineName, config, 0, db.PipelinePaused)
				Expect(err).NotTo(HaveOccurred())

				pipeline, err := database.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pipeline.Paused).To(BeTrue())

				_, configVersion, err := database.GetConfig(pipelineName)
				Expect(err).NotTo(HaveOccurred())

				_, err = database.SaveConfig(pipelineName, config, configVersion, db.PipelineNoChange)
				Expect(err).NotTo(HaveOccurred())

				pipeline, err = database.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pipeline.Paused).To(BeTrue())
			})

			It("maintains unpaused if the pipeline is unpaused", func() {
				_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pipeline, err := database.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pipeline.Paused).To(BeFalse())

				_, configVersion, err := database.GetConfig(pipelineName)
				Expect(err).NotTo(HaveOccurred())

				_, err = database.SaveConfig(pipelineName, config, configVersion, db.PipelineNoChange)
				Expect(err).NotTo(HaveOccurred())

				pipeline, err = database.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pipeline.Paused).To(BeFalse())
			})
		})
	})

	It("can lookup a pipeline by name", func() {
		pipelineName := "a-pipeline-name"
		otherPipelineName := "an-other-pipeline-name"

		_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
		_, err = database.SaveConfig(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipeline, err := database.GetPipelineByName(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(pipeline.Name).To(Equal(pipelineName))
		Expect(pipeline.Config).To(Equal(config))
		Expect(pipeline.ID).NotTo(Equal(0))

		otherPipeline, err := database.GetPipelineByName(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherPipeline.Name).To(Equal(otherPipelineName))
		Expect(otherPipeline.Config).To(Equal(otherConfig))
		Expect(otherPipeline.ID).NotTo(Equal(0))
	})

	It("can order pipelines", func() {
		_, err := database.SaveConfig("some-pipeline", atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig("pipeline-1", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig("pipeline-2", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig("pipeline-3", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig("pipeline-4", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig("pipeline-5", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		err = database.OrderPipelines([]string{
			"pipeline-4",
			"pipeline-3",
			"pipeline-5",
			"pipeline-1",
			"pipeline-2",
			"bogus-pipeline-name", // does not affect it
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig("pipeline-6", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelines, err := database.GetAllActivePipelines()
		Expect(err).NotTo(HaveOccurred())

		Expect(pipelines).To(Equal([]db.SavedPipeline{
			{
				ID: 5,
				Pipeline: db.Pipeline{
					Name:    "pipeline-4",
					Config:  config,
					Version: db.ConfigVersion(5),
				},
			},
			{
				ID: 4,
				Pipeline: db.Pipeline{
					Name:    "pipeline-3",
					Config:  config,
					Version: db.ConfigVersion(4),
				},
			},
			{
				ID: 6,
				Pipeline: db.Pipeline{
					Name:    "pipeline-5",
					Config:  config,
					Version: db.ConfigVersion(6),
				},
			},
			{
				ID: 2,
				Pipeline: db.Pipeline{
					Name:    "pipeline-1",
					Config:  config,
					Version: db.ConfigVersion(2),
				},
			},
			{
				ID: 3,
				Pipeline: db.Pipeline{
					Name:    "pipeline-2",
					Config:  config,
					Version: db.ConfigVersion(3),
				},
			},

			{
				ID: 1,
				Pipeline: db.Pipeline{
					Name:    "some-pipeline",
					Version: db.ConfigVersion(1),
				},
			},

			{
				ID: 7,
				Pipeline: db.Pipeline{
					Name:    "pipeline-6",
					Config:  config,
					Version: db.ConfigVersion(7),
				},
			},
		}))
	})

	It("can get a list of all active pipelines ordered by 'ordering'", func() {
		pipelineName := "a-pipeline-name"
		otherPipelineName := "an-other-pipeline-name"

		_, err := database.SaveConfig("some-pipeline", atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		err = database.OrderPipelines([]string{
			"some-pipeline",
			pipelineName,
			otherPipelineName,
		})
		Expect(err).NotTo(HaveOccurred())

		pipelines, err := database.GetAllActivePipelines()
		Expect(err).NotTo(HaveOccurred())

		Expect(pipelines).To(Equal([]db.SavedPipeline{
			{
				ID: 1,
				Pipeline: db.Pipeline{
					Name:    "some-pipeline",
					Version: db.ConfigVersion(1),
				},
			},
			{
				ID: 2,
				Pipeline: db.Pipeline{
					Name:    pipelineName,
					Config:  config,
					Version: db.ConfigVersion(2),
				},
			},
			{
				ID: 3,
				Pipeline: db.Pipeline{
					Name:    otherPipelineName,
					Config:  otherConfig,
					Version: db.ConfigVersion(3),
				},
			},
		}))
	})

	It("can lookup configs by build id", func() {
		_, err := database.SaveConfig("my-pipeline", config, 0, db.PipelineUnpaused)

		myPipelineDB, err := pipelineDBFactory.BuildWithName("my-pipeline")
		Expect(err).NotTo(HaveOccurred())

		build, err := myPipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		gottenConfig, _, err := database.GetConfigByBuildID(build.ID)
		Expect(gottenConfig).To(Equal(config))
	})

	It("can manage multiple pipeline configurations", func() {
		pipelineName := "a-pipeline-name"
		otherPipelineName := "an-other-pipeline-name"

		By("initially being empty")
		Expect(database.GetConfig(pipelineName)).To(BeZero())
		Expect(database.GetConfig(otherPipelineName)).To(BeZero())

		By("being able to save the config")
		_, err := database.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, err = database.SaveConfig(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		By("returning the saved config to later gets")
		returnedConfig, configVersion, err := database.GetConfig(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(returnedConfig).To(Equal(config))
		Expect(configVersion).NotTo(Equal(db.ConfigVersion(0)))

		otherReturnedConfig, otherConfigVersion, err := database.GetConfig(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherReturnedConfig).To(Equal(otherConfig))
		Expect(otherConfigVersion).NotTo(Equal(db.ConfigVersion(0)))

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
			Name: "new-job",
			Plan: atc.PlanSequence{
				{
					Get:      "new-input",
					Resource: "new-resource",
					Params: atc.Params{
						"new-param": "new-value",
					},
				},
				{
					Task:           "some-task",
					TaskConfigPath: "new/config/path.yml",
				},
			},
		})

		By("not allowing non-sequential updates")
		_, err = database.SaveConfig(pipelineName, updatedConfig, configVersion-1, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		_, err = database.SaveConfig(pipelineName, updatedConfig, configVersion+10, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		_, err = database.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion-1, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		_, err = database.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion+10, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		By("being able to update the config with a valid con")
		_, err = database.SaveConfig(pipelineName, updatedConfig, configVersion, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
		_, err = database.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		By("returning the updated config")
		returnedConfig, newConfigVersion, err := database.GetConfig(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(returnedConfig).To(Equal(updatedConfig))
		Expect(newConfigVersion).NotTo(Equal(configVersion))

		otherReturnedConfig, newOtherConfigVersion, err := database.GetConfig(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherReturnedConfig).To(Equal(updatedConfig))
		Expect(newOtherConfigVersion).NotTo(Equal(otherConfigVersion))
	})
})
