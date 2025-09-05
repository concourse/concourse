package db_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/vars"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"

	// load dummy credential manager
	_ "github.com/concourse/concourse/atc/creds/dummy"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline       db.Pipeline
		team           db.Team
		pipelineConfig atc.Config
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		pipelineConfig = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"job-1", "job-2"},
					Resources: []string{"some-resource", "some-other-resource"},
				},
			},
			VarSources: atc.VarSourceConfigs{
				{
					Name: "some-var-source",
					Type: "dummy",
					Config: map[string]any{
						"vars": map[string]any{"pk": "pv"},
					},
				},
			},
			Display: &atc.DisplayConfig{
				BackgroundImage: "background.jpg",
			},
			Jobs: atc.JobConfigs{
				{
					Name: "job-name",

					Public: true,

					Serial: true,

					SerialGroups: []string{"serial-group"},

					PlanSequence: []atc.Step{
						{
							Config: &atc.PutStep{
								Name: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
						},
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
						},
						{
							Config: &atc.TaskStep{
								Name:       "some-task",
								Privileged: true,
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									RootfsURI: "some-image",
								},
							},
						},
						{
							Config: &atc.SetPipelineStep{
								Name:     "some-pipeline",
								File:     "some-file",
								VarFiles: []string{"var-file1", "var-file2"},
								Vars: map[string]any{
									"k1": "v1",
									"k2": "v2",
								},
							},
						},
					},
				},
				{
					Name:   "some-other-job",
					Serial: true,
				},
				{
					Name: "a-job",
				},
				{
					Name: "shared-job",
				},
				{
					Name: "random-job",
				},
				{
					Name: "job-1",
				},
				{
					Name: "job-2",
				},
				{
					Name:         "other-serial-group-job",
					SerialGroups: []string{"serial-group", "really-different-group"},
				},
				{
					Name:         "different-serial-group-job",
					SerialGroups: []string{"different-serial-group"},
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "other-source"},
				},
				{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				},
			},
			ResourceTypes: atc.ResourceTypes{
				{
					Name:   "some-other-resource-type",
					Type:   "base-type",
					Source: atc.Source{"some": "other-type-soure"},
				},
				{
					Name:   "some-resource-type",
					Type:   "base-type",
					Source: atc.Source{"some": "type-soure"},
				},
			},
			Prototypes: atc.Prototypes{
				{
					Name:   "some-other-prototype",
					Type:   "base-type",
					Source: atc.Source{"some": "other-type-source"},
				},
				{
					Name:   "some-prototype",
					Type:   "base-type",
					Source: atc.Source{"some": "type-source"},
				},
			},
		}
		var created bool
		pipeline, created, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline"}, pipelineConfig, db.ConfigVersion(0), false)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())

		_, found, err := pipeline.Job("job-name")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-type",
		}

		_, err = brt.FindOrCreate(setupTx, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())
	})

	Describe("CheckPaused", func() {
		var paused bool
		JustBeforeEach(func() {
			var err error
			paused, err = pipeline.CheckPaused()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the pipeline is unpaused", func() {
			BeforeEach(func() {
				Expect(pipeline.Unpause()).To(Succeed())
			})

			It("returns the pipeline is paused", func() {
				Expect(paused).To(BeFalse())
			})
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause("")).To(Succeed())
			})

			It("returns the pipeline is paused", func() {
				Expect(paused).To(BeTrue())
			})
		})
	})

	Describe("Pause", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Pause("")).To(Succeed())

			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is unpaused", func() {
			BeforeEach(func() {
				Expect(pipeline.Unpause()).To(Succeed())
			})

			It("pauses the pipeline", func() {
				Expect(pipeline.Paused()).To(BeTrue())
			})
		})
	})

	Describe("PausedBy", func() {
		JustBeforeEach(func() {
			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause("")).To(Succeed())
			})

			It("has an empty paused by", func() {
				Expect(pipeline.PausedBy()).To(Equal(""))
			})
		})

		Context("when the pipeline is paused with a username", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause("concourse")).To(Succeed())
			})

			It("has an paused by", func() {
				Expect(pipeline.PausedBy()).To(Equal("concourse"))
			})
		})
	})

	Describe("PausedAt", func() {
		JustBeforeEach(func() {
			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause("")).To(Succeed())
			})

			It("has a paused at time stamp", func() {
				Expect(pipeline.PausedAt()).Should(BeTemporally("~", time.Now(), time.Duration(1*time.Second)))
			})
		})
	})

	Describe("Archive", func() {
		var initialLastUpdated time.Time

		BeforeEach(func() {
			initialLastUpdated = pipeline.LastUpdated()
		})

		JustBeforeEach(func() {
			pipeline.Archive()
			pipeline.Reload()
		})

		It("archives the pipeline", func() {
			Expect(pipeline.Archived()).To(BeTrue(), "pipeline was not archived")
		})

		It("updates last updated", func() {
			lastUpdated := pipeline.LastUpdated()

			Expect(lastUpdated).To(BeTemporally(">", initialLastUpdated))
		})

		It("resets the pipeline version to zero", func() {
			version := pipeline.ConfigVersion()

			Expect(version).To(Equal(db.ConfigVersion(0)))
		})

		It("removes the config of each job", func() {
			jobs, err := pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())

			jobConfigs, err := jobs.Configs()
			Expect(err).ToNot(HaveOccurred())
			emptyJobConfigs := make(atc.JobConfigs, len(pipelineConfig.Jobs))
			Expect(jobConfigs).To(Equal(emptyJobConfigs))
		})

		It("removes the config of each resource", func() {
			resources, err := pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigs := resources.Configs()

			emptyResourceConfigs := make(atc.ResourceConfigs, len(pipelineConfig.Resources))
			Expect(resourceConfigs).To(Equal(emptyResourceConfigs))
		})

		It("removes the config of each resource_type", func() {
			resourceTypes, err := pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceTypeConfigs := resourceTypes.Configs()

			emptyResourceTypeConfigs := atc.ResourceTypes{
				{Name: "some-other-resource-type", Type: "base-type"},
				{Name: "some-resource-type", Type: "base-type"},
			}
			Expect(resourceTypeConfigs).To(Equal(emptyResourceTypeConfigs))
		})

		It("removes the config of each prototype", func() {
			prototypes, err := pipeline.Prototypes()
			Expect(err).ToNot(HaveOccurred())

			prototypeConfigs := prototypes.Configs()

			emptyPrototypeConfigs := atc.Prototypes{
				{Name: "some-other-prototype", Type: "base-type"},
				{Name: "some-prototype", Type: "base-type"},
			}
			Expect(prototypeConfigs).To(Equal(emptyPrototypeConfigs))
		})
	})

	Describe("Unpause", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Unpause()).To(Succeed())

			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause("")).To(Succeed())
			})

			It("unpauses the pipeline", func() {
				Expect(pipeline.Paused()).To(BeFalse())
			})
		})

		Context("when requesting schedule for unpausing pipeline", func() {
			var found bool
			var err error
			var job1, job2, job3, job4, job5, job6, job7, job8, job9 db.Job
			var initialRequestedTime1, initialRequestedTime2, initialRequestedTime3, initialRequestedTime4, initialRequestedTime5, initialRequestedTime6, initialRequestedTime7, initialRequestedTime8, initialRequestedTime9 time.Time

			BeforeEach(func() {
				job1, found, err = pipeline.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime1 = job1.ScheduleRequestedTime()

				job2, found, err = pipeline.Job("some-other-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime2 = job2.ScheduleRequestedTime()

				job3, found, err = pipeline.Job("a-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime3 = job3.ScheduleRequestedTime()

				job4, found, err = pipeline.Job("shared-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime4 = job4.ScheduleRequestedTime()

				job5, found, err = pipeline.Job("random-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime5 = job5.ScheduleRequestedTime()

				job6, found, err = pipeline.Job("job-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime6 = job6.ScheduleRequestedTime()

				job7, found, err = pipeline.Job("job-2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime7 = job7.ScheduleRequestedTime()

				job8, found, err = pipeline.Job("other-serial-group-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime8 = job8.ScheduleRequestedTime()

				job9, found, err = pipeline.Job("different-serial-group-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				initialRequestedTime9 = job9.ScheduleRequestedTime()
			})

			It("requests schedule on all the jobs in the pipeline", func() {
				found, err = job1.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job2.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job3.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job4.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job5.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job6.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job7.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job8.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = job9.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job1.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime1))
				Expect(job2.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime2))
				Expect(job3.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime3))
				Expect(job4.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime4))
				Expect(job5.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime5))
				Expect(job6.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime6))
				Expect(job7.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime7))
				Expect(job8.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime8))
				Expect(job9.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime9))
			})
		})
	})

	Describe("Resource Config Versions", func() {
		resourceName := "some-resource"
		otherResourceName := "some-other-resource"
		reallyOtherResourceName := "some-really-other-resource"

		var (
			scenarioPipeline1 *dbtest.Scenario
			scenarioPipeline2 *dbtest.Scenario

			resource              db.Resource
			otherResource         db.Resource
			reallyOtherResource   db.Resource
			otherPipelineResource db.Resource
		)

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"some-resource", "some-other-resource"},
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
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-other-value",
						},
					},
					{
						Name: "some-really-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-really-other-value",
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

						Serial: true,

						SerialGroups: []string{"serial-group"},

						PlanSequence: []atc.Step{
							{
								Config: &atc.PutStep{
									Name: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
									Passed:  []string{"job-1", "job-2"},
									Trigger: true,
								},
							},
							{
								Config: &atc.TaskStep{
									Name:       "some-task",
									Privileged: true,
									ConfigPath: "some/config/path.yml",
									Config: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
								},
							},
						},
					},
					{
						Name:   "some-other-job",
						Serial: true,
					},
					{
						Name: "a-job",
					},
					{
						Name: "shared-job",
					},
					{
						Name: "random-job",
					},
					{
						Name:         "other-serial-group-job",
						SerialGroups: []string{"serial-group", "really-different-group"},
					},
					{
						Name:         "different-serial-group-job",
						SerialGroups: []string{"different-serial-group"},
					},
					{
						Name: "job-1",
					},
					{
						Name: "job-2",
					},
				},
			}

			scenarioPipeline1 = dbtest.Setup(
				builder.WithPipeline(pipelineConfig),
				builder.WithResourceVersions("some-resource"),
				builder.WithResourceVersions("some-really-other-resource"),
			)

			otherPipelineConfig := atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"some-resource", "some-other-resource"},
					},
				},

				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"other-source-config": "some-value",
						},
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"other-source-config": "some-other-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
					{
						Name: "a-job",
					},
					{
						Name: "shared-job",
					},
					{
						Name: "other-serial-group-job",
					},
				},
			}

			scenarioPipeline2 = dbtest.Setup(
				builder.WithPipeline(otherPipelineConfig),
				builder.WithResourceVersions("some-other-resource"),
			)

			resource = scenarioPipeline1.Resource(resourceName)
			otherResource = scenarioPipeline1.Resource(otherResourceName)
			reallyOtherResource = scenarioPipeline1.Resource(reallyOtherResourceName)
			otherPipelineResource = scenarioPipeline2.Resource(otherResourceName)
		})

		It("returns correct resource", func() {
			Expect(resource.Name()).To(Equal("some-resource"))
			Expect(resource.PipelineName()).To(Equal("some-pipeline"))
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{"source-config": "some-value"}))
		})

		Context("DebugLoadVersionsDB", func() {
			It("it can load all information about the current state of the db", func() {
				versions, err := scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versions.ResourceVersions).To(BeEmpty())
				Expect(versions.BuildOutputs).To(BeEmpty())
				Expect(versions.Resources).To(ConsistOf([]atc.DebugResource{
					{
						ID:      resource.ID(),
						Name:    resource.Name(),
						ScopeID: intptr(resource.ResourceConfigScopeID()),
					},
					{
						ID:      otherResource.ID(),
						Name:    otherResource.Name(),
						ScopeID: nil,
					},
					{
						ID:      reallyOtherResource.ID(),
						Name:    reallyOtherResource.Name(),
						ScopeID: intptr(reallyOtherResource.ResourceConfigScopeID()),
					},
				}))

				jobs := []atc.DebugJob{
					{Name: "some-job", ID: scenarioPipeline1.Job("some-job").ID()},
					{Name: "some-other-job", ID: scenarioPipeline1.Job("some-other-job").ID()},
					{Name: "a-job", ID: scenarioPipeline1.Job("a-job").ID()},
					{Name: "shared-job", ID: scenarioPipeline1.Job("shared-job").ID()},
					{Name: "random-job", ID: scenarioPipeline1.Job("random-job").ID()},
					{Name: "other-serial-group-job", ID: scenarioPipeline1.Job("other-serial-group-job").ID()},
					{Name: "different-serial-group-job", ID: scenarioPipeline1.Job("different-serial-group-job").ID()},
					{Name: "job-1", ID: scenarioPipeline1.Job("job-1").ID()},
					{Name: "job-2", ID: scenarioPipeline1.Job("job-2").ID()},
				}

				Expect(versions.Jobs).To(ConsistOf(jobs))

				By("initially having no versions")
				resourceVersions, _, _, err := resource.Versions(db.Page{Limit: 10}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(resourceVersions).To(HaveLen(0))

				By("including saved versioned resources of the current pipeline")
				scenarioPipeline1.Run(builder.WithResourceVersions(resourceName, atc.Version{"version": "1"}))

				savedVR1, found, err := resource.FindVersion(atc.Version{"version": "1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				scenarioPipeline1.Run(builder.WithResourceVersions(resourceName, atc.Version{"version": "2"}))

				savedVR2, found, err := resource.FindVersion(atc.Version{"version": "2"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versions.ResourceVersions).To(ConsistOf([]atc.DebugResourceVersion{
					{VersionID: savedVR1.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR1.CheckOrder()},
					{VersionID: savedVR2.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR2.CheckOrder()},
				}))

				Expect(versions.BuildOutputs).To(BeEmpty())
				Expect(versions.Resources).To(ConsistOf([]atc.DebugResource{
					{
						ID:      resource.ID(),
						Name:    resource.Name(),
						ScopeID: intptr(resource.ResourceConfigScopeID()),
					},
					{
						ID:      otherResource.ID(),
						Name:    otherResource.Name(),
						ScopeID: nil,
					},
					{
						ID:      reallyOtherResource.ID(),
						Name:    reallyOtherResource.Name(),
						ScopeID: intptr(reallyOtherResource.ResourceConfigScopeID()),
					},
				}))
				Expect(versions.Jobs).To(ConsistOf(jobs))

				By("not including saved versioned resources of other pipelines")
				scenarioPipeline2.Run(builder.WithResourceVersions(otherResourceName, atc.Version{"version": "1"}))

				_, found, err = otherPipelineResource.FindVersion(atc.Version{"version": "1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versions.ResourceVersions).To(ConsistOf([]atc.DebugResourceVersion{
					{
						VersionID:  savedVR1.ID(),
						ResourceID: resource.ID(),
						ScopeID:    resource.ResourceConfigScopeID(),
						CheckOrder: savedVR1.CheckOrder(),
					},
					{
						VersionID:  savedVR2.ID(),
						ResourceID: resource.ID(),
						ScopeID:    resource.ResourceConfigScopeID(),
						CheckOrder: savedVR2.CheckOrder(),
					},
				}))

				Expect(versions.BuildOutputs).To(BeEmpty())
				Expect(versions.Resources).To(ConsistOf([]atc.DebugResource{
					{
						ID:      resource.ID(),
						Name:    resource.Name(),
						ScopeID: intptr(resource.ResourceConfigScopeID()),
					},
					{
						ID:      otherResource.ID(),
						Name:    otherResource.Name(),
						ScopeID: nil,
					},
					{
						ID:      reallyOtherResource.ID(),
						Name:    reallyOtherResource.Name(),
						ScopeID: intptr(reallyOtherResource.ResourceConfigScopeID()),
					},
				}))
				Expect(versions.Jobs).To(ConsistOf(jobs))

				By("including outputs of successful builds")
				build1DB, err := scenarioPipeline1.Job("a-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				err = build1DB.SaveOutput("some-type", nil, atc.Source{"source-config": "some-value"}, atc.Version{"version": "1"}, nil, "some-output-name", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				err = build1DB.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versions.ResourceVersions).To(ConsistOf([]atc.DebugResourceVersion{
					{VersionID: savedVR1.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR1.CheckOrder()},
					{VersionID: savedVR2.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR2.CheckOrder()},
				}))

				explicitOutput := atc.DebugBuildOutput{
					DebugResourceVersion: atc.DebugResourceVersion{
						VersionID:  savedVR1.ID(),
						ResourceID: resource.ID(),
						ScopeID:    resource.ResourceConfigScopeID(),
						CheckOrder: savedVR1.CheckOrder(),
					},
					JobID:   scenarioPipeline1.Job("a-job").ID(),
					BuildID: build1DB.ID(),
				}

				Expect(versions.BuildOutputs).To(ConsistOf([]atc.DebugBuildOutput{
					explicitOutput,
				}))

				Expect(versions.Resources).To(ConsistOf([]atc.DebugResource{
					{
						ID:      resource.ID(),
						Name:    resource.Name(),
						ScopeID: intptr(resource.ResourceConfigScopeID()),
					},
					{
						ID:      otherResource.ID(),
						Name:    otherResource.Name(),
						ScopeID: nil,
					},
					{
						ID:      reallyOtherResource.ID(),
						Name:    reallyOtherResource.Name(),
						ScopeID: intptr(reallyOtherResource.ResourceConfigScopeID()),
					},
				}))
				Expect(versions.Jobs).To(ConsistOf(jobs))

				By("not including outputs of failed builds")
				build2DB, err := scenarioPipeline1.Job("a-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				err = build2DB.SaveOutput("some-type", nil, atc.Source{"source-config": "some-value"}, atc.Version{"version": "1"}, nil, "some-output-name", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				err = build2DB.Finish(db.BuildStatusFailed)
				Expect(err).ToNot(HaveOccurred())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versions.ResourceVersions).To(ConsistOf([]atc.DebugResourceVersion{
					{VersionID: savedVR1.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR1.CheckOrder()},
					{VersionID: savedVR2.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR2.CheckOrder()},
				}))

				Expect(versions.BuildOutputs).To(ConsistOf([]atc.DebugBuildOutput{
					{
						DebugResourceVersion: atc.DebugResourceVersion{
							VersionID:  savedVR1.ID(),
							ResourceID: resource.ID(),
							ScopeID:    resource.ResourceConfigScopeID(),
							CheckOrder: savedVR1.CheckOrder(),
						},
						JobID:   scenarioPipeline1.Job("a-job").ID(),
						BuildID: build1DB.ID(),
					},
				}))

				Expect(versions.Resources).To(ConsistOf([]atc.DebugResource{
					{
						ID:      resource.ID(),
						Name:    resource.Name(),
						ScopeID: intptr(resource.ResourceConfigScopeID()),
					},
					{
						ID:      otherResource.ID(),
						Name:    otherResource.Name(),
						ScopeID: nil,
					},
					{
						ID:      reallyOtherResource.ID(),
						Name:    reallyOtherResource.Name(),
						ScopeID: intptr(reallyOtherResource.ResourceConfigScopeID()),
					},
				}))
				Expect(versions.Jobs).To(ConsistOf(jobs))

				By("not including outputs of builds in other pipelines")
				otherPipelineBuild, err := scenarioPipeline1.Job("a-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				err = otherPipelineBuild.SaveOutput("some-type", nil, atc.Source{"other-source-config": "some-other-value"}, atc.Version{"version": "1"}, nil, "some-output-name", "some-other-resource")
				Expect(err).ToNot(HaveOccurred())

				// After SaveOutput to other resource, we need to reload it because its resourceConfigScopeID
				// is supposed to be updated.
				otherResource = scenarioPipeline1.Resource(otherResourceName)
				Expect(otherResource).ToNot(BeNil())

				savedVR3, found, err := otherResource.FindVersion(atc.Version{"version": "1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = otherPipelineBuild.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versions.ResourceVersions).To(ConsistOf([]atc.DebugResourceVersion{
					{VersionID: savedVR1.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR1.CheckOrder()},
					{VersionID: savedVR2.ID(), ResourceID: resource.ID(), ScopeID: resource.ResourceConfigScopeID(), CheckOrder: savedVR2.CheckOrder()},
					{VersionID: savedVR3.ID(), ResourceID: otherResource.ID(), ScopeID: otherResource.ResourceConfigScopeID(), CheckOrder: savedVR3.CheckOrder()},
				}))

				Expect(versions.BuildOutputs).To(ConsistOf([]atc.DebugBuildOutput{
					{
						DebugResourceVersion: atc.DebugResourceVersion{
							VersionID:  savedVR3.ID(),
							ResourceID: otherResource.ID(),
							ScopeID:    otherResource.ResourceConfigScopeID(),
							CheckOrder: savedVR3.CheckOrder(),
						},
						JobID:   scenarioPipeline1.Job("a-job").ID(),
						BuildID: otherPipelineBuild.ID(),
					},
					{
						DebugResourceVersion: atc.DebugResourceVersion{
							VersionID:  savedVR1.ID(),
							ResourceID: resource.ID(),
							ScopeID:    resource.ResourceConfigScopeID(),
							CheckOrder: savedVR1.CheckOrder(),
						},
						JobID:   scenarioPipeline1.Job("a-job").ID(),
						BuildID: build1DB.ID(),
					},
				}))

				Expect(versions.Resources).To(ConsistOf([]atc.DebugResource{
					{
						ID:      resource.ID(),
						Name:    resource.Name(),
						ScopeID: intptr(resource.ResourceConfigScopeID()),
					},
					{
						ID:      otherResource.ID(),
						Name:    otherResource.Name(),
						ScopeID: intptr(otherResource.ResourceConfigScopeID()),
					},
					{
						ID:      reallyOtherResource.ID(),
						Name:    reallyOtherResource.Name(),
						ScopeID: intptr(reallyOtherResource.ResourceConfigScopeID()),
					},
				}))
				Expect(versions.Jobs).To(ConsistOf(jobs))

				err = scenarioPipeline1.Job("a-job").SaveNextInputMapping(db.InputMapping{
					"some-input-name": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"version": "1"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: true,
						},
						PassedBuildIDs: []int{},
					}}, true)
				Expect(err).ToNot(HaveOccurred())

				build1DB, err = scenarioPipeline1.Job("a-job").CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				_, found, err = build1DB.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1DB.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				Expect(versions.BuildInputs).To(ConsistOf([]atc.DebugBuildInput{
					{
						DebugResourceVersion: atc.DebugResourceVersion{
							VersionID:  savedVR1.ID(),
							ResourceID: resource.ID(),
							ScopeID:    resource.ResourceConfigScopeID(),
							CheckOrder: savedVR1.CheckOrder(),
						},
						JobID:     scenarioPipeline1.Job("a-job").ID(),
						BuildID:   build1DB.ID(),
						InputName: "some-input-name",
					},
				}))

				By("including implicit outputs of successful builds")
				implicitOutput := atc.DebugBuildOutput{
					DebugResourceVersion: atc.DebugResourceVersion{
						VersionID:  savedVR1.ID(),
						ResourceID: resource.ID(),
						ScopeID:    resource.ResourceConfigScopeID(),
						CheckOrder: savedVR1.CheckOrder(),
					},
					JobID:   scenarioPipeline1.Job("a-job").ID(),
					BuildID: build1DB.ID(),
				}

				By("including put-only resource's outputs of successful builds")
				otherExplicitOutput := atc.DebugBuildOutput{
					DebugResourceVersion: atc.DebugResourceVersion{
						VersionID:  savedVR3.ID(),
						ResourceID: otherResource.ID(),
						ScopeID:    otherResource.ResourceConfigScopeID(),
						CheckOrder: savedVR3.CheckOrder(),
					},
					JobID:   scenarioPipeline1.Job("a-job").ID(),
					BuildID: otherPipelineBuild.ID(),
				}

				Expect(versions.BuildOutputs).To(ConsistOf([]atc.DebugBuildOutput{
					otherExplicitOutput,
					explicitOutput,
					implicitOutput,
				}))

				By("including build rerun mappings for builds")
				build2DB, err = scenarioPipeline1.Job("a-job").RerunBuild(build1DB, defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				versions, err = scenarioPipeline1.Pipeline.LoadDebugVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				Expect(versions.BuildReruns).To(ConsistOf([]atc.DebugBuildRerun{
					{
						JobID:   build1DB.JobID(),
						BuildID: build2DB.ID(),
						RerunOf: build1DB.ID(),
					},
				}))
			})
		})

		It("can load up the latest versioned resource, enabled or not", func() {
			By("initially having no versions")
			resourceVersions, _, found, err := resource.Versions(db.Page{Limit: 10}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resourceVersions).To(HaveLen(0))

			By("including saved versioned resources of the current pipeline")
			scenarioPipeline1.Run(builder.WithResourceVersions(resourceName, atc.Version{"version": "1"}))

			savedVR1, found, err := resource.FindVersion(atc.Version{"version": "1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			scenarioPipeline1.Run(builder.WithResourceVersions(resourceName, atc.Version{"version": "2"}))

			savedVR2, found, err := resource.FindVersion(atc.Version{"version": "2"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(savedVR1.Version()).To(Equal(db.Version{"version": "1"}))
			Expect(savedVR2.Version()).To(Equal(db.Version{"version": "2"}))

			By("not including saved versioned resources of other pipelines")
			scenarioPipeline2.Run(builder.WithResourceVersions(otherResourceName, atc.Version{"version": "3"}, atc.Version{"version": "4"}, atc.Version{"version": "5"}))

			_, found, err = scenarioPipeline2.Resource(otherResourceName).FindVersion(atc.Version{"version": "3"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, found, err = scenarioPipeline2.Resource(otherResourceName).FindVersion(atc.Version{"version": "1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including disabled versions")
			err = resource.DisableVersion(savedVR2.ID())
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err := resource.FindVersion(atc.Version{"version": "2"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version()).To(Equal(db.Version{"version": "2"}))
		})

		It("initially has no pending build for a job", func() {
			pendingBuilds, err := scenarioPipeline1.Job("some-job").GetPendingBuilds()
			Expect(err).ToNot(HaveOccurred())
			Expect(pendingBuilds).To(HaveLen(0))
		})
	})

	Describe("Destroy", func() {
		var scenario *dbtest.Scenario

		It("removes the pipeline and all of its data", func() {
			By("populating resources table")
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "some-type",
							Source: atc.Source{"some": "source"},
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
									},
								},
							},
						},
					},
				}),
				builder.WithResourceVersions("some-resource", atc.Version{"key": "value"}),
			)

			By("populating builds")
			build, err := scenario.Job("some-job").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			By("populating build inputs")
			err = scenario.Job("some-job").SaveNextInputMapping(db.InputMapping{
				"build-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"key": "value"})),
							ResourceID: scenario.Resource("some-resource").ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				}}, true)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := build.AdoptInputsAndPipes()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			By("populating build outputs")
			err = build.SaveOutput("some-type", nil, atc.Source{"some": "source"}, atc.Version{"key": "value"}, nil, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			By("populating build events")
			err = build.SaveEvent(event.StartTask{})
			Expect(err).ToNot(HaveOccurred())

			err = scenario.Pipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())

			found, err = scenario.Pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			found, err = build.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = scenario.Team.Pipeline(atc.PipelineRef{Name: pipeline.Name()})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("marks the pipeline ID in the deleted_pipelines table", func() {
			destroy(pipeline)

			var exists bool
			err := dbConn.QueryRow(fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM deleted_pipelines WHERE id = %d)", pipeline.ID())).Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "did not mark the pipeline id in deleted_pipelines")
		})
	})

	Describe("Dashboard", func() {
		It("returns a Dashboard object with a DashboardJob corresponding to each configured job", func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = job.UpdateFirstLoggedBuildID(57)
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := pipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := pipeline.Job("shared-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := pipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job1, found, err := pipeline.Job("job-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job2, found, err := pipeline.Job("job-2")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := pipeline.Job("other-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := pipeline.Job("different-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			By("returning jobs with no builds")
			actualDashboard, err := pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Name).To(Equal(job.Name()))
			Expect(actualDashboard[1].Name).To(Equal(otherJob.Name()))
			Expect(actualDashboard[2].Name).To(Equal(aJob.Name()))
			Expect(actualDashboard[3].Name).To(Equal(sharedJob.Name()))
			Expect(actualDashboard[4].Name).To(Equal(randomJob.Name()))
			Expect(actualDashboard[5].Name).To(Equal(job1.Name()))
			Expect(actualDashboard[6].Name).To(Equal(job2.Name()))
			Expect(actualDashboard[7].Name).To(Equal(otherSerialGroupJob.Name()))
			Expect(actualDashboard[8].Name).To(Equal(differentSerialGroupJob.Name()))

			By("returning a job's most recent pending build if there are no running builds")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			firstJobBuild, err := job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Name).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID).To(Equal(firstJobBuild.ID()))

			By("returning a job's most recent started build")
			found, err = firstJobBuild.Start(atc.Plan{ID: "some-id"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			found, err = firstJobBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Name).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID).To(Equal(firstJobBuild.ID()))
			Expect(actualDashboard[0].NextBuild.Status).To(Equal(atc.StatusStarted))

			By("returning a job's most recent started build even if there is a newer pending build")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			secondJobBuild, err := job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Name).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID).To(Equal(firstJobBuild.ID()))

			By("returning a job's most recent finished build")
			err = firstJobBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = secondJobBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			found, err = secondJobBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Name).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild).To(BeNil())
			Expect(actualDashboard[0].FinishedBuild.ID).To(Equal(secondJobBuild.ID()))

			By("returning the job inputs and outputs")
			Expect(actualDashboard[0].Outputs).To(ConsistOf(atc.JobOutputSummary{
				Name:     "some-resource",
				Resource: "some-resource",
			}))
			Expect(actualDashboard[0].Inputs).To(ConsistOf(atc.JobInputSummary{
				Name:     "some-input",
				Resource: "some-resource",
				Passed:   []string{"job-1", "job-2"},
				Trigger:  true,
			}))
		})
	})

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1DB, err := pipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.SaveEvent(event.Log{
				Payload: "log 1",
			})
			Expect(err).ToNot(HaveOccurred())

			build2DB, err := pipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).ToNot(HaveOccurred())

			build3DB, err := pipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build3DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			build4DB, err := pipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			By("doing nothing if the list is empty")
			err = pipeline.DeleteBuildEventsByBuildIDs([]int{})
			Expect(err).ToNot(HaveOccurred())

			By("not returning an error")
			err = pipeline.DeleteBuildEventsByBuildIDs([]int{build3DB.ID(), build4DB.ID(), build1DB.ID()})
			Expect(err).ToNot(HaveOccurred())

			err = build4DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			By("deleting events for build 1")
			events1, err := build1DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events1)

			_, err = events1.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("preserving events for build 2")
			events2, err := build2DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events2)

			build2Event1, err := events2.Next()
			Expect(err).ToNot(HaveOccurred())
			Expect(build2Event1).To(Equal(envelope(event.Log{
				Payload: "log 2",
			}, "0")))

			_, err = events2.Next() // finish event
			Expect(err).ToNot(HaveOccurred())

			_, err = events2.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("deleting events for build 3")
			events3, err := build3DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events3)

			_, err = events3.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")
			events4, err := build4DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events4)

			_, err = events4.Next() // finish event
			Expect(err).ToNot(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")
			found, err := build1DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build1DB.ReapTime()).To(BeTemporally(">", build1DB.EndTime()))

			found, err = build2DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build2DB.ReapTime()).To(BeZero())

			found, err = build3DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build3DB.ReapTime()).To(Equal(build1DB.ReapTime()))

			found, err = build4DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(build4DB.ReapTime()).To(Equal(build1DB.ReapTime()))
		})
		It("deletes all build logs when there are more than 65_536", func() {
			numToInsert := 250_000
			ids := make([]int, numToInsert)

			// we break the sql.db abstraction so that we can efficiently bulk
			// insert a heap of build events. If we did this the "right" way
			// (like the test above) it's excruciatingly slow.
			conn, err := dbConn.Conn(context.Background())
			Expect(err).ToNot(HaveOccurred())
			err = conn.Raw(func(driverConn any) error {
				pgxConn := driverConn.(*stdlib.Conn).Conn() // conn is a *pgx.Conn now
				txn, err := pgxConn.Begin(context.Background())
				Expect(err).ToNot(HaveOccurred())

				cols := []string{"event_id", "build_id", "type", "version", "payload"}
				counter := 0
				copyCount, err := txn.CopyFrom(context.Background(),
					pgx.Identifier{fmt.Sprintf("pipeline_build_events_%d", pipeline.ID())},
					cols, pgx.CopyFromFunc(func() (row []any, err error) {
						if counter >= numToInsert {
							return nil, nil
						}
						c := fmt.Sprintf("%d", counter)
						row = []any{c, c, c, c, ""}
						ids[counter] = counter
						counter++
						return row, nil
					}))
				Expect(int(copyCount)).To(Equal(numToInsert))
				Expect(err).ToNot(HaveOccurred())

				err = txn.Commit(context.Background())
				Expect(err).ToNot(HaveOccurred())

				return nil
			})
			Expect(err).ToNot(HaveOccurred())
			err = conn.Close()
			Expect(err).ToNot(HaveOccurred())

			var count int
			err = dbConn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pipeline_build_events_%d", pipeline.ID())).Scan(&count)
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(numToInsert))

			err = pipeline.DeleteBuildEventsByBuildIDs(ids)
			Expect(err).ToNot(HaveOccurred())

			err = dbConn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pipeline_build_events_%d", pipeline.ID())).Scan(&count)
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Describe("Jobs", func() {
		var jobs []db.Job

		BeforeEach(func() {
			var err error
			jobs, err = pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns all the jobs", func() {
			Expect(jobs[0].Name()).To(Equal("job-name"))
			Expect(jobs[1].Name()).To(Equal("some-other-job"))
			Expect(jobs[2].Name()).To(Equal("a-job"))
			Expect(jobs[3].Name()).To(Equal("shared-job"))
			Expect(jobs[4].Name()).To(Equal("random-job"))
			Expect(jobs[5].Name()).To(Equal("job-1"))
			Expect(jobs[6].Name()).To(Equal("job-2"))
			Expect(jobs[7].Name()).To(Equal("other-serial-group-job"))
			Expect(jobs[8].Name()).To(Equal("different-serial-group-job"))
		})
	})

	Describe("GetBuildsWithVersionAsInput", func() {
		var (
			resourceConfigVersion int
			expectedBuilds        []int
			dbSecondBuild         db.Build
			scenario              *dbtest.Scenario
		)

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "job-name",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
									},
								},
							},
						},
						{
							Name:   "some-other-job",
							Serial: true,
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "some-type",
							Source: atc.Source{"some": "source"},
						},
					},
				}),
				builder.WithResourceVersions("some-resource", atc.Version{"version": "v1"}),
			)

			build, err := scenario.Job("job-name").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build.ID())

			secondBuild, err := scenario.Job("job-name").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild.ID())

			_, err = scenario.Job("some-other-job").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			dbBuild, found, err := buildFactory.Build(build.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = scenario.Job("job-name").SaveNextInputMapping(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"version": "v1"})),
							ResourceID: scenario.Resource("some-resource").ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				}}, true)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = dbBuild.AdoptInputsAndPipes()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			dbSecondBuild, found, err = buildFactory.Build(secondBuild.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			scenario.Run(builder.WithResourceVersions(
				"some-resource",
				atc.Version{"version": "v2"},
				atc.Version{"version": "v3"},
				atc.Version{"version": "v4"},
			))

			err = scenario.Job("job-name").SaveNextInputMapping(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"version": "v1"})),
							ResourceID: scenario.Resource("some-resource").ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
				"some-other-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"version": "v3"})),
							ResourceID: scenario.Resource("some-resource").ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}, true)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = dbSecondBuild.AdoptInputsAndPipes()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			rcv1 := scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"})

			resourceConfigVersion = rcv1.ID()
		})

		It("returns the two builds for which the provided version id was an input", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(scenario.Resource("some-resource").ID(), resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(HaveLen(len(expectedBuilds)))

			buildIDs := []int{}
			for _, b := range builds {
				buildIDs = append(buildIDs, b.ID())
			}

			Expect(buildIDs).To(ConsistOf(expectedBuilds))
		})

		It("returns the one build that uses the version as an input", func() {
			rcv3 := scenario.ResourceVersion("some-resource", atc.Version{"version": "v3"})

			builds, err := pipeline.GetBuildsWithVersionAsInput(scenario.Resource("some-resource").ID(), rcv3.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(HaveLen(1))
			Expect(builds[0].ID()).To(Equal(dbSecondBuild.ID()))
		})

		It("returns an empty slice of builds when the provided version id exists but is not used", func() {
			rcv4 := scenario.ResourceVersion("some-resource", atc.Version{"version": "v4"})

			builds, err := pipeline.GetBuildsWithVersionAsInput(scenario.Resource("some-resource").ID(), rcv4.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(scenario.Resource("some-resource").ID(), resourceConfigVersion+100)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided resource id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(10293912, resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})

	Describe("GetBuildsWithVersionAsOutput", func() {
		var (
			resourceConfigVersion int
			expectedBuilds        []db.Build
			secondBuild           db.Build
			scenario              *dbtest.Scenario
		)

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "job-name",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
									},
								},
							},
						},
						{
							Name:   "some-other-job",
							Serial: true,
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "some-type",
							Source: atc.Source{"some": "source"},
						},
					},
				}),
				builder.WithResourceVersions("some-resource", atc.Version{"version": "v3"}, atc.Version{"version": "v4"}),
			)

			build, err := scenario.Job("job-name").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err = scenario.Job("job-name").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			_, err = scenario.Job("some-other-job").CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			dbBuild, found, err := buildFactory.Build(build.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbBuild.SaveOutput("some-type", nil, atc.Source{"some": "source"}, atc.Version{"version": "v1"}, []db.ResourceConfigMetadataField{
				{
					Name:  "some",
					Value: "value",
				},
			}, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			dbSecondBuild, found, err := buildFactory.Build(secondBuild.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbSecondBuild.SaveOutput("some-type", nil, atc.Source{"some": "source"}, atc.Version{"version": "v1"}, []db.ResourceConfigMetadataField{
				{
					Name:  "some",
					Value: "value",
				},
			}, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			err = dbSecondBuild.SaveOutput("some-type", nil, atc.Source{"some": "source"}, atc.Version{"version": "v3"}, nil, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			rcv1 := scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"})

			resourceConfigVersion = rcv1.ID()
		})

		It("returns the two builds for which the provided version id was an output", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(scenario.Resource("some-resource").ID(), resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns the one build that uses the version as an input", func() {
			rcv3 := scenario.ResourceVersion("some-resource", atc.Version{"version": "v3"})

			builds, err := pipeline.GetBuildsWithVersionAsOutput(scenario.Resource("some-resource").ID(), rcv3.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(HaveLen(1))
			Expect(builds[0].ID()).To(Equal(secondBuild.ID()))
		})

		It("returns an empty slice of builds when the provided version id exists but is not used", func() {
			rcv4 := scenario.ResourceVersion("some-resource", atc.Version{"version": "v4"})

			builds, err := pipeline.GetBuildsWithVersionAsOutput(scenario.Resource("some-resource").ID(), rcv4.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided resource id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(10293912, resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(scenario.Resource("some-resource").ID(), resourceConfigVersion+100)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})

	Describe("Builds", func() {
		var expectedBuilds []db.Build

		BeforeEach(func() {
			_, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			thirdBuild, err := someOtherJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, thirdBuild)
		})

		It("returns builds for the current pipeline", func() {
			builds, _, err := pipeline.Builds(db.Page{Limit: 10})
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})
	})

	Describe("CreateStartedBuild", func() {
		var (
			plan         atc.Plan
			startedBuild db.Build
			err          error
		)

		BeforeEach(func() {
			plan = atc.Plan{
				ID: atc.PlanID("56"),
				Get: &atc.GetPlan{
					Type:     "some-type",
					Name:     "some-name",
					Resource: "some-resource",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Version:  &atc.Version{"some": "version"},
					Tags:     atc.Tags{"some-tags"},
				},
			}

			startedBuild, err = pipeline.CreateStartedBuild(plan)
			Expect(err).ToNot(HaveOccurred())
		})

		It("can create started builds with plans", func() {
			Expect(startedBuild.ID()).ToNot(BeZero())
			Expect(startedBuild.JobName()).To(BeZero())
			Expect(startedBuild.PipelineName()).To(Equal("fake-pipeline"))
			Expect(startedBuild.Name()).To(Equal(strconv.Itoa(startedBuild.ID())))
			Expect(startedBuild.TeamName()).To(Equal(team.Name()))
			Expect(startedBuild.Status()).To(Equal(db.BuildStatusStarted))
		})

		It("saves the public plan", func() {
			found, err := startedBuild.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(startedBuild.PublicPlan()).To(Equal(plan.Public()))
		})

		It("creates Start event", func() {
			found, err := startedBuild.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			events, err := startedBuild.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   startedBuild.StartTime().Unix(),
			}, "0")))
		})
	})

	Describe("ResourceTypes", func() {
		var resourceTypes db.ResourceTypes
		var scenario *dbtest.Scenario

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					ResourceTypes: atc.ResourceTypes{
						{
							Name:   "some-other-resource-type",
							Type:   "global-base-type",
							Source: atc.Source{"some": "other-type-soure"},
						},
						{
							Name:   "some-resource-type",
							Type:   "global-base-type",
							Source: atc.Source{"some": "type-soure"},
						},
					},
				}),
			)
		})

		JustBeforeEach(func() {
			var err error
			resourceTypes, err = scenario.Pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the resource types", func() {
			var rtNames []string
			for _, rt := range resourceTypes {
				rtNames = append(rtNames, rt.Name())
			}
			Expect(rtNames).To(ConsistOf("some-other-resource-type", "some-resource-type"))
		})
	})

	Describe("ResourceVersion", func() {
		var (
			rv                    atc.ResourceVersion
			resourceConfigVersion db.ResourceConfigVersion
			scenario              *dbtest.Scenario
		)

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "some-type",
							Source: atc.Source{"some": "source"},
						},
					},
				}),
				builder.WithResourceVersions("some-resource",
					atc.Version{"version": "1"},
				),
			)

			resourceConfigVersion = scenario.ResourceVersion("some-resource", atc.Version{"version": "1"})
		})

		JustBeforeEach(func() {
			var found bool
			var err error

			rv, found, err = pipeline.ResourceVersion(resourceConfigVersion.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when a resource is enabled", func() {
			It("should return the version with enabled set to true", func() {
				Expect(rv).To(Equal(atc.ResourceVersion{
					Version: atc.Version{"version": "1"},
					ID:      resourceConfigVersion.ID(),
					Enabled: true,
				}))
			})
		})

		Context("when a resource is not enabled", func() {
			BeforeEach(func() {
				err := scenario.Resource("some-resource").DisableVersion(resourceConfigVersion.ID())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the version with enabled set to false", func() {
				Expect(rv).To(Equal(atc.ResourceVersion{
					Version: atc.Version{"version": "1"},
					ID:      resourceConfigVersion.ID(),
					Enabled: false,
				}))
			})
		})
	})

	Describe("BuildsWithTime", func() {
		var (
			pipeline db.Pipeline
			builds   = make([]db.Build, 4)
			job      db.Job
		)

		BeforeEach(func() {
			var (
				err   error
				found bool
			)

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
			}
			pipeline, _, err = team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, config, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			for i := range builds {
				builds[i], err = job.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				buildStart := time.Date(2020, 11, i+1, 0, 0, 0, 0, time.UTC)
				_, err = dbConn.Exec("UPDATE builds SET start_time = to_timestamp($1) WHERE id = $2", buildStart.Unix(), builds[i].ID())
				Expect(err).NotTo(HaveOccurred())

				builds[i], found, err = job.Build(builds[i].Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			}

			otherPipeline, _, err := team.SavePipeline(atc.PipelineRef{Name: "another-pipeline"}, config, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err := otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = otherJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when not providing boundaries", func() {
			Context("without a limit specified", func() {
				It("returns no builds", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{})
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedBuilds).To(BeEmpty())
				})
			})

			Context("with a limit specified", func() {
				It("returns a subset of the builds", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						Limit: 2,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[3], builds[2]))
				})
			})
		})

		Context("when providing boundaries", func() {

			Context("only to", func() {
				It("returns only those before and including to", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						To:    db.NewIntPtr(int(builds[2].StartTime().Unix())),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[0], builds[1], builds[2]))
				})
			})

			Context("only from", func() {
				It("returns only those after from", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						From:  db.NewIntPtr(int(builds[1].StartTime().Unix())),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2], builds[3]))
				})
			})

			Context("from and to", func() {
				It("returns only elements in the range", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						From:  db.NewIntPtr(int(builds[1].StartTime().Unix())),
						To:    db.NewIntPtr(int(builds[2].StartTime().Unix())),
						Limit: 50,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2]))
				})
			})
		})
	})

	Describe("Variables", func() {
		var (
			fakeGlobalSecrets *credsfakes.FakeSecrets
			pool              creds.VarSourcePool

			pvars vars.Variables
			err   error
		)

		BeforeEach(func() {
			credentialManagement := creds.CredentialManagementConfig{
				RetryConfig: creds.SecretRetryConfig{
					Attempts: 5,
					Interval: time.Second,
				},
				CacheConfig: creds.SecretCacheConfig{
					Enabled:          true,
					Duration:         time.Minute,
					DurationNotFound: time.Minute,
					PurgeInterval:    time.Minute * 10,
				},
			}
			pool = creds.NewVarSourcePool(logger, credentialManagement, 1*time.Minute, 1*time.Second, clock.NewClock())
		})

		AfterEach(func() {
			pool.Close()
		})

		JustBeforeEach(func() {
			fakeGlobalSecrets = new(credsfakes.FakeSecrets)
			fakeGlobalSecrets.GetStub = func(key string) (any, *time.Time, bool, error) {
				if key == "gk" {
					return "gv", nil, true, nil
				}
				return nil, nil, false, nil
			}

			params := creds.SecretLookupParams{
				Team:         pipeline.TeamName(),
				Pipeline:     pipeline.Name(),
				InstanceVars: pipeline.InstanceVars(),
			}

			pvars, err = pipeline.Variables(logger, fakeGlobalSecrets, pool, params)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should get var from pipeline var source", func() {
			v, found, err := pvars.Get(vars.Reference{Source: "some-var-source", Path: "pk"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(v.(string)).To(Equal("pv"))
		})

		It("should not get pipeline var 'pk' without specifying var_source name", func() {
			_, found, err := pvars.Get(vars.Reference{Path: "pk"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("should not get from global secrets if found in the pipeline var source", func() {
			pvars.Get(vars.Reference{Source: "some-var-source", Path: "pk"})
			Expect(fakeGlobalSecrets.GetCallCount()).To(Equal(0))
		})

		It("should get var from global var source", func() {
			v, found, err := pvars.Get(vars.Reference{Path: "gk"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(v.(string)).To(Equal("gv"))
		})

		It("should not get var 'foo'", func() {
			_, found, err := pvars.Get(vars.Reference{Path: "foo"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		Context("with the second var_source", func() {
			BeforeEach(func() {
				pipelineConfig.VarSources = append(pipelineConfig.VarSources, atc.VarSourceConfig{
					Name: "second-var-source",
					Type: "dummy",
					Config: map[string]any{
						"vars": map[string]any{"pk": "((some-var-source:pk))"},
					},
				})

				var created bool
				pipeline, created, err = team.SavePipeline(atc.PipelineRef{Name: "fake-pipeline"}, pipelineConfig, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeFalse())
			})

			// The second var source is configured with vars that needs to be interpolated
			// from "some-var-source".
			It("should get pipeline var 'pk' from the second var_source", func() {
				v, found, err := pvars.Get(vars.Reference{Source: "second-var-source", Path: "pk"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("pv"))
			})
		})
	})

	Describe("SetParentIDs", func() {
		It("sets the parent_job_id and parent_build_id fields", func() {
			jobID := 123
			buildID := 456
			Expect(pipeline.SetParentIDs(jobID, buildID)).To(Succeed())

			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pipeline.ParentJobID()).To(Equal(jobID))
			Expect(pipeline.ParentBuildID()).To(Equal(buildID))
		})

		It("returns an error if job or build ID are less than or equal to zero", func() {
			err := pipeline.SetParentIDs(0, 0)
			Expect(err).To(MatchError("job and build id cannot be negative or zero-value"))
			err = pipeline.SetParentIDs(-1, -6)
			Expect(err).To(MatchError("job and build id cannot be negative or zero-value"))
		})

		Context("pipeline was saved by a newer build", func() {
			It("returns ErrSetByNewerBuild", func() {
				By("setting the build ID to a high number")
				pipeline.SetParentIDs(1, 60)

				By("trying to set the build ID to a lower number")
				err := pipeline.SetParentIDs(1, 2)
				Expect(err).To(MatchError(db.ErrSetByNewerBuild))
			})
		})

		Context("pipeline was previously saved by team.SavePipeline", func() {
			It("successfully updates the parent build and job IDs", func() {
				By("using the defaultPipeline saved by defaultTeam at the suite level")
				Expect(defaultPipeline.ParentJobID()).To(Equal(0), "should be zero if sql value is null")
				Expect(defaultPipeline.ParentBuildID()).To(Equal(0), "should be zero if sql value is null")

				err := defaultPipeline.SetParentIDs(1, 6)
				Expect(err).ToNot(HaveOccurred())
				defaultPipeline.Reload()
				Expect(defaultPipeline.ParentJobID()).To(Equal(1), "should be zero if sql value is null")
				Expect(defaultPipeline.ParentBuildID()).To(Equal(6), "should be zero if sql value is null")
			})
		})
	})

	Context("Config", func() {
		It("should return config correctly", func() {
			Expect(pipeline.Config()).To(Equal(pipelineConfig))
		})
	})
})

func intptr(i int) *int {
	return &i
}
