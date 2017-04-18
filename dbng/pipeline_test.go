package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline dbng.Pipeline
	)

	BeforeEach(func() {
		var err error
		pipeline, _, err = defaultTeam.SavePipeline("fake-pipeline", atc.Config{
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
			pipeline, found, err := defaultTeam.Pipeline("oopsies")
			Expect(pipeline.Name()).To(Equal("oopsies"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("NextBuildInputs", func() {
		var pipelineFactory dbng.PipelineFactory
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

			pipeline, _, err = defaultTeam.SavePipeline("some-pipeline", config, 0, dbng.PipelineUnpaused)
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
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())
			_, err = build.SaveInput(db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource:   "some-resource",
					Type:       "some-type",
					Version:    db.Version{"version": "v1"},
					Metadata:   []db.MetadataField{{Name: "name1", Value: "value1"}},
					PipelineID: pipelineDB.GetPipelineID(),
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

			reversions, _, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 3})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions = []db.SavedVersionedResource{reversions[2], reversions[1], reversions[0]}

			savedPipeline2, _, err := teamDB.SaveConfigToBeDeprecated("some-pipeline-2", config, 1, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB2 = pipelineDBFactory.Build(savedPipeline2)
		})

		AfterEach(func() {
			err := dbConn.Close()
			Expect(err).NotTo(HaveOccurred())

			err = listener.Close()
			Expect(err).NotTo(HaveOccurred())
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
				err := pipelineDB.SaveIndependentInputMapping(inputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				pipeline2InputVersions := algorithm.InputMapping{
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
				}
				err = pipelineDB2.SaveIndependentInputMapping(pipeline2InputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []db.BuildInput{
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

				actualBuildInputs, err := pipelineDB.GetIndependentBuildInputs("some-job")
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
				err = pipelineDB.SaveIndependentInputMapping(inputVersions2, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs2 := []db.BuildInput{
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

				actualBuildInputs2, err := pipelineDB.GetIndependentBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

				By("updating independent build inputs to an empty set when the mapping is nil")
				err = pipelineDB.SaveIndependentInputMapping(nil, "some-job")
				Expect(err).NotTo(HaveOccurred())

				actualBuildInputs3, err := pipelineDB.GetIndependentBuildInputs("some-job")
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
				err := pipelineDB.SaveNextInputMapping(inputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				pipeline2InputVersions := algorithm.InputMapping{
					"some-input-3": algorithm.InputVersion{
						VersionID:       versions[2].ID,
						FirstOccurrence: false,
					},
				}
				err = pipelineDB2.SaveNextInputMapping(pipeline2InputVersions, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []db.BuildInput{
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

				actualBuildInputs, found, err := pipelineDB.GetNextBuildInputs("some-job")
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
				err = pipelineDB.SaveNextInputMapping(inputVersions2, "some-job")
				Expect(err).NotTo(HaveOccurred())

				buildInputs2 := []db.BuildInput{
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

				actualBuildInputs2, found, err := pipelineDB.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

				By("updating next build inputs to an empty set when the mapping is nil")
				err = pipelineDB.SaveNextInputMapping(nil, "some-job")
				Expect(err).NotTo(HaveOccurred())

				actualBuildInputs3, found, err := pipelineDB.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(actualBuildInputs3).To(BeEmpty())
			})

			It("distinguishes between a job with no inputs and a job with missing inputs", func() {
				By("initially returning not found")
				_, found, err := pipelineDB.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())

				By("returning found when an empty input mapping is saved")
				err = pipelineDB.SaveNextInputMapping(algorithm.InputMapping{}, "some-job")
				Expect(err).NotTo(HaveOccurred())

				_, found, err = pipelineDB.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				By("returning not found when the input mapping is deleted")
				err = pipelineDB.DeleteNextInputMapping("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, found, err = pipelineDB.GetNextBuildInputs("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
