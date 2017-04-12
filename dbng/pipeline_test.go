package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	Describe("LoadVersionsDB", func() {
		var (
			team                dbng.Team
			pipeline            dbng.Pipeline
			resource            dbng.Resource
			otherResource       dbng.Resource
			reallyOtherResource dbng.Resource
		)

		resourceName := "some-resource"
		otherResourceName := "some-other-resource"
		reallyOtherResourceName := "some-really-other-resource"

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
							"source-config": "some-value",
						},
					},
					{
						Name: "some-really-other-resource",
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
									Image: "some-image",
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
			team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline, _, err = team.SavePipeline("fake-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = pipeline.Resource(resourceName)
			Expect(err).NotTo(HaveOccurred())

			otherResource, _, err = pipeline.Resource(otherResourceName)
			Expect(err).NotTo(HaveOccurred())

			reallyOtherResource, _, err = pipeline.Resource(reallyOtherResourceName)
			Expect(err).NotTo(HaveOccurred())

		})

		It("can load up versioned resource information relevant to scheduling", func() {
			job, found, err := pipeline.Job("some-job")
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			otherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := pipeline.Job("a-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := pipeline.Job("shared-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := pipeline.Job("random-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := pipeline.Job("other-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := pipeline.Job("different-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err := pipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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

			// By("initially having no latest versioned resource")
			// _, found, err = pipeline.GetLatestVersionedResource(resource.Name())
			// Expect(err).NotTo(HaveOccurred())
			// Expect(found).To(BeFalse())

			// By("including saved versioned resources of the current pipeline")
			// err = pipeline.SaveResourceVersions(atc.ResourceConfig{
			// 	Name:   resource.Name(),
			// 	Type:   "some-type",
			// 	Source: atc.Source{"some": "source"},
			// }, []atc.Version{{"version": "1"}})
			// Expect(err).NotTo(HaveOccurred())

			// savedVR1, found, err := pipeline.GetLatestVersionedResource(resource.Name)
			// Expect(err).NotTo(HaveOccurred())
			// Expect(found).To(BeTrue())
			// Expect(savedVR1.ModifiedTime).NotTo(BeNil())
			// Expect(savedVR1.ModifiedTime).To(BeTemporally(">", time.Time{}))

			// err = pipeline.SaveResourceVersions(atc.ResourceConfig{
			// 	Name:   resource.Name,
			// 	Type:   "some-type",
			// 	Source: atc.Source{"some": "source"},
			// }, []atc.Version{{"version": "2"}})
			// Expect(err).NotTo(HaveOccurred())

			// savedVR2, found, err := pipeline.GetLatestVersionedResource(resource.Name)
			// Expect(err).NotTo(HaveOccurred())
			// Expect(found).To(BeTrue())

			// versions, err = pipeline.LoadVersionsDB()
			// Expect(err).NotTo(HaveOccurred())
			// Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
			// 	{VersionID: savedVR1.ID, ResourceID: resource.ID, CheckOrder: savedVR1.CheckOrder},
			// 	{VersionID: savedVR2.ID, ResourceID: resource.ID, CheckOrder: savedVR2.CheckOrder},
			// }))

			// Expect(versions.BuildOutputs).To(BeEmpty())
			// Expect(versions.ResourceIDs).To(Equal(map[string]int{
			// 	resource.Name:            resource.ID,
			// 	otherResource.Name:       otherResource.ID,
			// 	reallyOtherResource.Name: reallyOtherResource.ID,
			// }))

			// Expect(versions.JobIDs).To(Equal(map[string]int{
			// 	"some-job":                   job.ID,
			// 	"some-other-job":             otherJob.ID,
			// 	"a-job":                      aJob.ID,
			// 	"shared-job":                 sharedJob.ID,
			// 	"random-job":                 randomJob.ID,
			// 	"other-serial-group-job":     otherSerialGroupJob.ID,
			// 	"different-serial-group-job": differentSerialGroupJob.ID,
			// }))

			// By("not including saved versioned resources of other pipelines")
			// otherPipelineResource, _, err := otherPipelineDB.GetResource("some-other-resource")
			// Expect(err).NotTo(HaveOccurred())

			// err = otherPipelineDB.SaveResourceVersions(atc.ResourceConfig{
			// 	Name:   otherPipelineResource.Name,
			// 	Type:   "some-type",
			// 	Source: atc.Source{"some": "source"},
			// }, []atc.Version{{"version": "1"}})
			// Expect(err).NotTo(HaveOccurred())

			// otherPipelineSavedVR, found, err := otherPipelineDB.GetLatestVersionedResource(otherPipelineResource.Name)
			// Expect(err).NotTo(HaveOccurred())
			// Expect(found).To(BeTrue())

			// versions, err = pipeline.LoadVersionsDB()
			// Expect(err).NotTo(HaveOccurred())
			// Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
			// 	{VersionID: savedVR1.ID, ResourceID: resource.ID, CheckOrder: savedVR1.CheckOrder},
			// 	{VersionID: savedVR2.ID, ResourceID: resource.ID, CheckOrder: savedVR2.CheckOrder},
			// }))

			// Expect(versions.BuildOutputs).To(BeEmpty())
			// Expect(versions.ResourceIDs).To(Equal(map[string]int{
			// 	resource.Name:            resource.ID,
			// 	otherResource.Name:       otherResource.ID,
			// 	reallyOtherResource.Name: reallyOtherResource.ID,
			// }))

			// Expect(versions.JobIDs).To(Equal(map[string]int{
			// 	"some-job":                   job.ID,
			// 	"some-other-job":             otherJob.ID,
			// 	"a-job":                      aJob.ID,
			// 	"shared-job":                 sharedJob.ID,
			// 	"random-job":                 randomJob.ID,
			// 	"other-serial-group-job":     otherSerialGroupJob.ID,
			// 	"different-serial-group-job": differentSerialGroupJob.ID,
			// }))

			// By("including outputs of successful builds")
			// build1DB, err := pipeline.CreateJobBuild("a-job")
			// Expect(err).NotTo(HaveOccurred())

			// savedVR1, err = pipeline.SaveOutput(build1DB.ID(), savedVR1.VersionedResource, false)
			// Expect(err).NotTo(HaveOccurred())

			// err = build1DB.Finish(db.StatusSucceeded)
			// Expect(err).NotTo(HaveOccurred())

			// versions, err = pipeline.LoadVersionsDB()
			// Expect(err).NotTo(HaveOccurred())
			// Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
			// 	{VersionID: savedVR1.ID, ResourceID: resource.ID, CheckOrder: savedVR1.CheckOrder},
			// 	{VersionID: savedVR2.ID, ResourceID: resource.ID, CheckOrder: savedVR2.CheckOrder},
			// }))

			// Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
			// 	{
			// 		ResourceVersion: algorithm.ResourceVersion{
			// 			VersionID:  savedVR1.ID,
			// 			ResourceID: resource.ID,
			// 			CheckOrder: savedVR1.CheckOrder,
			// 		},
			// 		JobID:   aJob.ID,
			// 		BuildID: build1DB.ID(),
			// 	},
			// }))

			// Expect(versions.ResourceIDs).To(Equal(map[string]int{
			// 	resource.Name:            resource.ID,
			// 	otherResource.Name:       otherResource.ID,
			// 	reallyOtherResource.Name: reallyOtherResource.ID,
			// }))

			// Expect(versions.JobIDs).To(Equal(map[string]int{
			// 	"some-job":                   job.ID,
			// 	"a-job":                      aJob.ID,
			// 	"some-other-job":             otherJob.ID,
			// 	"shared-job":                 sharedJob.ID,
			// 	"random-job":                 randomJob.ID,
			// 	"other-serial-group-job":     otherSerialGroupJob.ID,
			// 	"different-serial-group-job": differentSerialGroupJob.ID,
			// }))

			// By("not including outputs of failed builds")
			// build2DB, err := pipeline.CreateJobBuild("a-job")
			// Expect(err).NotTo(HaveOccurred())

			// savedVR1, err = pipeline.SaveOutput(build2DB.ID(), savedVR1.VersionedResource, false)
			// Expect(err).NotTo(HaveOccurred())

			// err = build2DB.Finish(db.StatusFailed)
			// Expect(err).NotTo(HaveOccurred())

			// versions, err = pipeline.LoadVersionsDB()
			// Expect(err).NotTo(HaveOccurred())
			// Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
			// 	{VersionID: savedVR1.ID, ResourceID: resource.ID, CheckOrder: savedVR1.CheckOrder},
			// 	{VersionID: savedVR2.ID, ResourceID: resource.ID, CheckOrder: savedVR2.CheckOrder},
			// }))

			// Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
			// 	{
			// 		ResourceVersion: algorithm.ResourceVersion{
			// 			VersionID:  savedVR1.ID,
			// 			ResourceID: resource.ID,
			// 			CheckOrder: savedVR1.CheckOrder,
			// 		},
			// 		JobID:   aJob.ID,
			// 		BuildID: build1DB.ID(),
			// 	},
			// }))

			// Expect(versions.ResourceIDs).To(Equal(map[string]int{
			// 	resource.Name:            resource.ID,
			// 	otherResource.Name:       otherResource.ID,
			// 	reallyOtherResource.Name: reallyOtherResource.ID,
			// }))

			// Expect(versions.JobIDs).To(Equal(map[string]int{
			// 	"some-job":                   job.ID,
			// 	"a-job":                      aJob.ID,
			// 	"some-other-job":             otherJob.ID,
			// 	"shared-job":                 sharedJob.ID,
			// 	"random-job":                 randomJob.ID,
			// 	"other-serial-group-job":     otherSerialGroupJob.ID,
			// 	"different-serial-group-job": differentSerialGroupJob.ID,
			// }))

			// By("not including outputs of builds in other pipelines")
			// otherPipelineBuild, err := otherPipelineDB.CreateJobBuild("a-job")
			// Expect(err).NotTo(HaveOccurred())

			// _, err = otherPipelineDB.SaveOutput(otherPipelineBuild.ID(), otherPipelineSavedVR.VersionedResource, false)
			// Expect(err).NotTo(HaveOccurred())

			// err = otherPipelineBuild.Finish(db.StatusSucceeded)
			// Expect(err).NotTo(HaveOccurred())

			// versions, err = pipeline.LoadVersionsDB()
			// Expect(err).NotTo(HaveOccurred())
			// Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
			// 	{VersionID: savedVR1.ID, ResourceID: resource.ID, CheckOrder: savedVR1.CheckOrder},
			// 	{VersionID: savedVR2.ID, ResourceID: resource.ID, CheckOrder: savedVR2.CheckOrder},
			// }))

			// Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
			// 	{
			// 		ResourceVersion: algorithm.ResourceVersion{
			// 			VersionID:  savedVR1.ID,
			// 			ResourceID: resource.ID,
			// 			CheckOrder: savedVR1.CheckOrder,
			// 		},
			// 		JobID:   aJob.ID,
			// 		BuildID: build1DB.ID(),
			// 	},
			// }))

			// Expect(versions.ResourceIDs).To(Equal(map[string]int{
			// 	resource.Name:            resource.ID,
			// 	otherResource.Name:       otherResource.ID,
			// 	reallyOtherResource.Name: reallyOtherResource.ID,
			// }))

			// Expect(versions.JobIDs).To(Equal(map[string]int{
			// 	"some-job":                   job.ID,
			// 	"a-job":                      aJob.ID,
			// 	"some-other-job":             otherJob.ID,
			// 	"shared-job":                 sharedJob.ID,
			// 	"random-job":                 randomJob.ID,
			// 	"other-serial-group-job":     otherSerialGroupJob.ID,
			// 	"different-serial-group-job": differentSerialGroupJob.ID,
			// }))

			// By("including build inputs")
			// build1DB, err = pipeline.CreateJobBuild("a-job")
			// Expect(err).NotTo(HaveOccurred())

			// savedVR1, err = pipeline.SaveInput(build1DB.ID(), db.BuildInput{
			// 	Name:              "some-input-name",
			// 	VersionedResource: savedVR1.VersionedResource,
			// })
			// Expect(err).NotTo(HaveOccurred())

			// err = build1DB.Finish(db.StatusSucceeded)
			// Expect(err).NotTo(HaveOccurred())

			// versions, err = pipeline.LoadVersionsDB()
			// Expect(err).NotTo(HaveOccurred())

			// Expect(versions.BuildInputs).To(ConsistOf([]algorithm.BuildInput{
			// 	{
			// 		ResourceVersion: algorithm.ResourceVersion{
			// 			VersionID:  savedVR1.ID,
			// 			ResourceID: resource.ID,
			// 			CheckOrder: savedVR1.CheckOrder,
			// 		},
			// 		JobID:     aJob.ID,
			// 		BuildID:   build1DB.ID(),
			// 		InputName: "some-input-name",
			// 	},
			// }))
		})
	})

})
