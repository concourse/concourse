package db_test

import (
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline       db.Pipeline
		team           db.Team
		pipelineConfig atc.Config
		job            db.Job
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
			Jobs: atc.JobConfigs{
				{
					Name: "job-name",

					Public: true,

					Serial: true,

					SerialGroups: []string{"serial-group"},

					Plan: atc.PlanSequence{
						{
							Put: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
						},
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
								RootfsURI: "some-image",
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
			},
			Resources: atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "other-source"},
				},
			},
			ResourceTypes: atc.ResourceTypes{
				{
					Name:   "some-resource-type",
					Type:   "base-type",
					Source: atc.Source{"some": "type-soure"},
				},
				{
					Name:   "some-other-resource-type",
					Type:   "base-type",
					Source: atc.Source{"some": "other-type-soure"},
				},
			},
		}
		var created bool
		pipeline, created, err = team.SavePipeline("fake-pipeline", pipelineConfig, db.ConfigVersion(0), db.PipelineUnpaused)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())

		var found bool
		job, found, err = pipeline.Job("job-name")
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
				Expect(pipeline.Pause()).To(Succeed())
			})

			It("returns the pipeline is paused", func() {
				Expect(paused).To(BeTrue())
			})
		})
	})

	Describe("Pause", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Pause()).To(Succeed())

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

			It("nulls out resource_config_id for all resources", func() {
				resource, found, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(resource.ResourceConfigID()).To(BeZero())

				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(resource.ResourceConfigID()).To(BeZero())
			})
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
				Expect(pipeline.Pause()).To(Succeed())
			})

			It("unpauses the pipeline", func() {
				Expect(pipeline.Paused()).To(BeFalse())
			})
		})
	})

	Describe("Rename", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Rename("oopsies")).To(Succeed())
		})

		It("renames the pipeline", func() {
			pipeline, found, err := team.Pipeline("oopsies")
			Expect(pipeline.Name()).To(Equal("oopsies"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Resource Config Versions", func() {
		resourceName := "some-resource"
		otherResourceName := "some-other-resource"
		reallyOtherResourceName := "some-really-other-resource"

		var (
			dbPipeline               db.Pipeline
			otherDBPipeline          db.Pipeline
			resource                 db.Resource
			otherResource            db.Resource
			reallyOtherResource      db.Resource
			resourceConfigScope      db.ResourceConfigScope
			otherResourceConfigScope db.ResourceConfigScope
			otherPipelineResource    db.Resource
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

						Plan: atc.PlanSequence{
							{
								Put: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
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
									RootfsURI: "some-image",
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
				},
			}

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

			var err error
			dbPipeline, _, err = team.SavePipeline("pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherDBPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = dbPipeline.Resource(resourceName)
			Expect(err).ToNot(HaveOccurred())

			otherResource, _, err = dbPipeline.Resource(otherResourceName)
			Expect(err).ToNot(HaveOccurred())

			reallyOtherResource, _, err = dbPipeline.Resource(reallyOtherResourceName)
			Expect(err).ToNot(HaveOccurred())

			otherPipelineResource, _, err = otherDBPipeline.Resource(otherResourceName)
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = resource.SetResourceConfig(logger, atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			otherResourceConfigScope, err = otherPipelineResource.SetResourceConfig(logger, atc.Source{"other-source-config": "some-other-value"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			_, err = reallyOtherResource.SetResourceConfig(logger, atc.Source{"source-config": "some-really-other-value"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns correct resource", func() {
			Expect(resource.Name()).To(Equal("some-resource"))
			Expect(resource.PipelineName()).To(Equal("pipeline-name"))
			Expect(resource.CheckSetupError()).To(BeNil())
			Expect(resource.CheckError()).To(BeNil())
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{"source-config": "some-value"}))
		})

		It("can load up resource config version information relevant to scheduling", func() {
			job, found, err := dbPipeline.Job("some-job")
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err := dbPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := dbPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := dbPipeline.Job("shared-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := dbPipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := dbPipeline.Job("other-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := dbPipeline.Job("different-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err := dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(BeEmpty())
			Expect(versions.BuildOutputs).To(BeEmpty())
			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"some-other-job":             otherJob.ID(),
				"a-job":                      aJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("initially having no latest versioned resource")
			_, found, err = resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = resourceConfigScope.SaveVersions([]atc.Version{atc.Version{"version": "1"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR1, found, err := resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resourceConfigScope.SaveVersions([]atc.Version{atc.Version{"version": "2"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR2, found, err := resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID(), ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder()},
				{VersionID: savedVR2.ID(), ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder()},
			}))

			Expect(versions.BuildOutputs).To(BeEmpty())
			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"some-other-job":             otherJob.ID(),
				"a-job":                      aJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("not including saved versioned resources of other pipelines")
			err = otherResourceConfigScope.SaveVersions([]atc.Version{atc.Version{"version": "1"}})
			Expect(err).ToNot(HaveOccurred())

			_, found, err = otherResourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID(), ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder()},
				{VersionID: savedVR2.ID(), ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder()},
			}))

			Expect(versions.BuildOutputs).To(BeEmpty())
			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"some-other-job":             otherJob.ID(),
				"a-job":                      aJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("including outputs of successful builds")
			build1DB, err := aJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.SaveOutput(logger, "some-type", atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{}, atc.Version{"version": "1"}, nil, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID(), ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder()},
				{VersionID: savedVR2.ID(), ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder()},
			}))

			explicitOutput := algorithm.BuildOutput{
				ResourceVersion: algorithm.ResourceVersion{
					VersionID:  savedVR1.ID(),
					ResourceID: resource.ID(),
					CheckOrder: savedVR1.CheckOrder(),
				},
				JobID:   aJob.ID(),
				BuildID: build1DB.ID(),
			}

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				explicitOutput,
			}))

			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"a-job":                      aJob.ID(),
				"some-other-job":             otherJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("not including outputs of failed builds")
			build2DB, err := aJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.SaveOutput(logger, "some-type", atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{}, atc.Version{"version": "1"}, nil, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID(), ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder()},
				{VersionID: savedVR2.ID(), ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder()},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID(),
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder(),
					},
					JobID:   aJob.ID(),
					BuildID: build1DB.ID(),
				},
			}))

			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"a-job":                      aJob.ID(),
				"some-other-job":             otherJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("not including outputs of builds in other pipelines")
			anotherJob, found, err := otherDBPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherPipelineBuild, err := anotherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = otherPipelineBuild.SaveOutput(logger, "some-type", atc.Source{"other-source-config": "some-other-value"}, creds.VersionedResourceTypes{}, atc.Version{"version": "1"}, nil, "some-output-name", "some-other-resource")
			Expect(err).ToNot(HaveOccurred())

			err = otherPipelineBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID(), ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder()},
				{VersionID: savedVR2.ID(), ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder()},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID(),
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder(),
					},
					JobID:   aJob.ID(),
					BuildID: build1DB.ID(),
				},
			}))

			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"a-job":                      aJob.ID(),
				"some-other-job":             otherJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("including build inputs")
			aJob, found, err = dbPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build1DB, err = aJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.UseInputs([]db.BuildInput{
				db.BuildInput{
					Name:       "some-input-name",
					Version:    atc.Version{"version": "1"},
					ResourceID: resource.ID(),
				},
			})
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())

			Expect(versions.BuildInputs).To(ConsistOf([]algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID(),
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder(),
					},
					JobID:     aJob.ID(),
					BuildID:   build1DB.ID(),
					InputName: "some-input-name",
				},
			}))

			By("including implicit outputs of successful builds")
			implicitOutput := algorithm.BuildOutput{
				ResourceVersion: algorithm.ResourceVersion{
					VersionID:  savedVR1.ID(),
					ResourceID: resource.ID(),
					CheckOrder: savedVR1.CheckOrder(),
				},
				JobID:   aJob.ID(),
				BuildID: build1DB.ID(),
			}

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				explicitOutput,
				implicitOutput,
			}))
		})

		It("can load up the latest versioned resource, enabled or not", func() {
			By("initially having no latest versioned resource")
			_, found, err := resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR1, found, err := resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "2"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR2, found, err := resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(savedVR1.Version()).To(Equal(db.Version{"version": "1"}))
			Expect(savedVR2.Version()).To(Equal(db.Version{"version": "2"}))

			By("not including saved versioned resources of other pipelines")
			_, _, err = otherDBPipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())

			err = otherResourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}, {"version": "2"}, {"version": "3"}})
			Expect(err).ToNot(HaveOccurred())

			otherPipelineSavedVR, found, err := otherResourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(otherPipelineSavedVR.Version()).To(Equal(db.Version{"version": "3"}))

			By("including disabled versions")
			err = resource.DisableVersion(savedVR2.ID())
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err := resourceConfigScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version()).To(Equal(db.Version{"version": "2"}))
		})

		Describe("enabling and disabling versioned resources", func() {
			It("returns an error if the version is bogus", func() {
				err := resource.EnableVersion(42)
				Expect(err).To(HaveOccurred())

				err = resource.DisableVersion(42)
				Expect(err).To(HaveOccurred())
			})

			It("does not affect explicitly fetching the latest version", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				savedRCV, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedRCV.Version()).To(Equal(db.Version{"version": "1"}))

				err = resource.DisableVersion(savedRCV.ID())
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Version()).To(Equal(db.Version{"version": "1"}))

				err = resource.EnableVersion(savedRCV.ID())
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err = resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Version()).To(Equal(db.Version{"version": "1"}))
			})

			It("doesn't change the check_order when saving a new build input", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).ToNot(HaveOccurred())

				job, found, err := dbPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions([]atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).ToNot(HaveOccurred())

				input := db.BuildInput{
					Name:       "input-name",
					Version:    atc.Version{"version": "3"},
					ResourceID: resource.ID(),
				}

				err = build.UseInputs([]db.BuildInput{input})
				Expect(err).ToNot(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new build output", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).ToNot(HaveOccurred())

				job, found, err := dbPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				beforeVR, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = resourceConfigScope.SaveVersions([]atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput(logger, "some-type", atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{}, atc.Version(beforeVR.Version()), nil, "some-output-name", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				versions, _, found, err := resource.Versions(db.Page{Limit: 10})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(HaveLen(5))
				Expect(versions[0].Version).To(Equal(atc.Version{"version": "5"}))
				Expect(versions[1].Version).To(Equal(atc.Version{"version": "4"}))
				Expect(versions[2].Version).To(Equal(atc.Version{"version": "3"}))
				Expect(versions[3].Version).To(Equal(atc.Version{"version": "2"}))
				Expect(versions[4].Version).To(Equal(atc.Version{"version": "1"}))
			})
		})

		Describe("saving versioned resources", func() {
			It("updates the latest versioned resource", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				savedResource, _, err := dbPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				resourceConfigScope, err = savedResource.SetResourceConfig(logger, atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Version()).To(Equal(db.Version{"version": "1"}))

				err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "2"}, {"version": "3"}})
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err = resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Version()).To(Equal(db.Version{"version": "3"}))
			})
		})

		It("initially has no pending build for a job", func() {
			job, found, err := dbPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pendingBuilds, err := job.GetPendingBuilds()
			Expect(err).ToNot(HaveOccurred())
			Expect(pendingBuilds).To(HaveLen(0))
		})
	})

	Describe("Disable and Enable Resource Versions", func() {
		var pipelineDB db.Pipeline
		var resource db.Resource
		var resourceConfigScope db.ResourceConfigScope

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "a-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some-source": "some-value"},
					},
				},
			}
			var err error
			pipelineDB, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			resource, found, err = pipelineDB.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope, err = resource.SetResourceConfig(logger, atc.Source{"some-source": "some-value"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when a version is disabled", func() {
			It("omits the version from the versions DB", func() {
				aJob, found, err := pipelineDB.Job("a-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build1, err := aJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "disabled"}})
				Expect(err).ToNot(HaveOccurred())

				disabledVersion, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				disabledInput := db.BuildInput{
					Name:       "disabled-input",
					Version:    atc.Version{"version": "disabled"},
					ResourceID: resource.ID(),
				}

				err = build1.SaveOutput(logger, "some-type", atc.Source{"some-source": "some-value"}, creds.VersionedResourceTypes{}, atc.Version{"version": "disabled"}, nil, "some-output-name", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "enabled"}})
				Expect(err).ToNot(HaveOccurred())

				enabledVersion, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "other-enabled"}})
				Expect(err).ToNot(HaveOccurred())

				otherEnabledVersion, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				enabledInput := db.BuildInput{
					Name:       "enabled-input",
					Version:    atc.Version{"version": "enabled"},
					ResourceID: resource.ID(),
				}
				err = build1.UseInputs([]db.BuildInput{disabledInput, enabledInput})
				Expect(err).ToNot(HaveOccurred())

				Expect(err).ToNot(HaveOccurred())

				err = build1.SaveOutput(logger, "some-type", atc.Source{"some-source": "some-value"}, creds.VersionedResourceTypes{}, atc.Version{"version": "other-enabled"}, nil, "some-output-name", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				err = build1.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				err = resource.DisableVersion(disabledVersion.ID())
				Expect(err).ToNot(HaveOccurred())

				err = resource.DisableVersion(enabledVersion.ID())
				Expect(err).ToNot(HaveOccurred())

				err = resource.EnableVersion(enabledVersion.ID())
				Expect(err).ToNot(HaveOccurred())

				versions, err := pipelineDB.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				aJob, found, err = pipelineDB.Job("a-job")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				By("omitting it from the list of resource versions")
				Expect(versions.ResourceVersions).To(ConsistOf(
					algorithm.ResourceVersion{
						VersionID:  enabledVersion.ID(),
						ResourceID: resource.ID(),
						CheckOrder: enabledVersion.CheckOrder(),
					},
					algorithm.ResourceVersion{
						VersionID:  otherEnabledVersion.ID(),
						ResourceID: resource.ID(),
						CheckOrder: otherEnabledVersion.CheckOrder(),
					},
				))

				By("omitting it from build outputs")
				Expect(versions.BuildOutputs).To(ConsistOf(
					// explicit output
					algorithm.BuildOutput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  otherEnabledVersion.ID(),
							ResourceID: resource.ID(),
							CheckOrder: otherEnabledVersion.CheckOrder(),
						},
						JobID:   aJob.ID(),
						BuildID: build1.ID(),
					},
					// implicit output
					algorithm.BuildOutput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  enabledVersion.ID(),
							ResourceID: resource.ID(),
							CheckOrder: enabledVersion.CheckOrder(),
						},
						JobID:   aJob.ID(),
						BuildID: build1.ID(),
					},
				))

				By("omitting it from build inputs")
				Expect(versions.BuildInputs).To(ConsistOf(
					algorithm.BuildInput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  enabledVersion.ID(),
							ResourceID: resource.ID(),
							CheckOrder: enabledVersion.CheckOrder(),
						},
						JobID:     aJob.ID(),
						BuildID:   build1.ID(),
						InputName: "enabled-input",
					},
				))
			})
		})
	})

	Describe("Destroy", func() {
		var resourceConfigScope db.ResourceConfigScope

		It("removes the pipeline and all of its data", func() {
			By("populating resources table")
			resource, found, err := pipeline.Resource("some-resource")
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			By("populating resource versions")
			err = resourceConfigScope.SaveVersions([]atc.Version{
				{
					"key": "value",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			By("populating builds")
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			By("populating build inputs")
			err = build.UseInputs([]db.BuildInput{
				db.BuildInput{
					Name:       "build-input",
					ResourceID: resource.ID(),
					Version:    atc.Version{"key": "value"},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			By("populating build outputs")
			err = build.SaveOutput(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{}, atc.Version{"key": "value"}, nil, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			By("populating build events")
			err = build.SaveEvent(event.StartTask{})
			Expect(err).ToNot(HaveOccurred())

			err = pipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())

			found, err = pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			found, err = build.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = team.Pipeline(pipeline.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("GetPendingBuilds/GetAllPendingBuilds", func() {
		Context("when a build is created", func() {
			BeforeEach(func() {
				_, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the build", func() {
				pendingBuildsForJob, err := job.GetPendingBuilds()
				Expect(err).ToNot(HaveOccurred())
				Expect(pendingBuildsForJob).To(HaveLen(1))

				pendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).ToNot(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))
				Expect(pendingBuilds["job-name"]).ToNot(BeNil())
			})
		})
	})

	Describe("VersionsDB caching", func() {
		var otherPipeline db.Pipeline
		BeforeEach(func() {
			otherPipelineConfig := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"some-source": "some-other-value",
						},
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
			}
			var err error
			otherPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when build outputs are added", func() {
			var build db.Build
			var savedVR db.ResourceConfigVersion
			var resourceConfigScope db.ResourceConfigScope
			var savedResource db.Resource

			BeforeEach(func() {
				var err error
				job, found, err := pipeline.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				savedResource, _, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				resourceConfigScope, err = savedResource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err = resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("will cache VersionsDB if no change has occured", func() {
				err := build.SaveOutput(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{}, atc.Version(savedVR.Version()), nil, "some-output-name", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
			})

			It("will not cache VersionsDB if a build has completed", func() {
				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				err = build.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			It("will not cache VersionsDB if a resource version is disabled or enabled", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				rcv, found, err := resourceConfigScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = savedResource.DisableVersion(rcv.ID())
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")

				err = savedResource.EnableVersion(rcv.ID())
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB2, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(cachedVersionsDB != cachedVersionsDB2).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			Context("when the build outputs are added for a different pipeline", func() {
				It("does not invalidate the cache for the original pipeline", func() {
					job, found, err := otherPipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					otherBuild, err := job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					otherSavedResource, _, err := otherPipeline.Resource("some-other-resource")
					Expect(err).ToNot(HaveOccurred())

					otherResourceConfigScope, err := otherSavedResource.SetResourceConfig(logger, atc.Source{"some-source": "some-other-value"}, creds.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())

					otherResourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
					Expect(err).ToNot(HaveOccurred())

					otherSavedVR, found, err := otherResourceConfigScope.LatestVersion()
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					versionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())

					err = otherBuild.SaveOutput(logger, "some-type", atc.Source{"some-source": "some-other-value"}, creds.VersionedResourceTypes{}, atc.Version(otherSavedVR.Version()), nil, "some-output-name", "some-other-resource")
					Expect(err).ToNot(HaveOccurred())

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())
					Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
				})
			})
		})

		Context("when versioned resources are added", func() {
			var resourceConfigScope db.ResourceConfigScope
			var otherResourceConfigScope db.ResourceConfigScope
			var resource db.Resource

			BeforeEach(func() {
				var err error
				resource, _, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				otherResource, _, err := pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())

				resourceConfigScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				otherResourceConfigScope, err = otherResource.SetResourceConfig(logger, atc.Source{"some": "other-source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("will cache VersionsDB if no change has occured", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
			})

			It("will not cache VersionsDB if a change occured", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				err = otherResourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			It("will not cache versions whose check order is zero", func() {
				err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "2"}})
				Expect(err).ToNot(HaveOccurred())

				By("creating a new version but not updating the check order yet")
				created, err := resource.SaveUncheckedVersion(atc.Version{"version": "1"}, nil, resourceConfigScope.ResourceConfig(), creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = build.UseInputs([]db.BuildInput{{Name: "some-resource", Version: atc.Version{"version": "1"}, ResourceID: resource.ID()}})
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB.ResourceVersions).To(HaveLen(1))
				Expect(versionsDB.BuildInputs).To(HaveLen(0))
				Expect(versionsDB.BuildOutputs).To(HaveLen(0))
			})

			Context("when the versioned resources are added for a different pipeline", func() {
				It("does not invalidate the cache for the original pipeline", func() {
					err := resourceConfigScope.SaveVersions([]atc.Version{{"version": "1"}})
					Expect(err).ToNot(HaveOccurred())

					versionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())

					otherPipelineResource, _, err := otherPipeline.Resource("some-other-resource")
					Expect(err).ToNot(HaveOccurred())

					otherPipelineResourceConfig, err := otherPipelineResource.SetResourceConfig(logger, atc.Source{"some-source": "some-other-value"}, creds.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())

					err = otherPipelineResourceConfig.SaveVersions([]atc.Version{{"version": "1"}})
					Expect(err).ToNot(HaveOccurred())

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())
					Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
				})
			})
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

			otherSerialGroupJob, found, err := pipeline.Job("other-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := pipeline.Job("different-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			By("returning jobs with no builds")
			actualDashboard, err := pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[1].Job.Name()).To(Equal(otherJob.Name()))
			Expect(actualDashboard[2].Job.Name()).To(Equal(aJob.Name()))
			Expect(actualDashboard[3].Job.Name()).To(Equal(sharedJob.Name()))
			Expect(actualDashboard[4].Job.Name()).To(Equal(randomJob.Name()))
			Expect(actualDashboard[5].Job.Name()).To(Equal(otherSerialGroupJob.Name()))
			Expect(actualDashboard[6].Job.Name()).To(Equal(differentSerialGroupJob.Name()))

			By("returning a job's most recent pending build if there are no running builds")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			firstJobBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(firstJobBuild.ID()))

			By("returning a job's most recent started build")
			found, err = firstJobBuild.Start(atc.Plan{ID: "some-id"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			found, err = firstJobBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(firstJobBuild.ID()))
			Expect(actualDashboard[0].NextBuild.Status()).To(Equal(db.BuildStatusStarted))
			Expect(actualDashboard[0].NextBuild.Schema()).To(Equal("exec.v2"))
			Expect(actualDashboard[0].NextBuild.PrivatePlan()).To(Equal(atc.Plan{ID: "some-id"}))

			By("returning a job's most recent started build even if there is a newer pending build")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			secondJobBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, err = pipeline.Dashboard()
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(firstJobBuild.ID()))

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

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild).To(BeNil())
			Expect(actualDashboard[0].FinishedBuild.ID()).To(Equal(secondJobBuild.ID()))
		})
	})

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.SaveEvent(event.Log{
				Payload: "log 1",
			})
			Expect(err).ToNot(HaveOccurred())

			build2DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).ToNot(HaveOccurred())

			build3DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build3DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			build4DB, err := team.CreateOneOffBuild()
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
			})))

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
			Expect(jobs[5].Name()).To(Equal("other-serial-group-job"))
			Expect(jobs[6].Name()).To(Equal("different-serial-group-job"))
		})
	})

	Describe("GetBuildsWithVersionAsInput", func() {
		var (
			resourceConfigVersion int
			expectedBuilds        []db.Build
			resource              db.Resource
			dbSecondBuild         db.Build
			resourceConfigScope   db.ResourceConfigScope
		)

		BeforeEach(func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()

			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = someOtherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			dbBuild, found, err := buildFactory.Build(build.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfigScope.SaveVersions([]atc.Version{atc.Version{"version": "v1"}})
			Expect(err).ToNot(HaveOccurred())

			err = dbBuild.UseInputs([]db.BuildInput{
				db.BuildInput{
					Name: "some-input",
					Version: atc.Version{
						"version": "v1",
					},
					ResourceID:      resource.ID(),
					FirstOccurrence: true,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			dbSecondBuild, found, err = buildFactory.Build(secondBuild.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			inputs1 := db.BuildInput{
				Name: "some-input",
				Version: atc.Version{
					"version": "v1",
				},
				ResourceID:      resource.ID(),
				FirstOccurrence: true,
			}

			err = resourceConfigScope.SaveVersions([]atc.Version{
				{"version": "v2"},
				{"version": "v3"},
				{"version": "v4"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = dbSecondBuild.UseInputs([]db.BuildInput{
				inputs1,
				db.BuildInput{
					Name: "some-input",
					Version: atc.Version{
						"version": "v3",
					},
					ResourceID:      resource.ID(),
					FirstOccurrence: true,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			rcv1, found, err := resourceConfigScope.FindVersion(atc.Version{"version": "v1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigVersion = rcv1.ID()
		})

		It("returns the two builds for which the provided version id was an input", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(resource.ID(), resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns the one build that uses the version as an input", func() {
			rcv3, found, err := resourceConfigScope.FindVersion(atc.Version{"version": "v3"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			builds, err := pipeline.GetBuildsWithVersionAsInput(resource.ID(), rcv3.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(HaveLen(1))
			Expect(builds[0]).To(Equal(dbSecondBuild))
		})

		It("returns an empty slice of builds when the provided version id exists but is not used", func() {
			rcv4, found, err := resourceConfigScope.FindVersion(atc.Version{"version": "v4"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			builds, err := pipeline.GetBuildsWithVersionAsInput(resource.ID(), rcv4.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(resource.ID(), resourceConfigVersion+100)
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
			resourceConfigScope   db.ResourceConfigScope
			resource              db.Resource
			secondBuild           db.Build
		)

		BeforeEach(func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = someOtherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			dbBuild, found, err := buildFactory.Build(build.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfigScope.SaveVersions([]atc.Version{
				{"version": "v3"},
				{"version": "v4"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = dbBuild.SaveOutput(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{}, atc.Version{"version": "v1"}, []db.ResourceConfigMetadataField{
				{
					Name:  "some",
					Value: "value",
				},
			}, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			dbSecondBuild, found, err := buildFactory.Build(secondBuild.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbSecondBuild.SaveOutput(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{}, atc.Version{"version": "v1"}, []db.ResourceConfigMetadataField{
				{
					Name:  "some",
					Value: "value",
				},
			}, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			err = dbSecondBuild.SaveOutput(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{}, atc.Version{"version": "v3"}, nil, "some-output-name", "some-resource")
			Expect(err).ToNot(HaveOccurred())

			rcv1, found, err := resourceConfigScope.FindVersion(atc.Version{"version": "v1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigVersion = rcv1.ID()
		})

		It("returns the two builds for which the provided version id was an output", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(resource.ID(), resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns the one build that uses the version as an input", func() {
			rcv3, found, err := resourceConfigScope.FindVersion(atc.Version{"version": "v3"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			builds, err := pipeline.GetBuildsWithVersionAsOutput(resource.ID(), rcv3.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(HaveLen(1))
			Expect(builds[0].ID()).To(Equal(secondBuild.ID()))
		})

		It("returns an empty slice of builds when the provided version id exists but is not used", func() {
			rcv4, found, err := resourceConfigScope.FindVersion(atc.Version{"version": "v4"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			builds, err := pipeline.GetBuildsWithVersionAsOutput(resource.ID(), rcv4.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided resource id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(10293912, resourceConfigVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(resource.ID(), resourceConfigVersion+100)
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

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			thirdBuild, err := someOtherJob.CreateBuild()
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
					VersionedResourceTypes: atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name:       "some-name",
								Source:     atc.Source{"some": "source"},
								Type:       "some-type",
								Privileged: true,
								Tags:       atc.Tags{"some-tags"},
							},
							Version: atc.Version{"some-resource-type": "version"},
						},
					},
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
			})))
		})
	})

	Describe("Resources", func() {
		var resourceTypes db.ResourceTypes

		BeforeEach(func() {
			var err error
			resourceType, _, err := pipeline.ResourceType("some-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceType.Version()).To(BeNil())

			otherResourceType, _, err := pipeline.ResourceType("some-other-resource-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceType.Version()).To(BeNil())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "base-type",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceTypeScope, err := resourceType.SetResourceConfig(logger, atc.Source{"some": "type-source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceTypeScope.SaveVersions([]atc.Version{
				atc.Version{"version": "1"},
				atc.Version{"version": "2"},
			})
			Expect(err).ToNot(HaveOccurred())

			otherResourceTypeScope, err := otherResourceType.SetResourceConfig(logger, atc.Source{"some": "other-type-source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = otherResourceTypeScope.SaveVersions([]atc.Version{
				atc.Version{"version": "3"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = otherResourceTypeScope.SaveVersions([]atc.Version{
				atc.Version{"version": "3"},
				atc.Version{"version": "5"},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			resourceTypes, err = pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the version", func() {
			resourceTypeVersions := []atc.Version{resourceTypes[0].Version()}
			resourceTypeVersions = append(resourceTypeVersions, resourceTypes[1].Version())
			Expect(resourceTypeVersions).To(ConsistOf(atc.Version{"version": "2"}, atc.Version{"version": "5"}))
		})
	})

	Describe("ResourceVersion", func() {
		var (
			resourceVersion, rv   atc.ResourceVersion
			resourceConfigVersion db.ResourceConfigVersion
			resource              db.Resource
		)

		BeforeEach(func() {
			var found bool
			var err error
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfig, err := resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			version := atc.Version{"version": "1"}
			err = resourceConfig.SaveVersions([]atc.Version{
				version,
			})
			Expect(err).ToNot(HaveOccurred())

			resourceConfigVersion, found, err = resourceConfig.FindVersion(version)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceVersion = atc.ResourceVersion{
				Version: version,
				ID:      resourceConfigVersion.ID(),
				Enabled: true,
			}
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
				Expect(rv).To(Equal(resourceVersion))
			})
		})

		Context("when a resource is not enabled", func() {
			BeforeEach(func() {
				err := resource.DisableVersion(resourceConfigVersion.ID())
				Expect(err).ToNot(HaveOccurred())

				resourceVersion.Enabled = false
			})

			It("should return the version with enabled set to false", func() {
				Expect(rv).To(Equal(resourceVersion))
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
			pipeline, _, err = team.SavePipeline("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			for i := range builds {
				builds[i], err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				buildStart := time.Date(2020, 11, i+1, 0, 0, 0, 0, time.UTC)
				_, err = dbConn.Exec("UPDATE builds SET start_time = to_timestamp($1) WHERE id = $2", buildStart.Unix(), builds[i].ID())
				Expect(err).NotTo(HaveOccurred())

				builds[i], found, err = job.Build(builds[i].Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			}

			otherPipeline, _, err := team.SavePipeline("another-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err := otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = otherJob.CreateBuild()
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

			Context("only until", func() {
				It("returns only those after until", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						Until: int(builds[2].StartTime().Unix()),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[0], builds[1], builds[2]))
				})
			})

			Context("only since", func() {
				It("returns only those before since", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						Since: int(builds[1].StartTime().Unix()),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2], builds[3]))
				})
			})

			Context("since and until", func() {
				It("returns only elements in the range", func() {
					returnedBuilds, _, err := pipeline.BuildsWithTime(db.Page{
						Until: int(builds[2].StartTime().Unix()),
						Since: int(builds[1].StartTime().Unix()),
						Limit: 50,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2]))
				})
			})

		})
	})
})
