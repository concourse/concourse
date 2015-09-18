package db_test

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineDB", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB

	BeforeEach(func() {
		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	pipelineConfig := atc.Config{
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

				SerialGroups: []string{"serial-group"},

				InputConfigs: []atc.JobInputConfig{
					{
						RawName:  "some-input",
						Resource: "some-resource",
						Params: atc.Params{
							"some-param": "some-value",
						},
						Passed:  []string{"job-1", "job-2"},
						Trigger: true,
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

	otherPipelineConfig := atc.Config{
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

	var (
		pipelineDB      db.PipelineDB
		otherPipelineDB db.PipelineDB
	)

	BeforeEach(func() {
		_, err := sqlDB.SaveConfig("a-pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
		Ω(err).ShouldNot(HaveOccurred())
		savedPipeline, err := sqlDB.GetPipelineByName("a-pipeline-name")
		Ω(err).ShouldNot(HaveOccurred())

		_, err = sqlDB.SaveConfig("other-pipeline-name", otherPipelineConfig, 0, db.PipelineUnpaused)
		Ω(err).ShouldNot(HaveOccurred())
		otherSavedPipeline, err := sqlDB.GetPipelineByName("other-pipeline-name")
		Ω(err).ShouldNot(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
		otherPipelineDB = pipelineDBFactory.Build(otherSavedPipeline)
	})

	loadAndGetLatestInputVersions := func(jobName string, inputs []config.JobInput) ([]db.BuildInput, error) {
		versions, err := pipelineDB.LoadVersionsDB()
		if err != nil {
			return nil, err
		}

		return pipelineDB.GetLatestInputVersions(versions, jobName, inputs)
	}

	Describe("destroying a pipeline", func() {
		It("can be deleted", func() {
			_, err := sqlDB.SaveConfig("a-pipeline-that-will-be-deleted", pipelineConfig, 0, db.PipelineUnpaused)
			Ω(err).ShouldNot(HaveOccurred())

			fetchedPipeline, err := sqlDB.GetPipelineByName("a-pipeline-that-will-be-deleted")
			Ω(err).ShouldNot(HaveOccurred())

			fetchedPipelineDB := pipelineDBFactory.Build(fetchedPipeline)

			// resource tree
			_, err = fetchedPipelineDB.GetResource("some-resource")
			Ω(err).ShouldNot(HaveOccurred())

			resourceConfig, found := pipelineConfig.Resources.Lookup("some-resource")
			Ω(found).Should(BeTrue())

			fetchedPipelineDB.SaveResourceVersions(resourceConfig, []atc.Version{
				{
					"key": "value",
				},
			})

			// job tree
			_, err = fetchedPipelineDB.GetJob("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			build, err := fetchedPipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			_, err = fetchedPipelineDB.GetRunningBuildsBySerialGroup("some-job", []string{"serial-group"})
			Ω(err).ShouldNot(HaveOccurred())

			fetchedPipelineDB.SaveBuildInput(build.ID, db.BuildInput{
				Name: "build-input",
			})

			_, err = fetchedPipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
				Resource:     "some-resource",
				PipelineName: "a-pipeline-that-will-be-deleted",
			}, false)
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.SaveBuildEvent(build.ID, event.StartTask{})
			Ω(err).ShouldNot(HaveOccurred())

			err = fetchedPipelineDB.Destroy()
			Ω(err).ShouldNot(HaveOccurred())

			pipelines, err := sqlDB.GetAllActivePipelines()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(pipelines).ShouldNot(ContainElement(fetchedPipeline))

			_, _, err = fetchedPipelineDB.GetConfig()
			Ω(err).Should(Equal(db.ErrPipelineNotFound))
		})
	})

	Describe("Pausing and unpausing a pipeline", func() {
		It("starts out as unpaused", func() {
			pipeline, err := sqlDB.GetPipelineByName("a-pipeline-name")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(pipeline.Paused).Should(BeFalse())
		})

		It("can be paused", func() {
			err := pipelineDB.Pause()
			Ω(err).ShouldNot(HaveOccurred())

			pipelinePaused, err := pipelineDB.IsPaused()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(pipelinePaused).Should(BeTrue())

			otherPipelinePaused, err := otherPipelineDB.IsPaused()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherPipelinePaused).Should(BeFalse())
		})

		It("can be unpaused", func() {
			err := pipelineDB.Pause()
			Ω(err).ShouldNot(HaveOccurred())

			err = otherPipelineDB.Pause()
			Ω(err).ShouldNot(HaveOccurred())

			err = pipelineDB.Unpause()
			Ω(err).ShouldNot(HaveOccurred())

			pipelinePaused, err := pipelineDB.IsPaused()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(pipelinePaused).Should(BeFalse())

			otherPipelinePaused, err := otherPipelineDB.IsPaused()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherPipelinePaused).Should(BeTrue())
		})
	})

	Describe("ScopedName", func() {
		It("concatenates the pipeline name with the passed in name", func() {
			pipelineDB := pipelineDBFactory.Build(db.SavedPipeline{
				Pipeline: db.Pipeline{
					Name: "some-pipeline",
				},
			})
			Ω(pipelineDB.ScopedName("something-else")).Should(Equal("some-pipeline:something-else"))
		})
	})

	Describe("getting the pipeline configuration", func() {
		It("can manage multiple pipeline configurations", func() {
			By("returning the saved config to later gets")
			returnedConfig, configVersion, err := pipelineDB.GetConfig()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(returnedConfig).Should(Equal(pipelineConfig))
			Ω(configVersion).ShouldNot(Equal(db.ConfigVersion(0)))

			otherReturnedConfig, otherConfigVersion, err := otherPipelineDB.GetConfig()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherReturnedConfig).Should(Equal(otherPipelineConfig))
			Ω(otherConfigVersion).ShouldNot(Equal(db.ConfigVersion(0)))

			updatedConfig := pipelineConfig

			updatedConfig.Groups = append(pipelineConfig.Groups, atc.GroupConfig{
				Name: "new-group",
				Jobs: []string{"new-job-1", "new-job-2"},
			})

			updatedConfig.Resources = append(pipelineConfig.Resources, atc.ResourceConfig{
				Name: "new-resource",
				Type: "new-type",
				Source: atc.Source{
					"new-source-config": "new-value",
				},
			})

			updatedConfig.Jobs = append(pipelineConfig.Jobs, atc.JobConfig{
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

			By("being able to update the config with a valid config")
			_, err = sqlDB.SaveConfig("a-pipeline-name", updatedConfig, configVersion, db.PipelineUnpaused)
			Ω(err).ShouldNot(HaveOccurred())
			_, err = sqlDB.SaveConfig("other-pipeline-name", updatedConfig, otherConfigVersion, db.PipelineUnpaused)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning the updated config")
			returnedConfig, newConfigVersion, err := pipelineDB.GetConfig()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(returnedConfig).Should(Equal(updatedConfig))
			Ω(newConfigVersion).ShouldNot(Equal(configVersion))

			otherReturnedConfig, newOtherConfigVersion, err := otherPipelineDB.GetConfig()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherReturnedConfig).Should(Equal(updatedConfig))
			Ω(newOtherConfigVersion).ShouldNot(Equal(otherConfigVersion))
		})
	})

	Context("Resources", func() {
		resourceName := "some-resource"

		var resource db.SavedResource

		BeforeEach(func() {
			var err error
			resource, err = pipelineDB.GetResource("some-resource")
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("can load up versioned resource information relevant to scheduling", func() {
			versions, err := pipelineDB.LoadVersionsDB()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versions.ResourceVersions).Should(BeEmpty())
			Ω(versions.BuildOutputs).Should(BeEmpty())
			Ω(versions.ResourceIDs).Should(Equal(map[string]int{
				resource.Name: resource.ID,
			}))
			Ω(versions.JobIDs).Should(Equal(map[string]int{}))

			By("including saved versioned resources of the current pipeline")
			err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name,
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Ω(err).ShouldNot(HaveOccurred())

			savedVR1, err := pipelineDB.GetLatestVersionedResource(resource)
			Ω(err).ShouldNot(HaveOccurred())

			err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name,
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Ω(err).ShouldNot(HaveOccurred())

			savedVR2, err := pipelineDB.GetLatestVersionedResource(resource)
			Ω(err).ShouldNot(HaveOccurred())

			versions, err = pipelineDB.LoadVersionsDB()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versions.ResourceVersions).Should(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID},
				{VersionID: savedVR2.ID, ResourceID: resource.ID},
			}))
			Ω(versions.BuildOutputs).Should(BeEmpty())
			Ω(versions.ResourceIDs).Should(Equal(map[string]int{
				resource.Name: resource.ID,
			}))
			Ω(versions.JobIDs).Should(Equal(map[string]int{}))

			By("not including saved versioned resources of other pipelines")
			otherPipelineResource, err := otherPipelineDB.GetResource("some-other-pipeline-resource")
			Ω(err).ShouldNot(HaveOccurred())

			err = otherPipelineDB.SaveResourceVersions(atc.ResourceConfig{
				Name:   otherPipelineResource.Name,
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Ω(err).ShouldNot(HaveOccurred())

			otherPipelineSavedVR, err := otherPipelineDB.GetLatestVersionedResource(otherPipelineResource)
			Ω(err).ShouldNot(HaveOccurred())

			versions, err = pipelineDB.LoadVersionsDB()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versions.ResourceVersions).Should(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID},
				{VersionID: savedVR2.ID, ResourceID: resource.ID},
			}))
			Ω(versions.BuildOutputs).Should(BeEmpty())
			Ω(versions.ResourceIDs).Should(Equal(map[string]int{
				resource.Name: resource.ID,
			}))
			Ω(versions.JobIDs).Should(Equal(map[string]int{}))

			By("including outputs of successful builds")
			build1, err := pipelineDB.CreateJobBuild("a-job")
			Ω(err).ShouldNot(HaveOccurred())

			_, err = pipelineDB.SaveBuildOutput(build1.ID, savedVR1.VersionedResource, false)
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.FinishBuild(build1.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			versions, err = pipelineDB.LoadVersionsDB()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versions.ResourceVersions).Should(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID},
				{VersionID: savedVR2.ID, ResourceID: resource.ID},
			}))
			Ω(versions.BuildOutputs).Should(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID,
					},
					JobID:   build1.JobID,
					BuildID: build1.ID,
				},
			}))
			Ω(versions.ResourceIDs).Should(Equal(map[string]int{
				resource.Name: resource.ID,
			}))
			Ω(versions.JobIDs).Should(Equal(map[string]int{
				"a-job": build1.JobID,
			}))

			By("not including outputs of failed builds")
			build2, err := pipelineDB.CreateJobBuild("a-job")
			Ω(err).ShouldNot(HaveOccurred())

			_, err = pipelineDB.SaveBuildOutput(build2.ID, savedVR1.VersionedResource, false)
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.FinishBuild(build2.ID, db.StatusFailed)
			Ω(err).ShouldNot(HaveOccurred())

			versions, err = pipelineDB.LoadVersionsDB()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versions.ResourceVersions).Should(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID},
				{VersionID: savedVR2.ID, ResourceID: resource.ID},
			}))
			Ω(versions.BuildOutputs).Should(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID,
					},
					JobID:   build1.JobID,
					BuildID: build1.ID,
				},
			}))
			Ω(versions.ResourceIDs).Should(Equal(map[string]int{
				resource.Name: resource.ID,
			}))
			Ω(versions.JobIDs).Should(Equal(map[string]int{
				"a-job": build1.JobID,
			}))

			By("not including outputs of builds in other pipelines")
			otherPipelineBuild, err := otherPipelineDB.CreateJobBuild("a-job")
			Ω(err).ShouldNot(HaveOccurred())

			_, err = otherPipelineDB.SaveBuildOutput(otherPipelineBuild.ID, otherPipelineSavedVR.VersionedResource, false)
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.FinishBuild(otherPipelineBuild.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			versions, err = pipelineDB.LoadVersionsDB()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versions.ResourceVersions).Should(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID},
				{VersionID: savedVR2.ID, ResourceID: resource.ID},
			}))
			Ω(versions.BuildOutputs).Should(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID,
					},
					JobID:   build1.JobID,
					BuildID: build1.ID,
				},
			}))
			Ω(versions.ResourceIDs).Should(Equal(map[string]int{
				resource.Name: resource.ID,
			}))
			Ω(versions.JobIDs).Should(Equal(map[string]int{
				"a-job": build1.JobID,
			}))
		})

		Describe("pausing and unpausing resources", func() {
			It("starts out as unpaused", func() {
				resource, err := pipelineDB.GetResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(resource.Paused).Should(BeFalse())
			})

			It("can be paused", func() {
				err := pipelineDB.PauseResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())

				pausedResource, err := pipelineDB.GetResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(pausedResource.Paused).Should(BeTrue())

				resource, err := otherPipelineDB.GetResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resource.Paused).Should(BeFalse())
			})

			It("can be unpaused", func() {
				err := pipelineDB.PauseResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())

				err = otherPipelineDB.PauseResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.UnpauseResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())

				unpausedResource, err := pipelineDB.GetResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(unpausedResource.Paused).Should(BeFalse())

				resource, err := otherPipelineDB.GetResource(resourceName)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resource.Paused).Should(BeTrue())
			})
		})

		Describe("enabling and disabling versioned resources", func() {
			It("returns an error if the resource or version is bogus", func() {
				err := pipelineDB.EnableVersionedResource(42)
				Ω(err).Should(HaveOccurred())

				err = pipelineDB.DisableVersionedResource(42)
				Ω(err).Should(HaveOccurred())
			})

			It("does not affect explicitly fetching the latest version", func() {
				err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(savedVR.VersionedResource).Should(Equal(db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  db.Version{"version": "1"},
				}))

				err = pipelineDB.DisableVersionedResource(savedVR.ID)
				Ω(err).ShouldNot(HaveOccurred())

				disabledVR := savedVR
				disabledVR.Enabled = false

				Ω(pipelineDB.GetLatestVersionedResource(resource)).Should(Equal(disabledVR))

				err = pipelineDB.EnableVersionedResource(savedVR.ID)
				Ω(err).ShouldNot(HaveOccurred())

				enabledVR := savedVR
				enabledVR.Enabled = true

				Ω(pipelineDB.GetLatestVersionedResource(resource)).Should(Equal(enabledVR))
			})

			It("prevents the resource version from being eligible as a previous set of inputs", func() {
				err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR1, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				otherResource, err := pipelineDB.GetResource("some-other-resource")
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Ω(err).ShouldNot(HaveOccurred())

				otherSavedVR1, err := pipelineDB.GetLatestVersionedResource(otherResource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR2, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}})
				Ω(err).ShouldNot(HaveOccurred())

				otherSavedVR2, err := pipelineDB.GetLatestVersionedResource(otherResource)
				Ω(err).ShouldNot(HaveOccurred())

				jobBuildInputs := []config.JobInput{
					{
						Name:     "some-input-name",
						Resource: "some-resource",
					},
					{
						Name:     "some-other-input-name",
						Resource: "some-other-resource",
					},
				}

				build1, err := pipelineDB.CreateJobBuild("a-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build1.ID, db.BuildInput{
					Name:              "some-input-name",
					VersionedResource: savedVR1.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build1.ID, db.BuildInput{
					Name:              "some-other-input-name",
					VersionedResource: otherSavedVR1.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				build2, err := pipelineDB.CreateJobBuild("a-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build2.ID, db.BuildInput{
					Name:              "some-input-name",
					VersionedResource: savedVR2.VersionedResource,
				})

				Ω(err).ShouldNot(HaveOccurred())
				_, err = pipelineDB.SaveBuildInput(build2.ID, db.BuildInput{
					Name:              "some-other-input-name",
					VersionedResource: otherSavedVR2.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.DisableVersionedResource(savedVR2.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", jobBuildInputs)).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "some-input-name",
						VersionedResource: savedVR1.VersionedResource,
					},
					{
						Name:              "some-other-input-name",
						VersionedResource: otherSavedVR2.VersionedResource,
					},
				}))
			})

			It("prevents the resource version from being a candidate for build inputs", func() {
				err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR1, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR2, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				jobBuildInputs := []config.JobInput{
					{
						Name:     "some-input-name",
						Resource: "some-resource",
					},
				}

				Ω(loadAndGetLatestInputVersions("a-job", jobBuildInputs)).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "some-input-name",
						VersionedResource: savedVR2.VersionedResource,
					},
				}))

				err = pipelineDB.DisableVersionedResource(savedVR2.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", jobBuildInputs)).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "some-input-name",
						VersionedResource: savedVR1.VersionedResource,
					},
				}))

				err = pipelineDB.DisableVersionedResource(savedVR1.ID)
				Ω(err).ShouldNot(HaveOccurred())

				// no versions
				_, err = loadAndGetLatestInputVersions("a-job", jobBuildInputs)
				Ω(err).Should(HaveOccurred())

				err = pipelineDB.EnableVersionedResource(savedVR1.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", jobBuildInputs)).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "some-input-name",
						VersionedResource: savedVR1.VersionedResource,
					},
				}))

				err = pipelineDB.EnableVersionedResource(savedVR2.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", jobBuildInputs)).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "some-input-name",
						VersionedResource: savedVR2.VersionedResource,
					},
				}))
			})
		})

		Describe("saving versioned resources", func() {
			It("updates the latest versioned resource", func() {
				err := pipelineDB.SaveResourceVersions(
					atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					[]atc.Version{{"version": "1"}},
				)
				Ω(err).ShouldNot(HaveOccurred())

				savedResource, err := pipelineDB.GetResource("some-resource")
				Ω(err).ShouldNot(HaveOccurred())

				savedVR, err := pipelineDB.GetLatestVersionedResource(savedResource)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(savedVR.VersionedResource).Should(Equal(db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  db.Version{"version": "1"},
				}))

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}, {"version": "3"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR, err = pipelineDB.GetLatestVersionedResource(savedResource)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(savedVR.VersionedResource).Should(Equal(db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  db.Version{"version": "3"},
				}))
			})
		})

		It("initially reports zero builds for a job", func() {
			builds, err := pipelineDB.GetAllJobBuilds("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(builds).Should(BeEmpty())
		})

		It("initially has no current build for a job", func() {
			_, err := pipelineDB.GetCurrentBuild("some-job")
			Ω(err).Should(Equal(db.ErrNoBuild))
		})

		It("initially has no pending build for a job", func() {
			_, err := pipelineDB.GetNextPendingBuild("some-job")
			Ω(err).Should(Equal(db.ErrNoBuild))
		})

		Describe("marking resource checks as errored", func() {
			var resource db.SavedResource

			BeforeEach(func() {
				var err error
				resource, err = pipelineDB.GetResource("resource-name")
				Ω(err).ShouldNot(HaveOccurred())
			})

			Context("when the resource is first created", func() {
				It("is not errored", func() {
					Ω(resource.CheckError).Should(BeNil())
				})
			})

			Context("when a resource check is marked as errored", func() {
				It("is then marked as errored", func() {
					originalCause := errors.New("on fire")

					err := pipelineDB.SetResourceCheckError(resource, originalCause)
					Ω(err).ShouldNot(HaveOccurred())

					returnedResource, err := pipelineDB.GetResource("resource-name")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returnedResource.CheckError).Should(Equal(originalCause))
				})
			})

			Context("when a resource is cleared of check errors", func() {
				It("is not marked as errored again", func() {
					originalCause := errors.New("on fire")

					err := pipelineDB.SetResourceCheckError(resource, originalCause)
					Ω(err).ShouldNot(HaveOccurred())

					err = pipelineDB.SetResourceCheckError(resource, nil)
					Ω(err).ShouldNot(HaveOccurred())

					returnedResource, err := pipelineDB.GetResource("resource-name")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returnedResource.CheckError).Should(BeNil())
				})
			})
		})

		Describe("GetResourceHistoryMaxID", func() {
			BeforeEach(func() {
				for i := 0; i < 10; i++ {
					err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": i}})
					Ω(err).ShouldNot(HaveOccurred())
				}

				for i := 0; i < 5; i++ {
					err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
						Name:   "other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": i}})
					Ω(err).ShouldNot(HaveOccurred())
				}

				err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "another-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{})
				Ω(err).ShouldNot(HaveOccurred())
			})

			Context("when the resource that doesn't have any versions", func() {
				It("returns 0", func() {
					savedResource, err := pipelineDB.GetResource("another-resource")
					Ω(err).ShouldNot(HaveOccurred())
					id, err := pipelineDB.GetResourceHistoryMaxID(savedResource.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(id).Should(Equal(0))
				})
			})

			Context("when the resource exists and has versions", func() {
				It("gets the max version id for the given resource", func() {
					savedResource, err := pipelineDB.GetResource("some-resource")
					Ω(err).ShouldNot(HaveOccurred())
					maxID, err := pipelineDB.GetResourceHistoryMaxID(savedResource.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(maxID).Should(Equal(10))

					savedResource, err = pipelineDB.GetResource("other-resource")
					Ω(err).ShouldNot(HaveOccurred())
					maxID, err = pipelineDB.GetResourceHistoryMaxID(savedResource.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(maxID).Should(Equal(15))
				})
			})
		})

		Describe("GetResourceHistoryCursor", func() {
			BeforeEach(func() {
				for i := 1; i <= 20; i++ {
					err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": i}})
					Ω(err).ShouldNot(HaveOccurred())
				}
				for i := 1; i <= 10; i++ {
					err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
						Name:   "other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": i}})
					Ω(err).ShouldNot(HaveOccurred())
				}
				for i := 21; i <= 30; i++ {
					err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": i}})
					Ω(err).ShouldNot(HaveOccurred())
				}

				err := pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "another-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{})
				Ω(err).ShouldNot(HaveOccurred())
			})

			Context("when greaterThanStartingID is false", func() {
				greaterThanStartingID := false

				Context("when numResults is greater than the number of versions to return", func() {
					It("should return a slice of the VersionHistorys in descending order from the startingID", func() {
						versionHistories, hasNext, err := pipelineDB.GetResourceHistoryCursor("some-resource", 35, greaterThanStartingID, 50)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(hasNext).Should(BeFalse())
						numResults := len(versionHistories)
						Ω(numResults).Should(Equal(25))
						firstID := versionHistories[0].VersionedResource.ID
						Ω(firstID).Should(Equal(35))
						Ω(firstID).Should(BeNumerically(">", versionHistories[numResults-1].VersionedResource.ID))
					})
				})

				Context("when numResults is less than the number of versions to return", func() {
					It("should return a limited slice of the VersionHistories", func() {
						versionHistories, hasNext, err := pipelineDB.GetResourceHistoryCursor("some-resource", 30, greaterThanStartingID, 17)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(len(versionHistories)).Should(Equal(17))
						Ω(hasNext).Should(BeTrue())
					})
				})

				Context("when numResults is 0", func() {
					It("returns all the rows", func() {
						versionHistories, hasNext, err := pipelineDB.GetResourceHistoryCursor("some-resource", 100, greaterThanStartingID, 0)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(len(versionHistories)).Should(Equal(30))
						Ω(hasNext).Should(BeFalse())
					})
				})
			})

			Context("when greaterThanStartingID is true", func() {
				greaterThanStartingID := true

				Context("when numResults is greater than the number of versions to return", func() {
					It("should return a slice of the VersionHistorys in descending order from that are greater then or equal to the startingID", func() {
						versionHistories, hasNext, err := pipelineDB.GetResourceHistoryCursor("some-resource", 15, greaterThanStartingID, 7)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(hasNext).Should(BeTrue())

						numResults := len(versionHistories)
						Ω(numResults).Should(Equal(7))

						lastID := versionHistories[numResults-1].VersionedResource.ID
						Ω(lastID).Should(Equal(15))

						firstID := versionHistories[0].VersionedResource.ID
						Ω(firstID).Should(Equal(31))

						Ω(firstID).Should(BeNumerically(">", lastID))
					})
				})

				Context("when numResults is 0", func() {
					It("returns all the rows", func() {
						versionHistories, hasNext, err := pipelineDB.GetResourceHistoryCursor("some-resource", 0, greaterThanStartingID, 0)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(len(versionHistories)).Should(Equal(30))
						Ω(hasNext).Should(BeFalse())
					})
				})
			})
		})
	})

	Context("Jobs", func() {
		Describe("CreateJobBuild", func() {
			var build db.Build

			BeforeEach(func() {
				var err error
				build, err = pipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("sets the properties of a build for a given job", func() {
				Ω(build.ID).ShouldNot(BeZero())
				Ω(build.JobID).ShouldNot(BeZero())
				Ω(build.Name).Should(Equal("1"))
				Ω(build.Status).Should(Equal(db.StatusPending))
				Ω(build.Scheduled).Should(BeFalse())
			})
		})

		Describe("saving builds for scheduling", func() {
			buildMetadata := []db.MetadataField{
				{
					Name:  "meta1",
					Value: "value1",
				},
				{
					Name:  "meta2",
					Value: "value2",
				},
			}

			vr1 := db.VersionedResource{
				PipelineName: "a-pipeline-name",
				Resource:     "some-resource",
				Type:         "some-type",
				Version:      db.Version{"ver": "1"},
				Metadata:     buildMetadata,
			}

			vr2 := db.VersionedResource{
				PipelineName: "a-pipeline-name",
				Resource:     "some-other-resource",
				Type:         "some-type",
				Version:      db.Version{"ver": "2"},
			}

			input1 := db.BuildInput{
				Name:              "some-input",
				VersionedResource: vr1,
			}

			input2 := db.BuildInput{
				Name:              "some-other-input",
				VersionedResource: vr2,
			}

			inputs := []db.BuildInput{input1, input2}

			It("does not create a new build if one is already running that does not have determined inputs ", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				Ω(build.ID).ShouldNot(BeZero())
				Ω(build.JobID).ShouldNot(BeZero())
				Ω(build.Name).Should(Equal("1"))
				Ω(build.Status).Should(Equal(db.StatusPending))
				Ω(build.Scheduled).Should(BeFalse())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeFalse())
			})

			It("does create a new build if one does not have determined inputs but it has a different name", func() {
				_, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-other-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("does create a new build if one does not have determined inputs but in a different pipeline", func() {
				_, err := otherPipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("does create a new build if one is already saved but it has already locked down its inputs", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				err = pipelineDB.UseInputsForBuild(build.ID, inputs)
				Ω(err).ShouldNot(HaveOccurred())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("does create a new build if one is already saved but does not have determined inputs but is not running (errored)", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				err = sqlDB.ErrorBuild(build.ID, errors.New("disaster"))
				Ω(err).ShouldNot(HaveOccurred())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("does create a new build if one is already saved but does not have determined inputs but is not running (aborted)", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				err = sqlDB.AbortBuild(build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("does create a new build if one is already saved but does not have determined inputs but is not running (succeeded)", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				err = sqlDB.FinishBuild(build.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("does create a new build if one is already saved but does not have determined inputs but is not running (failed)", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				err = sqlDB.FinishBuild(build.ID, db.StatusFailed)
				Ω(err).ShouldNot(HaveOccurred())

				_, created, err = pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())
			})

			It("saves all the build inputs", func() {
				build, created, err := pipelineDB.CreateJobBuildForCandidateInputs("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(created).Should(BeTrue())

				err = pipelineDB.UseInputsForBuild(build.ID, inputs)
				Ω(err).ShouldNot(HaveOccurred())

				foundBuild, err := pipelineDB.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(foundBuild).Should(Equal(build))
			})
		})

		Describe("saving build inputs", func() {
			buildMetadata := []db.MetadataField{
				{
					Name:  "meta1",
					Value: "value1",
				},
				{
					Name:  "meta2",
					Value: "value2",
				},
			}

			vr1 := db.VersionedResource{
				PipelineName: "a-pipeline-name",
				Resource:     "some-resource",
				Type:         "some-type",
				Version:      db.Version{"ver": "1"},
				Metadata:     buildMetadata,
			}

			vr2 := db.VersionedResource{
				PipelineName: "a-pipeline-name",
				Resource:     "some-other-resource",
				Type:         "some-type",
				Version:      db.Version{"ver": "2"},
			}

			It("saves build's inputs and outputs as versioned resources", func() {
				build, err := pipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				input1 := db.BuildInput{
					Name:              "some-input",
					VersionedResource: vr1,
				}

				input2 := db.BuildInput{
					Name:              "some-other-input",
					VersionedResource: vr2,
				}

				otherInput := db.BuildInput{
					Name:              "some-random-input",
					VersionedResource: vr2,
				}

				_, err = sqlDB.SaveBuildInput(build.ID, input1)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).Should(Equal(db.ErrNoBuild))

				_, err = sqlDB.SaveBuildInput(build.ID, otherInput)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).Should(Equal(db.ErrNoBuild))

				_, err = sqlDB.SaveBuildInput(build.ID, input2)
				Ω(err).ShouldNot(HaveOccurred())

				foundBuild, err := pipelineDB.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(foundBuild).Should(Equal(build))

				modifiedVR2 := vr2
				modifiedVR2.Version = db.Version{"ver": "3"}

				inputs, _, err := pipelineDB.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
					{Name: "some-other-input", VersionedResource: vr2, FirstOccurrence: true},
					{Name: "some-random-input", VersionedResource: vr2, FirstOccurrence: true},
				}))

				duplicateBuild, err := pipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = sqlDB.SaveBuildInput(duplicateBuild.ID, db.BuildInput{
					Name:              "other-build-input",
					VersionedResource: vr1,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = sqlDB.SaveBuildInput(duplicateBuild.ID, db.BuildInput{
					Name:              "other-build-other-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = pipelineDB.GetBuildResources(duplicateBuild.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "other-build-input", VersionedResource: vr1, FirstOccurrence: false},
					{Name: "other-build-other-input", VersionedResource: vr2, FirstOccurrence: false},
				}))

				newBuildInOtherJob, err := pipelineDB.CreateJobBuild("some-other-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = sqlDB.SaveBuildInput(newBuildInOtherJob.ID, db.BuildInput{
					Name:              "other-job-input",
					VersionedResource: vr1,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = sqlDB.SaveBuildInput(newBuildInOtherJob.ID, db.BuildInput{
					Name:              "other-job-other-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = pipelineDB.GetBuildResources(newBuildInOtherJob.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "other-job-input", VersionedResource: vr1, FirstOccurrence: true},
					{Name: "other-job-other-input", VersionedResource: vr2, FirstOccurrence: true},
				}))
			})

			It("updates metadata of existing inputs resources", func() {
				build, err := pipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = sqlDB.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err := pipelineDB.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: vr2, FirstOccurrence: true},
				}))

				withMetadata := vr2
				withMetadata.Metadata = buildMetadata

				_, err = sqlDB.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-other-input",
					VersionedResource: withMetadata,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = pipelineDB.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
					{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
				}))

				_, err = sqlDB.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-input",
					VersionedResource: withMetadata,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = pipelineDB.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
					{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
				}))
			})
		})

		Describe("saving inputs, implicit outputs, and explicit outputs", func() {
			vr1 := db.VersionedResource{
				PipelineName: "a-pipeline-name",
				Resource:     "some-resource",
				Type:         "some-type",
				Version:      db.Version{"ver": "1"},
			}

			vr2 := db.VersionedResource{
				PipelineName: "a-pipeline-name",
				Resource:     "some-other-resource",
				Type:         "some-type",
				Version:      db.Version{"ver": "2"},
			}

			It("correctly distinguishes them", func() {
				build, err := pipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				// save a normal 'get'
				_, err = sqlDB.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-input",
					VersionedResource: vr1,
				})
				Ω(err).ShouldNot(HaveOccurred())

				// save implicit output from 'get'
				_, err = sqlDB.SaveBuildOutput(build.ID, vr1, false)
				Ω(err).ShouldNot(HaveOccurred())

				// save explicit output from 'put'
				_, err = sqlDB.SaveBuildOutput(build.ID, vr2, true)
				Ω(err).ShouldNot(HaveOccurred())

				// save the dependent get
				_, err = sqlDB.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-dependent-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				// save the dependent 'get's implicit output
				_, err = sqlDB.SaveBuildOutput(build.ID, vr2, false)
				Ω(err).ShouldNot(HaveOccurred())

				inputs, outputs, err := pipelineDB.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
				}))
				Ω(outputs).Should(ConsistOf([]db.BuildOutput{
					{VersionedResource: vr2},
				}))
			})
		})

		Describe("pausing and unpausing jobs", func() {
			job := "some-job"

			It("starts out as unpaused", func() {
				job, err := pipelineDB.GetJob(job)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(job.Paused).Should(BeFalse())
			})

			It("can be paused", func() {
				err := pipelineDB.PauseJob(job)
				Ω(err).ShouldNot(HaveOccurred())

				err = otherPipelineDB.UnpauseJob(job)
				Ω(err).ShouldNot(HaveOccurred())

				pausedJob, err := pipelineDB.GetJob(job)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(pausedJob.Paused).Should(BeTrue())

				otherJob, err := otherPipelineDB.GetJob(job)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(otherJob.Paused).Should(BeFalse())
			})

			It("can be unpaused", func() {
				err := pipelineDB.PauseJob(job)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.UnpauseJob(job)
				Ω(err).ShouldNot(HaveOccurred())

				unpausedJob, err := pipelineDB.GetJob(job)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(unpausedJob.Paused).Should(BeFalse())
			})
		})

		Context("when the first build is created", func() {
			var firstBuild db.Build

			var job db.SavedJob
			var jobConfig atc.JobConfig
			var serialJobConfig atc.JobConfig

			BeforeEach(func() {
				var err error

				job, err = pipelineDB.GetJob("some-job")
				jobConfig = atc.JobConfig{
					Name:   "some-job",
					Serial: false,
				}
				serialJobConfig = atc.JobConfig{
					Name:   "some-job",
					Serial: true,
				}

				Ω(err).ShouldNot(HaveOccurred())

				firstBuild, err = pipelineDB.CreateJobBuild(job.Name)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(firstBuild.Name).Should(Equal("1"))
				Ω(firstBuild.Status).Should(Equal(db.StatusPending))
			})

			Context("and the pipeline is paused", func() {
				BeforeEach(func() {
					err := pipelineDB.Pause()
					Ω(err).ShouldNot(HaveOccurred())
				})

				Describe("scheduling the build", func() {
					It("fails", func() {
						scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Context("and then errored", func() {
				BeforeEach(func() {
					cause := errors.New("everything is broken")
					err := sqlDB.ErrorBuild(firstBuild.ID, cause)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("changes the state to errored", func() {
					build, err := pipelineDB.GetJobBuild(job.Name, firstBuild.Name)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(db.StatusErrored))
				})

				It("saves off the error for later debugging", func() {
					eventStream, err := sqlDB.GetBuildEvents(firstBuild.ID, 0)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(eventStream.Next()).Should(Equal(event.Error{
						Message: "everything is broken",
					}))
				})

				Describe("scheduling the build", func() {
					It("fails", func() {
						scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Context("and then aborted", func() {
				BeforeEach(func() {
					err := sqlDB.FinishBuild(firstBuild.ID, db.StatusAborted)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("changes the state to aborted", func() {
					build, err := pipelineDB.GetJobBuild(job.Name, firstBuild.Name)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(db.StatusAborted))
				})

				Describe("scheduling the build", func() {
					It("fails", func() {
						scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Context("when the job is paused", func() {
				BeforeEach(func() {
					err := pipelineDB.PauseJob(job.Name)
					Ω(err).ShouldNot(HaveOccurred())
				})

				Describe("scheduling the build", func() {
					It("fails", func() {
						scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Context("and then scheduled", func() {
				BeforeEach(func() {
					scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("and then aborted", func() {
					BeforeEach(func() {
						err := sqlDB.FinishBuild(firstBuild.ID, db.StatusAborted)
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("changes the state to aborted", func() {
						build, err := pipelineDB.GetJobBuild(job.Name, firstBuild.Name)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build.Status).Should(Equal(db.StatusAborted))
					})

					Describe("starting the build", func() {
						It("fails", func() {
							started, err := sqlDB.StartBuild(firstBuild.ID, "some-engine", "some-meta")
							Ω(err).ShouldNot(HaveOccurred())
							Ω(started).Should(BeFalse())
						})
					})
				})
			})

			Describe("scheduling the build", func() {
				It("succeeds", func() {
					scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Describe("twice", func() {
					It("succeeds idempotently", func() {
						scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())

						scheduled, err = pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})
				})

				Context("serially", func() {
					It("succeeds", func() {
						scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, serialJobConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Describe("twice", func() {
						It("succeeds idempotently", func() {
							scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, serialJobConfig)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())

							scheduled, err = pipelineDB.ScheduleBuild(firstBuild.ID, serialJobConfig)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})
					})
				})
			})

			Context("and a second build is created", func() {
				var secondBuild db.Build

				Context("for a different job", func() {
					BeforeEach(func() {
						var err error
						jobConfig.Name = "some-other-job"
						serialJobConfig.Name = "some-other-job"

						secondBuild, err = pipelineDB.CreateJobBuild("some-other-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(secondBuild.Name).Should(Equal("1"))
						Ω(secondBuild.Status).Should(Equal(db.StatusPending))
					})

					Describe("scheduling the second build", func() {
						It("succeeds", func() {
							scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, jobConfig)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Describe("serially", func() {
							It("succeeds", func() {
								scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, serialJobConfig)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})
						})
					})
				})

				Context("for the same job", func() {
					BeforeEach(func() {
						var err error

						secondBuild, err = pipelineDB.CreateJobBuild(job.Name)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(secondBuild.Name).Should(Equal("2"))
						Ω(secondBuild.Status).Should(Equal(db.StatusPending))
					})

					Describe("scheduling the second build", func() {
						It("succeeds", func() {
							scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, jobConfig)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Describe("serially", func() {
							It("fails", func() {
								scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, serialJobConfig)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})
					})

					Describe("after the first build schedules", func() {
						BeforeEach(func() {
							scheduled, err := pipelineDB.ScheduleBuild(firstBuild.ID, jobConfig)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Context("when the second build is scheduled serially", func() {
							It("fails", func() {
								scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, serialJobConfig)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})

						for _, s := range []db.Status{db.StatusSucceeded, db.StatusFailed, db.StatusErrored} {
							status := s

							Context("and the first build's status changes to "+string(status), func() {
								BeforeEach(func() {
									err := sqlDB.FinishBuild(firstBuild.ID, status)
									Ω(err).ShouldNot(HaveOccurred())
								})

								Context("and the second build is scheduled serially", func() {
									It("succeeds", func() {
										scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, serialJobConfig)
										Ω(err).ShouldNot(HaveOccurred())
										Ω(scheduled).Should(BeTrue())
									})
								})
							})
						}
					})

					Describe("after the first build is aborted", func() {
						BeforeEach(func() {
							err := sqlDB.FinishBuild(firstBuild.ID, db.StatusAborted)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("when the second build is scheduled serially", func() {
							It("succeeds", func() {
								scheduled, err := pipelineDB.ScheduleBuild(secondBuild.ID, serialJobConfig)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})
						})
					})

					Context("and a third build is created", func() {
						var thirdBuild db.Build

						BeforeEach(func() {
							var err error

							thirdBuild, err = pipelineDB.CreateJobBuild(job.Name)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(thirdBuild.Name).Should(Equal("3"))
							Ω(thirdBuild.Status).Should(Equal(db.StatusPending))
						})

						Context("and the first build finishes", func() {
							BeforeEach(func() {
								err := sqlDB.FinishBuild(firstBuild.ID, db.StatusSucceeded)
								Ω(err).ShouldNot(HaveOccurred())
							})

							Context("and the third build is scheduled serially", func() {
								It("fails, as it would have jumped the queue", func() {
									scheduled, err := pipelineDB.ScheduleBuild(thirdBuild.ID, serialJobConfig)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(scheduled).Should(BeFalse())
								})
							})
						})

						Context("and then scheduled", func() {
							It("succeeds", func() {
								scheduled, err := pipelineDB.ScheduleBuild(thirdBuild.ID, jobConfig)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})

							Describe("serially", func() {
								It("fails", func() {
									scheduled, err := pipelineDB.ScheduleBuild(thirdBuild.ID, serialJobConfig)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(scheduled).Should(BeFalse())
								})
							})
						})
					})
				})
			})
		})

		Describe("GetNextPendingBuildBySerialGroup", func() {
			var jobOneConfig atc.JobConfig
			var jobOneTwoConfig atc.JobConfig
			BeforeEach(func() {
				jobOneConfig = atc.JobConfig{
					Name:         "job-one",
					SerialGroups: []string{"one"},
				}
				jobOneTwoConfig = atc.JobConfig{
					Name:         "job-one-two",
					SerialGroups: []string{"one", "two"},
				}
			})

			It("should return the next most pending build in a group of jobs", func() {
				buildOne, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Ω(err).ShouldNot(HaveOccurred())

				buildTwo, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Ω(err).ShouldNot(HaveOccurred())

				buildThree, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
				Ω(err).ShouldNot(HaveOccurred())

				otherBuildOne, err := otherPipelineDB.CreateJobBuild(jobOneConfig.Name)
				Ω(err).ShouldNot(HaveOccurred())

				otherBuildTwo, err := otherPipelineDB.CreateJobBuild(jobOneConfig.Name)
				Ω(err).ShouldNot(HaveOccurred())

				otherBuildThree, err := otherPipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
				Ω(err).ShouldNot(HaveOccurred())

				build, err := pipelineDB.GetNextPendingBuildBySerialGroup("job-one", []string{"one"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(buildOne.ID))
				build, err = pipelineDB.GetNextPendingBuildBySerialGroup("job-one-two", []string{"one", "two"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(buildOne.ID))

				scheduled, err := pipelineDB.ScheduleBuild(buildOne.ID, jobOneConfig)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
				Ω(sqlDB.FinishBuild(buildOne.ID, db.StatusSucceeded)).Should(Succeed())

				build, err = pipelineDB.GetNextPendingBuildBySerialGroup("job-one", []string{"one"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(buildTwo.ID))
				build, err = pipelineDB.GetNextPendingBuildBySerialGroup("job-one-two", []string{"one", "two"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(buildTwo.ID))

				scheduled, err = pipelineDB.ScheduleBuild(buildTwo.ID, jobOneConfig)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
				Ω(sqlDB.FinishBuild(buildTwo.ID, db.StatusSucceeded)).Should(Succeed())

				build, err = otherPipelineDB.GetNextPendingBuildBySerialGroup("job-one", []string{"one"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(otherBuildOne.ID))
				build, err = otherPipelineDB.GetNextPendingBuildBySerialGroup("job-one-two", []string{"one", "two"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(otherBuildOne.ID))

				scheduled, err = otherPipelineDB.ScheduleBuild(otherBuildOne.ID, jobOneConfig)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
				Ω(sqlDB.FinishBuild(otherBuildOne.ID, db.StatusSucceeded)).Should(Succeed())

				build, err = otherPipelineDB.GetNextPendingBuildBySerialGroup("job-one", []string{"one"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(otherBuildTwo.ID))
				build, err = otherPipelineDB.GetNextPendingBuildBySerialGroup("job-one-two", []string{"one", "two"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(otherBuildTwo.ID))

				scheduled, err = otherPipelineDB.ScheduleBuild(otherBuildTwo.ID, jobOneConfig)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
				Ω(sqlDB.FinishBuild(otherBuildTwo.ID, db.StatusSucceeded)).Should(Succeed())

				build, err = otherPipelineDB.GetNextPendingBuildBySerialGroup("job-one", []string{"one"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(otherBuildThree.ID))
				build, err = otherPipelineDB.GetNextPendingBuildBySerialGroup("job-one-two", []string{"one", "two"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(otherBuildThree.ID))

				build, err = pipelineDB.GetNextPendingBuildBySerialGroup("job-one", []string{"one"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(buildThree.ID))
				build, err = pipelineDB.GetNextPendingBuildBySerialGroup("job-one-two", []string{"one", "two"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(buildThree.ID))
			})
		})

		Describe("GetRunningBuildsBySerialGroup", func() {
			var startedBuild db.Build
			var scheduledBuild db.Build

			BeforeEach(func() {
				var err error
				_, err = pipelineDB.CreateJobBuild("matching-job")
				Ω(err).ShouldNot(HaveOccurred())

				startedBuild, err = pipelineDB.CreateJobBuild("matching-job")
				Ω(err).ShouldNot(HaveOccurred())
				_, err = sqlDB.StartBuild(startedBuild.ID, "", "")
				Ω(err).ShouldNot(HaveOccurred())

				scheduledBuild, err = pipelineDB.CreateJobBuild("matching-job")
				Ω(err).ShouldNot(HaveOccurred())

				scheduled, err := pipelineDB.ScheduleBuild(scheduledBuild.ID, atc.JobConfig{Name: "matching-job"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())

				_, err = pipelineDB.CreateJobBuild("not-matching-job")
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns a list of builds the matches the jobName passed in that are started or scheduled and have a different serial group", func() {
				builds, err := pipelineDB.GetRunningBuildsBySerialGroup("matching-job", []string{"matching-job"})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(len(builds)).Should(Equal(2))
			})
		})

		Context("when a build is created for a job", func() {
			var build1 db.Build
			var jobConfig atc.JobConfig

			BeforeEach(func() {
				var err error
				build1, err = pipelineDB.CreateJobBuild("some-job")

				jobConfig = atc.JobConfig{
					Serial: false,
				}
				Ω(err).ShouldNot(HaveOccurred())

				dbJob, err := pipelineDB.GetJob("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(build1.ID).ShouldNot(BeZero())
				Ω(build1.JobID).Should(Equal(dbJob.ID))
				Ω(build1.JobName).Should(Equal("some-job"))
				Ω(build1.Name).Should(Equal("1"))
				Ω(build1.Status).Should(Equal(db.StatusPending))
				Ω(build1.Scheduled).Should(BeFalse())
			})

			It("can be read back as the same object", func() {
				gotBuild, err := sqlDB.GetBuild(build1.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(gotBuild).Should(Equal(build1))
			})

			It("becomes the current build", func() {
				currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentBuild).Should(Equal(build1))
			})

			It("becomes the next pending build", func() {
				nextPending, err := pipelineDB.GetNextPendingBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(nextPending).Should(Equal(build1))
			})

			It("is returned in the job's builds", func() {
				Ω(pipelineDB.GetAllJobBuilds("some-job")).Should(ConsistOf([]db.Build{build1}))
			})

			Context("and another build for a different pipeline is created with the same job name", func() {
				BeforeEach(func() {
					otherBuild, err := otherPipelineDB.CreateJobBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					dbJob, err := otherPipelineDB.GetJob("some-job")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(otherBuild.ID).ShouldNot(BeZero())
					Ω(otherBuild.JobID).Should(Equal(dbJob.ID))
					Ω(otherBuild.JobName).Should(Equal("some-job"))
					Ω(otherBuild.Name).Should(Equal("1"))
					Ω(otherBuild.Status).Should(Equal(db.StatusPending))
					Ω(otherBuild.Scheduled).Should(BeFalse())
				})

				It("does not change the current build", func() {
					currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild).Should(Equal(build1))
				})

				It("does not change the next pending build", func() {
					nextPending, err := pipelineDB.GetNextPendingBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(nextPending).Should(Equal(build1))
				})

				It("is not returned in the job's builds", func() {
					Ω(pipelineDB.GetAllJobBuilds("some-job")).Should(ConsistOf([]db.Build{build1}))
				})
			})

			Context("when scheduled", func() {
				BeforeEach(func() {
					scheduled, err := pipelineDB.ScheduleBuild(build1.ID, jobConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
					build1.Scheduled = true
				})

				It("remains the current build", func() {
					currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild).Should(Equal(build1))
				})

				It("remains the next pending build", func() {
					nextPending, err := pipelineDB.GetNextPendingBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(nextPending).Should(Equal(build1))
				})
			})

			Context("when started", func() {
				BeforeEach(func() {
					started, err := sqlDB.StartBuild(build1.ID, "some-engine", "some-metadata")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(started).Should(BeTrue())
				})

				It("saves the updated status, and the engine and engine metadata", func() {
					currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild.Status).Should(Equal(db.StatusStarted))
					Ω(currentBuild.Engine).Should(Equal("some-engine"))
					Ω(currentBuild.EngineMetadata).Should(Equal("some-metadata"))
				})

				It("saves the build's start time", func() {
					currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild.StartTime.Unix()).Should(BeNumerically("~", time.Now().Unix(), 3))
				})
			})

			Context("when the build finishes", func() {
				BeforeEach(func() {
					err := sqlDB.FinishBuild(build1.ID, db.StatusSucceeded)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("sets the build's status and end time", func() {
					currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild.Status).Should(Equal(db.StatusSucceeded))
					Ω(currentBuild.EndTime.Unix()).Should(BeNumerically("~", time.Now().Unix(), 3))
				})
			})

			Context("and another is created for the same job", func() {
				var build2 db.Build

				BeforeEach(func() {
					var err error
					build2, err = pipelineDB.CreateJobBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(build2.ID).ShouldNot(BeZero())
					Ω(build2.ID).ShouldNot(Equal(build1.ID))
					Ω(build2.Name).Should(Equal("2"))
					Ω(build2.Status).Should(Equal(db.StatusPending))
				})

				It("can also be read back as the same object", func() {
					gotBuild, err := sqlDB.GetBuild(build2.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(gotBuild).Should(Equal(build2))
				})

				It("is returned in the job's builds, before the rest", func() {
					Ω(pipelineDB.GetAllJobBuilds("some-job")).Should(Equal([]db.Build{
						build2,
						build1,
					}))
				})

				Describe("the first build", func() {
					It("remains the next pending build", func() {
						nextPending, err := pipelineDB.GetNextPendingBuild("some-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(nextPending).Should(Equal(build1))
					})

					It("remains the current build", func() {
						currentBuild, err := pipelineDB.GetCurrentBuild("some-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(currentBuild).Should(Equal(build1))
					})
				})
			})

			Context("and another is created for a different job", func() {
				var otherJobBuild db.Build

				BeforeEach(func() {
					var err error

					otherJobBuild, err = pipelineDB.CreateJobBuild("some-other-job")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(otherJobBuild.ID).ShouldNot(BeZero())
					Ω(otherJobBuild.Name).Should(Equal("1"))
					Ω(otherJobBuild.Status).Should(Equal(db.StatusPending))
				})

				It("shows up in its job's builds", func() {
					Ω(pipelineDB.GetAllJobBuilds("some-other-job")).Should(Equal([]db.Build{otherJobBuild}))
				})

				It("does not show up in the first build's job's builds", func() {
					Ω(pipelineDB.GetAllJobBuilds("some-job")).Should(Equal([]db.Build{build1}))
				})
			})
		})

		Describe("determining the inputs for a job", func() {
			It("can still be scheduled with no inputs", func() {
				buildInputs, err := loadAndGetLatestInputVersions("third-job", []config.JobInput{})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(buildInputs).Should(Equal([]db.BuildInput{}))
			})

			It("ensures that when scanning for previous inputs versions it only considers those from the same job", func() {
				resource, err := pipelineDB.GetResource("some-resource")
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR1, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				otherResource, err := pipelineDB.GetResource("some-other-resource")
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Ω(err).ShouldNot(HaveOccurred())

				otherSavedVR1, err := pipelineDB.GetLatestVersionedResource(otherResource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR2, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}})
				Ω(err).ShouldNot(HaveOccurred())

				otherSavedVR2, err := pipelineDB.GetLatestVersionedResource(otherResource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "3"}})
				Ω(err).ShouldNot(HaveOccurred())

				savedVR3, err := pipelineDB.GetLatestVersionedResource(resource)
				Ω(err).ShouldNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "3"}})
				Ω(err).ShouldNot(HaveOccurred())

				otherSavedVR3, err := pipelineDB.GetLatestVersionedResource(otherResource)
				Ω(err).ShouldNot(HaveOccurred())

				build1, err := pipelineDB.CreateJobBuild("a-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build1.ID, db.BuildInput{
					Name:              "some-input-name",
					VersionedResource: savedVR1.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(build1.ID, savedVR1.VersionedResource, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build1.ID, db.BuildInput{
					Name:              "some-other-input-name",
					VersionedResource: otherSavedVR1.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(build1.ID, otherSavedVR1.VersionedResource, false)
				Ω(err).ShouldNot(HaveOccurred())

				otherBuild2, err := pipelineDB.CreateJobBuild("other-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(otherBuild2.ID, db.BuildInput{
					Name:              "some-input-name",
					VersionedResource: savedVR2.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(otherBuild2.ID, savedVR2.VersionedResource, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(otherBuild2.ID, db.BuildInput{
					Name:              "some-other-input-name",
					VersionedResource: otherSavedVR2.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(otherBuild2.ID, otherSavedVR2.VersionedResource, false)
				Ω(err).ShouldNot(HaveOccurred())

				build3, err := pipelineDB.CreateJobBuild("a-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build3.ID, db.BuildInput{
					Name:              "some-input-name",
					VersionedResource: savedVR3.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildInput(build3.ID, db.BuildInput{
					Name:              "some-other-input-name",
					VersionedResource: otherSavedVR3.VersionedResource,
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = sqlDB.FinishBuild(build1.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())
				err = sqlDB.FinishBuild(otherBuild2.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())
				err = sqlDB.FinishBuild(build3.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())

				jobBuildInputs := []config.JobInput{
					{
						Name:     "some-input-name",
						Resource: "some-resource",
						Passed:   []string{"a-job"},
					},
					{
						Name:     "some-other-input-name",
						Resource: "some-other-resource",
					},
				}

				Ω(loadAndGetLatestInputVersions("third-job", jobBuildInputs)).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "some-input-name",
						VersionedResource: savedVR1.VersionedResource,
					},
					{
						Name:              "some-other-input-name",
						VersionedResource: otherSavedVR3.VersionedResource,
					},
				}))
			})

			It("ensures that versions from jobs mentioned in two input's 'passed' sections came from the same successful builds", func() {
				j1b1, err := pipelineDB.CreateJobBuild("job-1")
				Ω(err).ShouldNot(HaveOccurred())

				j2b1, err := pipelineDB.CreateJobBuild("job-2")
				Ω(err).ShouldNot(HaveOccurred())

				sb1, err := pipelineDB.CreateJobBuild("shared-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.CreateJobBuild("job-1")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.CreateJobBuild("job-2")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.CreateJobBuild("shared-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = loadAndGetLatestInputVersions("a-job", []config.JobInput{
					{
						Name:     "input-1",
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Name:     "input-2",
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})
				Ω(err).Should(Equal(db.ErrNoVersions))

				_, err = pipelineDB.SaveBuildOutput(sb1.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.SaveBuildOutput(sb1.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(sb1.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.SaveBuildOutput(sb1.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				savedVR1, err := pipelineDB.SaveBuildOutput(j1b1.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.SaveBuildOutput(j1b1.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				savedVR2, err := pipelineDB.SaveBuildOutput(j2b1.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = otherPipelineDB.SaveBuildOutput(j2b1.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				err = sqlDB.FinishBuild(sb1.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())
				err = sqlDB.FinishBuild(j1b1.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())
				err = sqlDB.FinishBuild(j2b1.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", []config.JobInput{
					{
						Name:     "input-1",
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Name:     "input-2",
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "input-1",
						VersionedResource: savedVR1.VersionedResource,
					},
					{
						Name:              "input-2",
						VersionedResource: savedVR2.VersionedResource,
					},
				}))

				sb2, err := pipelineDB.CreateJobBuild("shared-job")
				Ω(err).ShouldNot(HaveOccurred())

				j1b2, err := pipelineDB.CreateJobBuild("job-1")
				Ω(err).ShouldNot(HaveOccurred())

				j2b2, err := pipelineDB.CreateJobBuild("job-2")
				Ω(err).ShouldNot(HaveOccurred())

				savedCommonVR1, err := pipelineDB.SaveBuildOutput(sb2.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r1-common-to-shared-and-j1"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(sb2.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r2-common-to-shared-and-j2"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				savedCommonVR1, err = pipelineDB.SaveBuildOutput(j1b2.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r1-common-to-shared-and-j1"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				// do NOT save resource-2 as an output of job-2

				Ω(loadAndGetLatestInputVersions("a-job", []config.JobInput{
					{
						Name:     "input-1",
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Name:     "input-2",
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "input-1",
						VersionedResource: savedVR1.VersionedResource,
					},
					{
						Name:              "input-2",
						VersionedResource: savedVR2.VersionedResource,
					},
				}))

				// now save the output of resource-2 job-2
				savedCommonVR2, err := pipelineDB.SaveBuildOutput(j2b2.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r2-common-to-shared-and-j2"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				err = sqlDB.FinishBuild(sb2.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())
				err = sqlDB.FinishBuild(j1b2.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())
				err = sqlDB.FinishBuild(j2b2.ID, db.StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", []config.JobInput{
					{
						Name:     "input-1",
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Name:     "input-2",
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "input-1",
						VersionedResource: savedCommonVR1.VersionedResource,
					},
					{
						Name:              "input-2",
						VersionedResource: savedCommonVR2.VersionedResource,
					},
				}))

				j2b3, err := pipelineDB.CreateJobBuild("job-2")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = pipelineDB.SaveBuildOutput(j2b3.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "should-not-be-emitted-because-of-failure"},
				}, false)
				Ω(err).ShouldNot(HaveOccurred())

				// Fail the 3rd build of the 2nd job, this should put the versions back to the previous set

				err = sqlDB.FinishBuild(j2b3.ID, db.StatusFailed)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(loadAndGetLatestInputVersions("a-job", []config.JobInput{
					{
						Name:     "input-2",
						Resource: "resource-2",
						Passed:   []string{"job-2"},
					},
				})).Should(ConsistOf([]db.BuildInput{
					{
						Name:              "input-2",
						VersionedResource: savedCommonVR2.VersionedResource,
					},
				}))

				// save newer versions; should be new latest
				for i := 0; i < 10; i++ {
					version := fmt.Sprintf("version-%d", i+1)

					savedCommonVR1, err := pipelineDB.SaveBuildOutput(sb1.ID, db.VersionedResource{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r1-common-to-shared-and-j1"},
					}, false)
					Ω(err).ShouldNot(HaveOccurred())

					savedCommonVR2, err := pipelineDB.SaveBuildOutput(sb1.ID, db.VersionedResource{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r2-common-to-shared-and-j2"},
					}, false)
					Ω(err).ShouldNot(HaveOccurred())

					savedCommonVR1, err = pipelineDB.SaveBuildOutput(j1b1.ID, db.VersionedResource{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r1-common-to-shared-and-j1"},
					}, false)
					Ω(err).ShouldNot(HaveOccurred())

					savedCommonVR2, err = pipelineDB.SaveBuildOutput(j2b1.ID, db.VersionedResource{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r2-common-to-shared-and-j2"},
					}, false)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(loadAndGetLatestInputVersions("a-job", []config.JobInput{
						{
							Name:     "input-1",
							Resource: "resource-1",
							Passed:   []string{"shared-job", "job-1"},
						},
						{
							Name:     "input-2",
							Resource: "resource-2",
							Passed:   []string{"shared-job", "job-2"},
						},
					})).Should(ConsistOf([]db.BuildInput{
						{
							Name:              "input-1",
							VersionedResource: savedCommonVR1.VersionedResource,
						},
						{
							Name:              "input-2",
							VersionedResource: savedCommonVR2.VersionedResource,
						},
					}))
				}
			})
		})

		It("can report a job's latest running and finished builds", func() {
			finished, next, err := pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next).Should(BeNil())
			Ω(finished).Should(BeNil())

			finishedBuild, err := pipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.FinishBuild(finishedBuild.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			otherFinishedBuild, err := otherPipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = sqlDB.FinishBuild(otherFinishedBuild.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next).Should(BeNil())
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			nextBuild, err := pipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			started, err := sqlDB.StartBuild(nextBuild.ID, "some-engine", "meta")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(started).Should(BeTrue())

			otherNextBuild, err := otherPipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			otherStarted, err := sqlDB.StartBuild(otherNextBuild.ID, "some-engine", "meta")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(otherStarted).Should(BeTrue())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(nextBuild.ID))
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			anotherRunningBuild, err := pipelineDB.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			started, err = sqlDB.StartBuild(anotherRunningBuild.ID, "some-engine", "meta")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(started).Should(BeTrue())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			err = sqlDB.FinishBuild(nextBuild.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(anotherRunningBuild.ID))
			Ω(finished.ID).Should(Equal(nextBuild.ID))
		})
	})
})
