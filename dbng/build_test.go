package dbng_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build", func() {
	var (
		team dbng.Team
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Reload", func() {
		It("updates the model", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			started, err := build.Start("engine", "metadata")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			Expect(build.Status()).To(Equal(dbng.BuildStatusPending))

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusStarted))
		})
	})

	Describe("Start", func() {
		var build dbng.Build
		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := build.Start("engine", "metadata")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())
		})

		It("creates Start event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusStarted))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   build.StartTime().Unix(),
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusStarted))
		})
	})

	Describe("Finish", func() {
		var build dbng.Build
		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Finish event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusSucceeded))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusSucceeded,
				Time:   build.EndTime().Unix(),
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusSucceeded))
		})
	})

	Describe("Abort", func() {
		var build dbng.Build
		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.Abort()
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusAborted))
		})
	})

	Describe("Events", func() {
		It("saves and emits status events", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			By("emitting a status event when started")
			started, err := build.Start("engine", "metadata")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   build.StartTime().Unix(),
			})))

			By("emitting a status event when finished")
			err = build.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			found, err = build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusSucceeded,
				Time:   build.EndTime().Unix(),
			})))

			By("ending the stream when finished")
			_, err = events.Next()
			Expect(err).To(Equal(dbng.ErrEndOfBuildEventStream))
		})
	})

	Describe("SaveEvent", func() {
		It("saves and propagates events correctly", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			By("saving them in order")
			err = build.SaveEvent(event.Log{
				Payload: "some ",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "some ",
			})))

			err = build.SaveEvent(event.Log{
				Payload: "log",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			})))

			By("allowing you to subscribe from an offset")
			eventsFrom1, err := build.Events(1)
			Expect(err).NotTo(HaveOccurred())

			defer eventsFrom1.Close()

			Expect(eventsFrom1.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			})))

			By("notifying those waiting on events as soon as they're saved")
			nextEvent := make(chan event.Envelope)
			nextErr := make(chan error)

			go func() {
				event, err := events.Next()
				if err != nil {
					nextErr <- err
				} else {
					nextEvent <- event
				}
			}()

			Consistently(nextEvent).ShouldNot(Receive())
			Consistently(nextErr).ShouldNot(Receive())

			err = build.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(nextEvent).Should(Receive(Equal(envelope(event.Log{
				Payload: "log 2",
			}))))

			By("returning ErrBuildEventStreamClosed for Next calls after Close")
			events3, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			err = events3.Close()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := events3.Next()
				return err
			}).Should(Equal(dbng.ErrBuildEventStreamClosed))
		})
	})

	Describe("SaveInput", func() {
		var pipeline dbng.Pipeline
		var job dbng.Job

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when a job build", func() {
			It("saves the build's input", func() {
				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				versionedResource := dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"some": "version",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
				}
				err = build.SaveInput(dbng.BuildInput{
					Name:              "some-input",
					VersionedResource: versionedResource,
				})
				Expect(err).ToNot(HaveOccurred())

				actualBuildInput, err := build.GetVersionedResources()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(actualBuildInput)).To(Equal(1))
				Expect(actualBuildInput[0].VersionedResource).To(Equal(versionedResource))
			})
		})

		Context("when a one off build", func() {
			It("does not save the build's input", func() {
				build, err := team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				versionedResource := dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"some": "version",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
				}
				err = build.SaveInput(dbng.BuildInput{
					Name:              "some-input",
					VersionedResource: versionedResource,
				})
				Expect(err).ToNot(HaveOccurred())

				actualBuildInput, err := build.GetVersionedResources()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(actualBuildInput)).To(Equal(0))
			})
		})
	})

	Describe("SaveOutput", func() {
		var pipeline dbng.Pipeline
		var job dbng.Job

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-implicit-resource",
						Type: "some-type",
					},
					{
						Name: "some-explicit-resource",
						Type: "some-type",
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when a job build", func() {
			It("can get a build's output", func() {
				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				versionedResource := dbng.VersionedResource{
					Resource: "some-explicit-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"some": "version",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
				}

				err = build.SaveOutput(versionedResource, true)
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput(dbng.VersionedResource{
					Resource: "some-implicit-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"some": "version",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
				}, false)
				Expect(err).ToNot(HaveOccurred())

				actualBuildOutput, err := build.GetVersionedResources()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(actualBuildOutput)).To(Equal(1))
				Expect(actualBuildOutput[0].VersionedResource).To(Equal(versionedResource))
			})
		})

		Context("when a one off build", func() {
			It("can not get a build's output", func() {
				build, err := team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				versionedResource := dbng.VersionedResource{
					Resource: "some-explicit-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"some": "version",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
				}

				err = build.SaveOutput(versionedResource, true)
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput(dbng.VersionedResource{
					Resource: "some-implicit-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"some": "version",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
				}, false)
				Expect(err).ToNot(HaveOccurred())

				actualBuildOutput, err := build.GetVersionedResources()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(actualBuildOutput)).To(Equal(0))
			})
		})
	})

	Describe("GetResources", func() {
		var (
			pipeline dbng.Pipeline
			job      dbng.Job
			vr1      dbng.VersionedResource
			vr2      dbng.VersionedResource
		)

		BeforeEach(func() {
			vr1 = dbng.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version:  dbng.ResourceVersion{"ver": "1"},
			}

			vr2 = dbng.VersionedResource{
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
						Name: "some-resource",
						Type: "some-type",
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("correctly distinguishes them", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			// save a normal 'get'
			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: vr1,
			})
			Expect(err).NotTo(HaveOccurred())

			// save implicit output from 'get'
			err = build.SaveOutput(vr1, false)
			Expect(err).NotTo(HaveOccurred())

			// save explicit output from 'put'
			err = build.SaveOutput(vr2, true)
			Expect(err).NotTo(HaveOccurred())

			// save the dependent get
			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-dependent-input",
				VersionedResource: vr2,
			})
			Expect(err).NotTo(HaveOccurred())

			// save the dependent 'get's implicit output
			err = build.SaveOutput(vr2, false)
			Expect(err).NotTo(HaveOccurred())

			inputs, outputs, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]dbng.BuildInput{
				{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
			}))

			Expect(outputs).To(ConsistOf([]dbng.BuildOutput{
				{VersionedResource: vr2},
			}))

		})

		It("fails to save build output if resource does not exist", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			vr := dbng.VersionedResource{
				Resource: "unknown-resource",
				Type:     "some-type",
				Version:  dbng.ResourceVersion{"ver": "2"},
			}

			err = build.SaveOutput(vr, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("resource 'unknown-resource' not found"))
		})
	})

	Describe("Pipeline", func() {
		var (
			build           dbng.Build
			foundPipeline   dbng.Pipeline
			createdPipeline dbng.Pipeline
			found           bool
		)

		JustBeforeEach(func() {
			var err error
			foundPipeline, found, err = build.Pipeline()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when a job build", func() {
			BeforeEach(func() {
				var err error
				createdPipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				job, found, err := createdPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the correct pipeline", func() {
				Expect(found).To(BeTrue())
				Expect(foundPipeline).To(Equal(createdPipeline))
			})
		})

		Context("when a one off build", func() {
			BeforeEach(func() {
				var err error
				build, err = team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not return a pipeline", func() {
				Expect(found).To(BeFalse())
				Expect(foundPipeline).To(BeNil())
			})
		})
	})

	Describe("Preparation", func() {
		var (
			build             dbng.Build
			err               error
			expectedBuildPrep dbng.BuildPreparation
		)
		BeforeEach(func() {
			expectedBuildPrep = dbng.BuildPreparation{
				BuildID:             123456789,
				PausedPipeline:      dbng.BuildPreparationStatusNotBlocking,
				PausedJob:           dbng.BuildPreparationStatusNotBlocking,
				MaxRunningBuilds:    dbng.BuildPreparationStatusNotBlocking,
				Inputs:              map[string]dbng.BuildPreparationStatus{},
				InputsSatisfied:     dbng.BuildPreparationStatusNotBlocking,
				MissingInputReasons: dbng.MissingInputReasons{},
			}
		})

		Context("for one-off build", func() {
			BeforeEach(func() {
				build, err = team.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				expectedBuildPrep.BuildID = build.ID()
			})

			It("returns build preparation", func() {
				buildPrep, found, err := build.Preparation()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(buildPrep).To(Equal(expectedBuildPrep))
			})

			Context("when the build is started", func() {
				BeforeEach(func() {
					started, err := build.Start("some-engine", "some-metadata")
					Expect(started).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())

					stillExists, err := build.Reload()
					Expect(stillExists).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns build preparation", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})
		})

		Context("for job build", func() {
			var (
				pipeline dbng.Pipeline
				job      dbng.Job
			)

			BeforeEach(func() {
				var err error
				pipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
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
						},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				job, found, err = pipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				expectedBuildPrep.BuildID = build.ID()

				job, found, err = pipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			Context("when inputs are satisfied", func() {
				BeforeEach(func() {
					err = pipeline.SaveResourceVersions(
						atc.ResourceConfig{
							Name: "some-resource",
							Type: "some-type",
						},
						[]atc.Version{
							{"version": "v5"},
						},
					)
					Expect(err).NotTo(HaveOccurred())

					versions, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					job.SaveNextInputMapping(algorithm.InputMapping{
						"some-input": {VersionID: versions[0].ID, FirstOccurrence: true},
					})

					expectedBuildPrep.Inputs = map[string]dbng.BuildPreparationStatus{
						"some-input": dbng.BuildPreparationStatusNotBlocking,
					}
				})

				Context("when the build is started", func() {
					BeforeEach(func() {
						started, err := build.Start("some-engine", "some-metadata")
						Expect(started).To(BeTrue())
						Expect(err).NotTo(HaveOccurred())

						stillExists, err := build.Reload()
						Expect(stillExists).To(BeTrue())
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.Inputs = map[string]dbng.BuildPreparationStatus{}
					})

					It("returns build preparation", func() {
						buildPrep, found, err := build.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when pipeline is paused", func() {
					BeforeEach(func() {
						err := pipeline.Pause()
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.PausedPipeline = dbng.BuildPreparationStatusBlocking
					})

					It("returns build preparation with paused pipeline", func() {
						buildPrep, found, err := build.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when job is paused", func() {
					BeforeEach(func() {
						err := job.Pause()
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.PausedJob = dbng.BuildPreparationStatusBlocking
					})

					It("returns build preparation with paused pipeline", func() {
						buildPrep, found, err := build.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when max running builds is reached", func() {
					BeforeEach(func() {
						err := job.SetMaxInFlightReached(true)
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.MaxRunningBuilds = dbng.BuildPreparationStatusBlocking
					})

					It("returns build preparation with max in flight reached", func() {
						buildPrep, found, err := build.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when max running builds is de-reached", func() {
					BeforeEach(func() {
						err := job.SetMaxInFlightReached(true)
						Expect(err).NotTo(HaveOccurred())

						err = job.SetMaxInFlightReached(false)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns build preparation with max in flight not reached", func() {
						buildPrep, found, err := build.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})
			})

			Context("when inputs are not satisfied", func() {
				BeforeEach(func() {
					expectedBuildPrep.InputsSatisfied = dbng.BuildPreparationStatusBlocking
				})

				It("returns blocking inputs satisfied", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})

			Context("when some inputs are not satisfied", func() {
				BeforeEach(func() {
					pipelineConfig := atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								Plan: atc.PlanSequence{
									{Get: "input1"},
									{Get: "input2"},
									{Get: "input3", Passed: []string{"some-upstream-job"}},
									{ // version doesn't exist
										Get:     "input4",
										Version: &atc.VersionConfig{Pinned: atc.Version{"version": "v4"}},
									},
									{ // version doesn't exist so constraint is irrelevant
										Get:     "input5",
										Passed:  []string{"some-upstream-job"},
										Version: &atc.VersionConfig{Pinned: atc.Version{"version": "v5"}},
									},
									{ // version exists but doesn't satisfy constraint
										Get:     "input6",
										Passed:  []string{"some-upstream-job"},
										Version: &atc.VersionConfig{Pinned: atc.Version{"version": "v6"}},
									},
								},
							},
						},
						Resources: atc.ResourceConfigs{
							{Name: "input1", Type: "some-type"},
							{Name: "input2", Type: "some-type"},
							{Name: "input3", Type: "some-type"},
							{Name: "input4", Type: "some-type"},
							{Name: "input5", Type: "some-type"},
							{Name: "input6", Type: "some-type"},
						},
					}

					pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(2), dbng.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())

					err = pipeline.SaveResourceVersions(
						atc.ResourceConfig{
							Name: "input1",
							Type: "some-type",
						},
						[]atc.Version{
							{"version": "v1"},
						},
					)
					Expect(err).NotTo(HaveOccurred())

					err = pipeline.SaveResourceVersions(
						atc.ResourceConfig{
							Name: "input6",
							Type: "some-type",
						},
						[]atc.Version{
							{"version": "v6"},
						},
					)
					Expect(err).NotTo(HaveOccurred())

					job, found, err := pipeline.Job("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					versions, _, found, err := pipeline.GetResourceVersions("input1", dbng.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					job.SaveIndependentInputMapping(algorithm.InputMapping{
						"input1": {VersionID: versions[0].ID, FirstOccurrence: true},
					})

					expectedBuildPrep.Inputs = map[string]dbng.BuildPreparationStatus{
						"input1": dbng.BuildPreparationStatusNotBlocking,
						"input2": dbng.BuildPreparationStatusBlocking,
						"input3": dbng.BuildPreparationStatusBlocking,
						"input4": dbng.BuildPreparationStatusBlocking,
						"input5": dbng.BuildPreparationStatusBlocking,
						"input6": dbng.BuildPreparationStatusBlocking,
					}
					expectedBuildPrep.InputsSatisfied = dbng.BuildPreparationStatusBlocking
					expectedBuildPrep.MissingInputReasons = dbng.MissingInputReasons{
						"input2": dbng.NoVersionsAvailable,
						"input3": dbng.NoVerionsSatisfiedPassedConstraints,
						"input4": fmt.Sprintf(dbng.PinnedVersionUnavailable, `{"version":"v4"}`),
						"input5": fmt.Sprintf(dbng.PinnedVersionUnavailable, `{"version":"v5"}`),
						"input6": dbng.NoVerionsSatisfiedPassedConstraints,
					}
				})

				It("returns blocking inputs satisfied", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})
		})

		Describe("Schedule", func() {
			var (
				build dbng.Build
				found bool
				f     bool
			)

			BeforeEach(func() {
				pipeline, _, err := team.SavePipeline("some-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
				}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				job, found, err := pipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				Expect(build.IsScheduled()).To(BeFalse())
			})

			JustBeforeEach(func() {
				found, err = build.Schedule()
				Expect(err).ToNot(HaveOccurred())

				f, err = build.Reload()
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when build exists", func() {
				It("sets the build to scheduled", func() {
					Expect(f).To(BeTrue())
					Expect(found).To(BeTrue())
					Expect(build.IsScheduled()).To(BeTrue())
				})
			})

			Context("when the build does not exist", func() {
				var found2 bool
				BeforeEach(func() {
					var err error
					found2, err = build.Delete()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns false", func() {
					Expect(f).To(BeFalse())
					Expect(found2).To(BeTrue())
					Expect(found).To(BeFalse())
				})
			})
		})
	})

	Describe("Resources", func() {
		It("can get (no) resources from a one-off build", func() {
			oneOffBuild, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			inputs, outputs, err := oneOffBuild.Resources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(BeEmpty())
			Expect(outputs).To(BeEmpty())
		})
	})

	Describe("UseInput", func() {
		var build dbng.Build
		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
					},
					{
						Name: "some-other-resource",
						Type: "some-other-type",
					},
					{
						Name: "weird",
						Type: "type",
					},
				},
			}

			var err error
			pipeline, _, err := team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			versionedResource := dbng.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: dbng.ResourceVersion{
					"some": "version",
				},
				Metadata: []dbng.ResourceMetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
			}
			err = build.SaveInput(dbng.BuildInput{
				Name:              "some-input",
				VersionedResource: versionedResource,
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("uses provided build inputs", func() {
			someVersionedResource := dbng.VersionedResource{
				Resource: "some-other-resource",
				Type:     "some-other-type",
				Version: dbng.ResourceVersion{
					"some": "weird-version",
				},
				Metadata: []dbng.ResourceMetadataField{
					{
						Name:  "meta3",
						Value: "data3",
					},
				},
			}

			someWeirdResource := dbng.VersionedResource{
				Resource: "weird",
				Type:     "type",
			}
			var err error
			err = build.UseInputs([]dbng.BuildInput{
				{
					Name:              "some-other-input",
					VersionedResource: someVersionedResource,
				},
				{
					Name:              "some-weird-input",
					VersionedResource: someWeirdResource,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			actualBuildInput, err := build.GetVersionedResources()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildInput)).To(Equal(2))
			Expect(actualBuildInput[0].VersionedResource).To(Equal(someVersionedResource))
			Expect(actualBuildInput[1].VersionedResource).To(Equal(someWeirdResource))
		})
	})

	Describe("MarkAsFailed", func() {
		var cause error
		var build dbng.Build

		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			cause = errors.New("disaster")
			err := build.MarkAsFailed(cause)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Error event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusErrored))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			Expect(events.Next()).To(Equal(envelope(event.Error{
				Message: "disaster",
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(dbng.BuildStatusErrored))
		})
	})
})

func envelope(ev atc.Event) event.Envelope {
	payload, err := json.Marshal(ev)
	Expect(err).ToNot(HaveOccurred())

	data := json.RawMessage(payload)

	return event.Envelope{
		Event:   ev.EventType(),
		Version: ev.Version(),
		Data:    &data,
	}
}
