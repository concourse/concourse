package db_test

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
)

var _ = Describe("Updating pipeline config for specific team", func() {
	type SerialGroup struct {
		JobID int
		Name  string
	}

	var dbConn db.Conn
	var listener *pq.Listener

	var database *db.SQLDB
	var pipelineDBFactory db.PipelineDBFactory

	var team db.SavedTeam
	var teamDB db.TeamDB
	var teamDBFactory db.TeamDBFactory

	var config atc.Config
	var otherConfig atc.Config

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(dbfakes.FakeConnector)
		retryableConn := &db.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := db.NewLockFactory(retryableConn)
		database = db.NewSQL(dbConn, bus, lockFactory)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		var err error
		team, err = database.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB = teamDBFactory.GetTeamDB("some-team")

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

			ResourceTypes: atc.ResourceTypes{
				{
					Name: "some-resource-type",
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

					Serial:       true,
					SerialGroups: []string{"serial-group-1", "serial-group-2"},

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
			_, created, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		It("caches the team id", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.TeamID).To(Equal(team.ID))
		})

		It("can be saved as paused", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused).To(BeTrue())
		})

		It("can be saved as unpaused", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused).To(BeFalse())
		})

		It("defaults to paused", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipeline.Paused).To(BeTrue())
		})

		It("creates all of the resources from the pipeline in the database", func() {
			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			resource, found, err := pipelineDB.GetResource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Config).To(Equal(atc.ResourceConfig{
				Name: "some-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			}))
		})

		It("updates resource config", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			config.Resources[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 1, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			resource, found, err := pipelineDB.GetResource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Config).To(Equal(atc.ResourceConfig{
				Name: "some-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-other-config": "some-other-value",
				},
			}))
		})

		It("marks resource as inactive if it is no longer in config", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			config.Resources = []atc.ResourceConfig{}

			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 1, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			_, found, err := pipelineDB.GetResource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the resource types from the pipeline in the database", func() {
			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			resourceType, found, err := pipelineDB.GetResourceType("some-resource-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resourceType.Config).To(Equal(atc.ResourceType{
				Name: "some-resource-type",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			}))
		})

		It("updates resource type config from the pipeline in the database", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			config.ResourceTypes[0].Source = atc.Source{
				"source-other-config": "some-other-value",
			}

			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 1, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			resourceType, found, err := pipelineDB.GetResourceType("some-resource-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resourceType.Config).To(Equal(atc.ResourceType{
				Name: "some-resource-type",
				Type: "some-type",
				Source: atc.Source{
					"source-other-config": "some-other-value",
				},
			}))
		})

		It("marks resource type as inactive if it is no longer in config", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			config.ResourceTypes = []atc.ResourceType{}

			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 1, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			_, found, err := pipelineDB.GetResourceType("some-resource-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the jobs from the pipeline in the database", func() {
			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			job, found, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.Config).To(Equal(config.Jobs[0]))
		})

		It("updates job config", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			config.Jobs[0].Public = false

			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 1, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			job, found, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.Config.Public).To(BeFalse())
		})

		It("marks job inactive", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			config.Jobs = []atc.JobConfig{}

			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 1, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			_, found, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("creates all of the serial groups from the jobs in the database", func() {
			savedPipeline, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			serialGroups := []SerialGroup{}
			rows, err := dbConn.Query("SELECT job_id, serial_group FROM jobs_serial_groups")
			Expect(err).NotTo(HaveOccurred())

			for rows.Next() {
				var serialGroup SerialGroup
				err = rows.Scan(&serialGroup.JobID, &serialGroup.Name)
				Expect(err).NotTo(HaveOccurred())
				serialGroups = append(serialGroups, serialGroup)
			}

			pipelineDB := pipelineDBFactory.Build(savedPipeline)

			job, found, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(serialGroups).To(ConsistOf([]SerialGroup{
				{
					JobID: job.ID,
					Name:  "serial-group-1",
				},
				{
					JobID: job.ID,
					Name:  "serial-group-2",
				},
			}))
		})
	})

	Context("on updates", func() {
		var pipelineName string

		BeforeEach(func() {
			pipelineName = "a-pipeline-name"
		})

		It("it returns created as false", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			_, _, configVersion, err := teamDB.GetConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, created, err := teamDB.SaveConfig(pipelineName, config, configVersion, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeFalse())
		})

		It("updating from paused to unpaused", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused).To(BeTrue())

			_, _, configVersion, err := teamDB.GetConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = teamDB.SaveConfig(pipelineName, config, configVersion, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err = teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused).To(BeFalse())
		})

		It("updating from unpaused to paused", func() {
			_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused).To(BeFalse())

			_, _, configVersion, err := teamDB.GetConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = teamDB.SaveConfig(pipelineName, config, configVersion, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pipeline, found, err = teamDB.GetPipelineByName(pipelineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.Paused).To(BeTrue())
		})

		Context("updating with no change", func() {
			It("maintains paused if the pipeline is paused", func() {
				_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelinePaused)
				Expect(err).NotTo(HaveOccurred())

				pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused).To(BeTrue())

				_, _, configVersion, err := teamDB.GetConfig(pipelineName)
				Expect(err).NotTo(HaveOccurred())

				_, _, err = teamDB.SaveConfig(pipelineName, config, configVersion, db.PipelineNoChange)
				Expect(err).NotTo(HaveOccurred())

				pipeline, found, err = teamDB.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused).To(BeTrue())
			})

			It("maintains unpaused if the pipeline is unpaused", func() {
				_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused).To(BeFalse())

				_, _, configVersion, err := teamDB.GetConfig(pipelineName)
				Expect(err).NotTo(HaveOccurred())

				_, _, err = teamDB.SaveConfig(pipelineName, config, configVersion, db.PipelineNoChange)
				Expect(err).NotTo(HaveOccurred())

				pipeline, found, err = teamDB.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline.Paused).To(BeFalse())
			})
		})
	})

	It("can lookup a pipeline by name", func() {
		pipelineName := "a-pipeline-name"
		otherPipelineName := "an-other-pipeline-name"

		_, _, err := teamDB.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = teamDB.SaveConfig(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipeline, found, err := teamDB.GetPipelineByName(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(pipeline.Name).To(Equal(pipelineName))
		Expect(pipeline.Config).To(Equal(config))
		Expect(pipeline.ID).NotTo(Equal(0))

		otherPipeline, found, err := teamDB.GetPipelineByName(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(otherPipeline.Name).To(Equal(otherPipelineName))
		Expect(otherPipeline.Config).To(Equal(otherConfig))
		Expect(otherPipeline.ID).NotTo(Equal(0))
	})

	It("can order pipelines", func() {
		_, _, err := teamDB.SaveConfig("some-pipeline", atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("pipeline-1", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("pipeline-2", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("pipeline-3", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("pipeline-4", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("pipeline-5", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		err = teamDB.OrderPipelines([]string{
			"pipeline-4",
			"pipeline-3",
			"pipeline-5",
			"pipeline-1",
			"pipeline-2",
			"bogus-pipeline-name", // does not affect it
		})
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig("pipeline-6", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelines, err := teamDB.GetPipelines()
		Expect(err).NotTo(HaveOccurred())

		Expect(pipelines).To(Equal([]db.SavedPipeline{
			{
				ID:       5,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "pipeline-4",
					Config:  config,
					Version: db.ConfigVersion(5),
				},
			},
			{
				ID:       4,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "pipeline-3",
					Config:  config,
					Version: db.ConfigVersion(4),
				},
			},
			{
				ID:       6,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "pipeline-5",
					Config:  config,
					Version: db.ConfigVersion(6),
				},
			},
			{
				ID:       2,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "pipeline-1",
					Config:  config,
					Version: db.ConfigVersion(2),
				},
			},
			{
				ID:       3,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "pipeline-2",
					Config:  config,
					Version: db.ConfigVersion(3),
				},
			},

			{
				ID:       1,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "some-pipeline",
					Version: db.ConfigVersion(1),
				},
			},

			{
				ID:       7,
				TeamID:   team.ID,
				TeamName: "some-team",
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

		_, _, err := teamDB.SaveConfig("some-pipeline", atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		err = teamDB.OrderPipelines([]string{
			"some-pipeline",
			pipelineName,
			otherPipelineName,
		})
		Expect(err).NotTo(HaveOccurred())

		pipelines, err := teamDB.GetPipelines()
		Expect(err).NotTo(HaveOccurred())

		Expect(pipelines).To(Equal([]db.SavedPipeline{
			{
				ID:       1,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    "some-pipeline",
					Version: db.ConfigVersion(1),
				},
			},
			{
				ID:       2,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    pipelineName,
					Config:  config,
					Version: db.ConfigVersion(2),
				},
			},
			{
				ID:       3,
				TeamID:   team.ID,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name:    otherPipelineName,
					Config:  otherConfig,
					Version: db.ConfigVersion(3),
				},
			},
		}))
	})

	It("can lookup configs by build id", func() {
		savedPipeline, _, err := teamDB.SaveConfig("my-pipeline", config, 0, db.PipelineUnpaused)

		myPipelineDB := pipelineDBFactory.Build(savedPipeline)

		build, err := myPipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		gottenConfig, _, err := build.GetConfig()
		Expect(gottenConfig).To(Equal(config))
	})

	It("can manage multiple pipeline configurations", func() {
		pipelineName := "a-pipeline-name"
		otherPipelineName := "an-other-pipeline-name"

		By("initially being empty")
		initialConfig, _, _, err := teamDB.GetConfig(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(initialConfig).To(BeZero())
		initialOtherConfig, _, _, err := teamDB.GetConfig(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(initialOtherConfig).To(BeZero())

		By("being able to save the config")
		_, _, err = teamDB.SaveConfig(pipelineName, config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = teamDB.SaveConfig(otherPipelineName, otherConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		By("returning the saved config to later gets")
		returnedConfig, returnedRawConfig, configVersion, err := teamDB.GetConfig(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(returnedConfig).To(Equal(config))
		jsonBytes, err := json.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		Expect(returnedRawConfig).To(MatchJSON(jsonBytes))
		Expect(configVersion).NotTo(Equal(db.ConfigVersion(0)))

		otherReturnedConfig, otherReturnedRawConfig, otherConfigVersion, err := teamDB.GetConfig(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherReturnedConfig).To(Equal(otherConfig))
		jsonBytes, err = json.Marshal(otherConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherReturnedRawConfig).To(MatchJSON(jsonBytes))
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
		_, _, err = teamDB.SaveConfig(pipelineName, updatedConfig, configVersion-1, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		_, _, err = teamDB.SaveConfig(pipelineName, updatedConfig, configVersion+10, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		_, _, err = teamDB.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion-1, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		_, _, err = teamDB.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion+10, db.PipelineUnpaused)
		Expect(err).To(Equal(db.ErrConfigComparisonFailed))

		By("being able to update the config with a valid con")
		_, _, err = teamDB.SaveConfig(pipelineName, updatedConfig, configVersion, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = teamDB.SaveConfig(otherPipelineName, updatedConfig, otherConfigVersion, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		By("returning the updated config")
		returnedConfig, returnedRawConfig, newConfigVersion, err := teamDB.GetConfig(pipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(returnedConfig).To(Equal(updatedConfig))
		rawConfigJSONBytes, err := json.Marshal(updatedConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(returnedRawConfig).To(MatchJSON(rawConfigJSONBytes))
		Expect(newConfigVersion).NotTo(Equal(configVersion))

		otherReturnedConfig, otherReturnedRawConfig, newOtherConfigVersion, err := teamDB.GetConfig(otherPipelineName)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherReturnedConfig).To(Equal(updatedConfig))
		Expect(returnedRawConfig).To(MatchJSON(rawConfigJSONBytes))
		Expect(newOtherConfigVersion).NotTo(Equal(otherConfigVersion))

		By("being able to retrieve invalid config")
		invalidPipelineName := "invalid-config"
		_, _, err = teamDB.SaveConfig(invalidPipelineName, config, 1, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		dbConn.Exec(`
		UPDATE pipelines
		SET config = ':bad_json:'
		WHERE name = 'invalid-config'
		`)

		_, _, invalidConfigVersion, err := teamDB.GetConfig(invalidPipelineName)
		Expect(err).To(BeAssignableToTypeOf(atc.MalformedConfigError{}))
		Expect(err.Error()).To(ContainSubstring("malformed config:"))
		Expect(invalidConfigVersion).NotTo(Equal(db.ConfigVersion(1)))
	})

	Context("when there are multiple teams", func() {
		var otherTeam db.SavedTeam
		var otherTeamDB db.TeamDB

		BeforeEach(func() {
			var err error
			otherTeam, err = database.CreateTeam(db.Team{Name: "some-other-team"})
			Expect(err).NotTo(HaveOccurred())
			otherTeamDB = teamDBFactory.GetTeamDB("some-other-team")
		})

		It("can allow pipelines with the same name across teams", func() {
			_, _, err := teamDB.SaveConfig("steve", config, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to save a pipeline with the same name in another team")
			_, _, err = otherTeamDB.SaveConfig("steve", otherConfig, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			By("getting the config for the correct team's pipeline")
			actualConfig, _, teamPipelineVersion, err := teamDB.GetConfig("steve")
			Expect(actualConfig).To(Equal(config))

			actualOtherConfig, _, otherTeamPipelineVersion, err := otherTeamDB.GetConfig("steve")
			Expect(actualOtherConfig).To(Equal(otherConfig))

			By("updating the pipeline config for the correct team's pipeline")
			_, _, err = teamDB.SaveConfig("steve", otherConfig, teamPipelineVersion, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = otherTeamDB.SaveConfig("steve", config, otherTeamPipelineVersion, db.PipelineNoChange)
			Expect(err).NotTo(HaveOccurred())

			actualOtherConfig, _, teamPipelineVersion, err = teamDB.GetConfig("steve")
			Expect(actualOtherConfig).To(Equal(otherConfig))

			actualConfig, _, otherTeamPipelineVersion, err = otherTeamDB.GetConfig("steve")
			Expect(actualConfig).To(Equal(config))

			By("pausing the correct team's pipeline")
			_, _, err = teamDB.SaveConfig("steve", otherConfig, teamPipelineVersion, db.PipelinePaused)
			Expect(err).NotTo(HaveOccurred())

			pausedPipeline, found, err := teamDB.GetPipelineByName("steve")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			unpausedPipeline, found, err := otherTeamDB.GetPipelineByName("steve")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pausedPipeline.Paused).To(BeTrue())
			Expect(unpausedPipeline.Paused).To(BeFalse())

			By("cannot cross update configs")
			_, _, err = teamDB.SaveConfig("steve", otherConfig, otherTeamPipelineVersion, db.PipelineNoChange)
			Expect(err).To(HaveOccurred())

			_, _, err = teamDB.SaveConfig("steve", otherConfig, otherTeamPipelineVersion, db.PipelinePaused)
			Expect(err).To(HaveOccurred())
		})
	})
})
