package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline dbng.Pipeline
		team     dbng.Team
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		pipeline, _, err = team.SavePipeline("fake-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "job-name"},
			},
		}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
		Expect(err).ToNot(HaveOccurred())
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

	Describe("GetLatestVersionedResource", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       atc.ResourceConfig
			latestVR             dbng.SavedVersionedResource
			found                bool
		)

		Context("when the resource exists", func() {
			BeforeEach(func() {
				var err error
				pipelineConfig := atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
							Source: atc.Source{
								"source-config": "some-value",
							},
						},
					},
				}
				pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				resource, _, err := pipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())

				resourceConfig = atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}

				originalVersionSlice = []atc.Version{
					{"ref": "v1"},
					{"ref": "v3"},
				}

				err = pipeline.SaveResourceVersions(resourceConfig, originalVersionSlice)
				Expect(err).NotTo(HaveOccurred())

				latestVR, found, err = pipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeTrue())

				Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"ref": "v3"}))
				Expect(latestVR.CheckOrder).To(Equal(2))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				var err error
				latestVR, found, err = pipeline.GetLatestVersionedResource("dummy")
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeFalse())
				Expect(latestVR).To(Equal(dbng.SavedVersionedResource{}))
			})
		})
	})

	Describe("SaveResourceVersions", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       atc.ResourceConfig
			pipeline             dbng.Pipeline
			resource             dbng.Resource
		)

		BeforeEach(func() {
			var err error
			pipelineConfig := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},
			}
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())

			resourceConfig = atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}

			originalVersionSlice = []atc.Version{
				{"ref": "v1"},
				{"ref": "v3"},
			}
		})

		It("ensures versioned resources have the correct check_order", func() {
			err := pipeline.SaveResourceVersions(resourceConfig, originalVersionSlice)
			Expect(err).NotTo(HaveOccurred())

			latestVR, found, err := pipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"ref": "v3"}))
			Expect(latestVR.CheckOrder).To(Equal(2))

			pretendCheckResults := []atc.Version{
				{"ref": "v2"},
				{"ref": "v3"},
			}

			err = pipeline.SaveResourceVersions(resourceConfig, pretendCheckResults)
			Expect(err).NotTo(HaveOccurred())

			latestVR, found, err = pipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"ref": "v3"}))
			Expect(latestVR.CheckOrder).To(Equal(4))
		})

		Context("resource not found", func() {
			BeforeEach(func() {
				resourceConfig = atc.ResourceConfig{
					Name:   "unknown-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}

				originalVersionSlice = []atc.Version{{"ref": "v1"}}
			})

			It("returns an error", func() {
				err := pipeline.SaveResourceVersions(resourceConfig, originalVersionSlice)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetVersionedResourceByVersion", func() {
		var savedVersion2 dbng.SavedVersionedResource
		BeforeEach(func() {
			var err error
			pipelineConfig := atc.Config{
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
				},
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
			}
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			err = pipeline.SaveResourceVersions(
				atc.ResourceConfig{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
				[]atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"}, // disabled
				},
			)
			Expect(err).NotTo(HaveOccurred())

			// save metadata for v2
			build, err := pipeline.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())
			err = build.SaveInput(dbng.BuildInput{
				Name: "some-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  dbng.ResourceVersion{"version": "v2"},
					Metadata: []dbng.ResourceMetadataField{{Name: "name1", Value: "value1"}},
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

			savedVersions, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedVersions).To(HaveLen(2))
			pipeline.DisableVersionedResource(savedVersions[0].ID)
			savedVersion2 = savedVersions[1]

			err = pipeline.SaveResourceVersions(
				atc.ResourceConfig{
					Name: "some-other-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
				[]atc.Version{
					{"version": "v2"},
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the SavedVersionedResource matching the given resource name and atc version", func() {
			By("returning versions that exist")
			actualSavedVersion, found, err := pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v2"},
				"some-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualSavedVersion).To(Equal(savedVersion2))

			By("returning not found for versions that don't exist")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"versioni": "v2"},
				"some-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning not found for versions that only exist in another resource")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v1"},
				"some-other-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning not found for disabled versions")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v3"},
				"some-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("NextBuildInputs", func() {
		var pipeline dbng.Pipeline
		var pipeline2 dbng.Pipeline
		var versions dbng.SavedVersionedResources

		BeforeEach(func() {
			resourceConfig := atc.ResourceConfig{
				Name: "some-resource",
				Type: "some-type",
			}

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
				Resources: atc.ResourceConfigs{resourceConfig},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", config, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			err = pipeline.SaveResourceVersions(
				resourceConfig,
				[]atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"},
				},
			)
			Expect(err).NotTo(HaveOccurred())

			// save metadata for v1
			build, err := pipeline.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())
			err = build.SaveInput(dbng.BuildInput{
				Name: "some-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  dbng.ResourceVersion{"version": "v1"},
					Metadata: []dbng.ResourceMetadataField{{Name: "name1", Value: "value1"}},
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

			reversions, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 3})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions = []dbng.SavedVersionedResource{reversions[2], reversions[1], reversions[0]}

			pipeline2, _, err = team.SavePipeline("some-pipeline-2", config, 1, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("independent build inputs", func() {
			It("gets independent build inputs for the given job name", func() {
				inputVersions := algorithm.InputMapping{
					"some-input-1": algorithm.InputVersion{
						VersionID:       versions[0].ID,
						FirstOccurrence: false,
					},
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[1].ID,
						FirstOccurrence: true,
					},
				}
				err := pipeline.SaveIndependentInputMapping(inputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				pipeline2InputVersions := algorithm.InputMapping{
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
				}
				err = pipeline2.SaveIndependentInputMapping(pipeline2InputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []dbng.BuildInput{
					{
						Name:              "some-input-1",
						VersionedResource: versions[0].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-2",
						VersionedResource: versions[1].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs, err := pipeline.GetIndependentBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs).To(ConsistOf(buildInputs))

				By("updating the set of independent build inputs")
				inputVersions2 := algorithm.InputMapping{
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: true,
					},
				}
				err = pipeline.SaveIndependentInputMapping(inputVersions2, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs2 := []dbng.BuildInput{
					{
						Name:              "some-input-2",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-3",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs2, err := pipeline.GetIndependentBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

				By("updating independent build inputs to an empty set when the mapping is nil")
				err = pipeline.SaveIndependentInputMapping(nil, "some-job")
				Expect(err).NotTo(HaveOccurred())

				actualBuildInputs3, err := pipeline.GetIndependentBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualBuildInputs3).To(BeEmpty())
			})
		})

		Describe("next build inputs", func() {
			It("gets next build inputs for the given job name", func() {
				inputVersions := algorithm.InputMapping{
					"some-input-1": algorithm.InputVersion{
						VersionID:       versions[0].ID,
						FirstOccurrence: false,
					},
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[1].ID,
						FirstOccurrence: true,
					},
				}
				err := pipeline.SaveNextInputMapping(inputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				pipeline2InputVersions := algorithm.InputMapping{
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
				}
				err = pipeline2.SaveNextInputMapping(pipeline2InputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []dbng.BuildInput{
					{
						Name:              "some-input-1",
						VersionedResource: versions[0].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-2",
						VersionedResource: versions[1].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs, found, err := pipeline.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(actualBuildInputs).To(ConsistOf(buildInputs))

				By("updating the set of next build inputs")
				inputVersions2 := algorithm.InputMapping{
					"some-input-2": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: true,
					},
				}
				err = pipeline.SaveNextInputMapping(inputVersions2, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs2 := []dbng.BuildInput{
					{
						Name:              "some-input-2",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   false,
					},
					{
						Name:              "some-input-3",
						VersionedResource: versions[2].VersionedResource,
						FirstOccurrence:   true,
					},
				}

				actualBuildInputs2, found, err := pipeline.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

				By("updating next build inputs to an empty set when the mapping is nil")
				err = pipeline.SaveNextInputMapping(nil, "some-job")
				Expect(err).NotTo(HaveOccurred())

				actualBuildInputs3, found, err := pipeline.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(actualBuildInputs3).To(BeEmpty())
			})

			It("distinguishes between a job with no inputs and a job with missing inputs", func() {
				By("initially returning not found")
				_, found, err := pipeline.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())

				By("returning found when an empty input mapping is saved")
				err = pipeline.SaveNextInputMapping(algorithm.InputMapping{}, "some-job")
				Expect(err).NotTo(HaveOccurred())

				_, found, err = pipeline.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				By("returning not found when the input mapping is deleted")
				err = pipeline.DeleteNextInputMapping("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, found, err = pipeline.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("PauseJob and UnpauseJob", func() {
		jobName := "job-name"

		It("starts out as unpaused", func() {
			job, found, err := pipeline.Job(jobName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.Paused()).To(BeFalse())
		})

		It("can be paused", func() {
			err := pipeline.PauseJob(jobName)
			Expect(err).NotTo(HaveOccurred())

			pausedJob, found, err := pipeline.Job(jobName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(pausedJob.Paused()).To(BeTrue())
		})

		It("can be unpaused", func() {
			err := pipeline.PauseJob(jobName)
			Expect(err).NotTo(HaveOccurred())

			err = pipeline.UnpauseJob(jobName)
			Expect(err).NotTo(HaveOccurred())

			unpausedJob, found, err := pipeline.Job(jobName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(unpausedJob.Paused()).To(BeFalse())
		})
	})

	Describe("saving build inputs", func() {
		var (
			buildMetadata []dbng.ResourceMetadataField
			vr1           dbng.VersionedResource
		)

		BeforeEach(func() {
			buildMetadata = []dbng.ResourceMetadataField{
				{
					Name:  "meta1",
					Value: "value1",
				},
				{
					Name:  "meta2",
					Value: "value2",
				},
			}

			vr1 = dbng.VersionedResource{
				Resource: "some-other-resource",
				Type:     "some-type",
				Version:  dbng.ResourceVersion{"ver": "2"},
			}

			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}
			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails to save build input if resource does not exist", func() {
			build, err := pipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			vr := dbng.VersionedResource{
				Resource: "unknown-resource",
				Type:     "some-type",
				Version:  dbng.ResourceVersion{"ver": "2"},
			}

			input := dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: vr,
			}

			err = build.SaveInput(input)
			Expect(err).To(HaveOccurred())
		})

		It("updates metadata of existing versioned resources", func() {
			build, err := pipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: vr1,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
			}))

			withMetadata := vr1
			withMetadata.Metadata = buildMetadata

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-other-input",
				VersionedResource: withMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err = build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
				{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: withMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err = build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
				{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))

		})

		It("does not clobber metadata of existing versioned resources", func() {
			build, err := pipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			withMetadata := vr1
			withMetadata.Metadata = buildMetadata

			withoutMetadata := vr1
			withoutMetadata.Metadata = nil

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: withMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))

			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-other-input",
				VersionedResource: withoutMetadata,
			})
			Expect(err).NotTo(HaveOccurred())

			inputs, _, err = build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
				{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
			}))
		})
	})

	Describe("a build is created for a job", func() {
		var (
			build1DB      dbng.Build
			pipeline      dbng.Pipeline
			otherPipeline dbng.Pipeline
		)

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}
			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherPipeline, _, err = team.SavePipeline("some-other-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			build1DB, err = pipeline.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())

			Expect(build1DB.ID()).NotTo(BeZero())
			Expect(build1DB.JobName()).To(Equal("some-job"))
			Expect(build1DB.Name()).To(Equal("1"))
			Expect(build1DB.Status()).To(Equal(dbng.BuildStatusPending))
			Expect(build1DB.IsScheduled()).To(BeFalse())
		})

		It("becomes the next pending build for job", func() {
			nextPendings, err := pipeline.GetPendingBuildsForJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			//time.Sleep(10 * time.Hour)
			Expect(nextPendings).NotTo(BeEmpty())
			Expect(nextPendings[0].ID()).To(Equal(build1DB.ID()))
		})

		It("is in the list of pending builds", func() {
			nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextPendingBuilds["some-job"]).To(HaveLen(1))
			Expect(nextPendingBuilds["some-job"]).To(Equal([]dbng.Build{build1DB}))
		})

		Context("and another build for a different pipeline is created with the same job name", func() {
			BeforeEach(func() {
				otherBuild, err := otherPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				Expect(otherBuild.ID()).NotTo(BeZero())
				Expect(otherBuild.JobName()).To(Equal("some-job"))
				Expect(otherBuild.Name()).To(Equal("1"))
				Expect(otherBuild.Status()).To(Equal(dbng.BuildStatusPending))
				Expect(otherBuild.IsScheduled()).To(BeFalse())
			})

			It("does not change the next pending build for job", func() {
				nextPendingBuilds, err := pipeline.GetPendingBuildsForJob("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds).To(Equal([]dbng.Build{build1DB}))
			})

			It("does not change pending builds", func() {
				nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds["some-job"]).To(HaveLen(1))
				Expect(nextPendingBuilds["some-job"]).To(Equal([]dbng.Build{build1DB}))
			})
		})

		Context("when scheduled", func() {
			BeforeEach(func() {
				var err error
				var found bool
				found, err = build1DB.Schedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("remains the next pending build for job", func() {
				nextPendingBuilds, err := pipeline.GetPendingBuildsForJob("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds).NotTo(BeEmpty())
				Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
			})

			It("remains in the list of pending builds", func() {
				nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds["some-job"]).To(HaveLen(1))
				Expect(nextPendingBuilds["some-job"][0].ID()).To(Equal(build1DB.ID()))
			})
		})

		Context("when started", func() {
			BeforeEach(func() {
				started, err := build1DB.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("saves the updated status, and the engine and engine metadata", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.Status()).To(Equal(dbng.BuildStatusStarted))
				Expect(build1DB.Engine()).To(Equal("some-engine"))
				Expect(build1DB.EngineMetadata()).To(Equal("some-metadata"))
			})

			It("saves the build's start time", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.StartTime().Unix()).To(BeNumerically("~", time.Now().Unix(), 3))
			})
		})

		Context("when the build finishes", func() {
			BeforeEach(func() {
				err := build1DB.Finish(dbng.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the build's status and end time", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.Status()).To(Equal(dbng.BuildStatusSucceeded))
				Expect(build1DB.EndTime().Unix()).To(BeNumerically("~", time.Now().Unix(), 3))
			})
		})

		Context("and another is created for the same job", func() {
			var build2DB dbng.Build

			BeforeEach(func() {
				var err error
				build2DB, err = pipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				Expect(build2DB.ID()).NotTo(BeZero())
				Expect(build2DB.ID()).NotTo(Equal(build1DB.ID()))
				Expect(build2DB.Name()).To(Equal("2"))
				Expect(build2DB.Status()).To(Equal(dbng.BuildStatusPending))
			})

			Describe("the first build", func() {
				It("remains the next pending build", func() {
					nextPendingBuilds, err := pipeline.GetPendingBuildsForJob("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPendingBuilds).To(HaveLen(2))
					Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
					Expect(nextPendingBuilds[1].ID()).To(Equal(build2DB.ID()))
				})

				It("remains in the list of pending builds", func() {
					nextPendingBuilds, err := pipeline.GetAllPendingBuilds()
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPendingBuilds["some-job"]).To(HaveLen(2))
					Expect(nextPendingBuilds["some-job"]).To(ConsistOf(build1DB, build2DB))
				})
			})
		})
	})
})
