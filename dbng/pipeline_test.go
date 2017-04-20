package dbng_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/event"
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

		var created bool
		pipeline, created, err = team.SavePipeline("fake-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "job-name"},
			},
			Resources: atc.ResourceConfigs{
				{Name: "resource-name"},
			},
		}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())
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

	Describe("Resource Versions", func() {
		resourceName := "some-resource"
		otherResourceName := "some-other-resource"
		reallyOtherResourceName := "some-really-other-resource"

		var (
			dbngPipeline        dbng.Pipeline
			otherDBNGPipeline   dbng.Pipeline
			resource            dbng.Resource
			otherResource       dbng.Resource
			reallyOtherResource dbng.Resource
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
									RootFsUri: "some-image",
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
			dbngPipeline, _, err = team.SavePipeline("pipeline-name", pipelineConfig, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherDBNGPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = dbngPipeline.Resource(resourceName)
			Expect(err).NotTo(HaveOccurred())

			otherResource, _, err = dbngPipeline.Resource(otherResourceName)
			Expect(err).NotTo(HaveOccurred())

			reallyOtherResource, _, err = dbngPipeline.Resource(reallyOtherResourceName)
			Expect(err).NotTo(HaveOccurred())

		})

		It("returns correct resource", func() {
			Expect(resource.Name()).To(Equal("some-resource"))
			Expect(resource.Paused()).To(Equal(false))
			Expect(resource.PipelineName()).To(Equal("pipeline-name"))
			Expect(resource.CheckError()).To(BeNil())
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{"source-config": "some-value"}))
		})

		It("can load up versioned resource information relevant to scheduling", func() {
			job, found, err := dbngPipeline.Job("some-job")
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			otherJob, found, err := dbngPipeline.Job("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := dbngPipeline.Job("a-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := dbngPipeline.Job("shared-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := dbngPipeline.Job("random-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := dbngPipeline.Job("other-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := dbngPipeline.Job("different-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err := dbngPipeline.LoadVersionsDB()
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

			By("initially having no latest versioned resource")
			_, found, err = dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR1, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedVR1.ModifiedTime).NotTo(BeNil())
			Expect(savedVR1.ModifiedTime).To(BeTemporally(">", time.Time{}))

			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR2, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
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
			otherPipelineResource, _, err := dbngPipeline.Resource("some-other-resource")
			Expect(err).NotTo(HaveOccurred())

			err = otherDBNGPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   otherPipelineResource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).NotTo(HaveOccurred())

			otherPipelineSavedVR, found, err := otherDBNGPipeline.GetLatestVersionedResource(otherPipelineResource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
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
			build1DB, err := dbngPipeline.CreateJobBuild("a-job")
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.SaveOutput(savedVR1.VersionedResource, false)
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
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

			By("not including outputs of failed builds")
			build2DB, err := dbngPipeline.CreateJobBuild("a-job")
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.SaveOutput(savedVR1.VersionedResource, false)
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.Finish(dbng.BuildStatusFailed)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
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
			otherPipelineBuild, err := otherDBNGPipeline.CreateJobBuild("a-job")
			Expect(err).NotTo(HaveOccurred())

			err = otherPipelineBuild.SaveOutput(otherPipelineSavedVR.VersionedResource, false)
			Expect(err).NotTo(HaveOccurred())

			err = otherPipelineBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
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
			build1DB, err = dbngPipeline.CreateJobBuild("a-job")
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.SaveInput(dbng.BuildInput{
				Name:              "some-input-name",
				VersionedResource: savedVR1.VersionedResource,
			})
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())

			Expect(versions.BuildInputs).To(ConsistOf([]algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
					},
					JobID:     aJob.ID(),
					BuildID:   build1DB.ID(),
					InputName: "some-input-name",
				},
			}))
		})

		It("can load up the latest versioned resource, enabled or not", func() {
			By("initially having no latest versioned resource")
			_, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR1, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR2, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(savedVR1.Version).To(Equal(dbng.ResourceVersion{"version": "1"}))
			Expect(savedVR2.Version).To(Equal(dbng.ResourceVersion{"version": "2"}))

			By("not including saved versioned resources of other pipelines")
			_, _, err = otherDBNGPipeline.Resource("some-other-resource")
			Expect(err).NotTo(HaveOccurred())

			err = otherDBNGPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}, {"version": "2"}, {"version": "3"}})
			Expect(err).NotTo(HaveOccurred())

			otherPipelineSavedVR, found, err := otherDBNGPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(otherPipelineSavedVR.Version).To(Equal(dbng.ResourceVersion{"version": "3"}))

			By("including disabled versions")
			err = dbngPipeline.DisableVersionedResource(savedVR2.ID)
			Expect(err).NotTo(HaveOccurred())

			latestVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"version": "2"}))
		})

		Describe("enabling and disabling versioned resources", func() {
			It("returns an error if the resource or version is bogus", func() {
				err := dbngPipeline.EnableVersionedResource(42)
				Expect(err).To(HaveOccurred())

				err = dbngPipeline.DisableVersionedResource(42)
				Expect(err).To(HaveOccurred())
			})

			It("does not affect explicitly fetching the latest version", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(dbng.ResourceVersion{"version": "1"}))
				initialTime := savedVR.ModifiedTime

				err = dbngPipeline.DisableVersionedResource(savedVR.ID)
				Expect(err).NotTo(HaveOccurred())

				disabledVR := savedVR
				disabledVR.Enabled = false

				latestVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Resource).To(Equal(disabledVR.Resource))
				Expect(latestVR.Type).To(Equal(disabledVR.Type))
				Expect(latestVR.Version).To(Equal(disabledVR.Version))
				Expect(latestVR.Enabled).To(BeFalse())
				Expect(latestVR.ModifiedTime).To(BeTemporally(">", initialTime))

				tmp_modified_time := latestVR.ModifiedTime

				err = dbngPipeline.EnableVersionedResource(savedVR.ID)
				Expect(err).NotTo(HaveOccurred())

				enabledVR := savedVR
				enabledVR.Enabled = true

				latestVR, found, err = dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Resource).To(Equal(enabledVR.Resource))
				Expect(latestVR.Type).To(Equal(enabledVR.Type))
				Expect(latestVR.Version).To(Equal(enabledVR.Version))
				Expect(latestVR.Enabled).To(BeTrue())
				Expect(latestVR.ModifiedTime).To(BeTemporally(">", tmp_modified_time))
			})

			It("doesn't change the check_order when saving a new build input", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).NotTo(HaveOccurred())

				build, err := dbngPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				beforeVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).NotTo(HaveOccurred())

				input := dbng.BuildInput{
					Name:              "input-name",
					VersionedResource: beforeVR.VersionedResource,
				}

				err = build.SaveInput(input)
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new implicit build output", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).NotTo(HaveOccurred())

				build, err := dbngPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				beforeVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).NotTo(HaveOccurred())

				err = build.SaveOutput(beforeVR.VersionedResource, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new implicit build output", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).NotTo(HaveOccurred())

				build, err := dbngPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				beforeVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).NotTo(HaveOccurred())

				err = build.SaveOutput(beforeVR.VersionedResource, true)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("saving versioned resources", func() {
			It("updates the latest versioned resource", func() {
				err := dbngPipeline.SaveResourceVersions(
					atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					[]atc.Version{{"version": "1"}},
				)
				Expect(err).NotTo(HaveOccurred())

				savedResource, _, err := dbngPipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err := dbngPipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(dbng.ResourceVersion{"version": "1"}))

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}, {"version": "3"}})
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err = dbngPipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(dbng.ResourceVersion{"version": "3"}))
			})
		})

		It("initially has no pending build for a job", func() {
			pendingBuilds, err := dbngPipeline.GetPendingBuildsForJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(pendingBuilds).To(HaveLen(0))
		})

		Describe("marking resource checks as errored", func() {
			BeforeEach(func() {
				var err error
				resource, _, err = dbngPipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the resource is first created", func() {
				It("is not errored", func() {
					Expect(resource.CheckError()).To(BeNil())
				})
			})

			Context("when a resource check is marked as errored", func() {
				It("is then marked as errored", func() {
					originalCause := errors.New("on fire")

					err := dbngPipeline.SetResourceCheckError(resource, originalCause)
					Expect(err).NotTo(HaveOccurred())

					returnedResource, _, err := dbngPipeline.Resource("some-resource")
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedResource.CheckError()).To(Equal(originalCause))
				})
			})

			Context("when a resource is cleared of check errors", func() {
				It("is not marked as errored again", func() {
					originalCause := errors.New("on fire")

					err := dbngPipeline.SetResourceCheckError(resource, originalCause)
					Expect(err).NotTo(HaveOccurred())

					err = dbngPipeline.SetResourceCheckError(resource, nil)
					Expect(err).NotTo(HaveOccurred())

					returnedResource, _, err := dbngPipeline.Resource("some-resource")
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedResource.CheckError()).To(Equal(errors.New("NULL")))
				})
			})
		})
	})

	Describe("Disable and Enable Resource Versions", func() {
		var pipelineDB dbng.Pipeline
		var resource dbng.Resource

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
						Source: atc.Source{"some": "source"},
					},
				},
			}
			var err error
			pipelineDB, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			resource, found, err = pipelineDB.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})
		Context("when a version is disabled", func() {
			It("omits the version from the versions DB", func() {
				build1, err := pipelineDB.CreateJobBuild("a-job")
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "disabled"}})
				Expect(err).NotTo(HaveOccurred())

				disabledVersion, found, err := pipelineDB.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1.SaveInput(dbng.BuildInput{
					Name:              "disabled-input",
					VersionedResource: disabledVersion.VersionedResource,
				})
				Expect(err).NotTo(HaveOccurred())

				err = build1.SaveOutput(disabledVersion.VersionedResource, false)
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "enabled"}})
				Expect(err).NotTo(HaveOccurred())

				enabledVersion, found, err := pipelineDB.GetLatestVersionedResource("some-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1.SaveInput(dbng.BuildInput{
					Name:              "enabled-input",
					VersionedResource: enabledVersion.VersionedResource,
				})
				Expect(err).NotTo(HaveOccurred())

				err = build1.SaveOutput(enabledVersion.VersionedResource, false)
				Expect(err).NotTo(HaveOccurred())

				err = build1.Finish(dbng.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				pipelineDB.DisableVersionedResource(disabledVersion.ID)

				pipelineDB.DisableVersionedResource(enabledVersion.ID)
				pipelineDB.EnableVersionedResource(enabledVersion.ID)

				versions, err := pipelineDB.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				aJob, found, err := pipelineDB.Job("a-job")
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

				By("omitting it from the list of resource versions")
				Expect(versions.ResourceVersions).To(ConsistOf(
					algorithm.ResourceVersion{
						VersionID:  enabledVersion.ID,
						ResourceID: resource.ID(),
						CheckOrder: enabledVersion.CheckOrder,
					},
				))

				By("omitting it from build outputs")
				Expect(versions.BuildOutputs).To(ConsistOf(
					algorithm.BuildOutput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  enabledVersion.ID,
							ResourceID: resource.ID(),
							CheckOrder: enabledVersion.CheckOrder,
						},
						JobID:   aJob.ID(),
						BuildID: build1.ID(),
					},
				))

				By("omitting it from build inputs")
				Expect(versions.BuildInputs).To(ConsistOf(
					algorithm.BuildInput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  enabledVersion.ID,
							ResourceID: resource.ID(),
							CheckOrder: enabledVersion.CheckOrder,
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
		It("removes the pipeline and all of its data", func() {
			By("populating resources table")
			resource, found, err := pipeline.Resource("resource-name")
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			By("populating resource versions")
			err = pipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   resource.Type(),
				Source: resource.Source(),
			}, []atc.Version{
				{
					"key": "value",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("populating builds")
			build, err := pipeline.CreateJobBuild("job-name")
			Expect(err).NotTo(HaveOccurred())

			By("populating build inputs")
			err = build.SaveInput(dbng.BuildInput{
				Name: "build-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "resource-name",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("populating build outputs")
			err = build.SaveOutput(dbng.VersionedResource{
				Resource: "resource-name",
			}, false)
			Expect(err).NotTo(HaveOccurred())

			By("populating build events")
			err = build.SaveEvent(event.StartTask{})
			Expect(err).NotTo(HaveOccurred())

			// populate image_resource_versions table
			err = build.SaveImageResourceVersion("some-plan-id", atc.Version{"digest": "readers"}, `docker{"some":"source"}`)
			Expect(err).NotTo(HaveOccurred())

			err = pipeline.Destroy()
			Expect(err).NotTo(HaveOccurred())

			found, err = pipeline.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			found, err = build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = team.Pipeline(pipeline.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("EnsurePendingBuildExists", func() {
		Context("when only a started build exists", func() {
			BeforeEach(func() {
				build1, err := pipeline.CreateJobBuild("job-name")
				Expect(err).NotTo(HaveOccurred())

				started, err := build1.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("creates a build", func() {
				err := pipeline.EnsurePendingBuildExists("job-name")
				Expect(err).NotTo(HaveOccurred())

				pendingBuildsForJob, err := pipeline.GetPendingBuildsForJob("job-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuildsForJob).To(HaveLen(1))
			})

			It("doesn't create another build the second time it's called", func() {
				err := pipeline.EnsurePendingBuildExists("job-name")
				Expect(err).NotTo(HaveOccurred())

				err = pipeline.EnsurePendingBuildExists("job-name")
				Expect(err).NotTo(HaveOccurred())

				builds2, err := pipeline.GetPendingBuildsForJob("job-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(1))

				started, err := builds2[0].Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				builds2, err = pipeline.GetPendingBuildsForJob("job-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(0))
			})
		})
	})

	Describe("GetPendingBuildsForJob/GetAllPendingBuilds", func() {
		Context("when a build is created", func() {
			BeforeEach(func() {
				_, err := pipeline.CreateJobBuild("job-name")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the build", func() {
				pendingBuildsForJob, err := pipeline.GetPendingBuildsForJob("job-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuildsForJob).To(HaveLen(1))

				pendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))
				Expect(pendingBuilds["job-name"]).NotTo(BeNil())
			})
		})
	})
})
