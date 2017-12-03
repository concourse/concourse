package db_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build", func() {
	var (
		team db.Team
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
			started, err := build.Start("engine", `{"meta":"data"}`, atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			Expect(build.Status()).To(Equal(db.BuildStatusPending))

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusStarted))
		})
	})

	Describe("Start", func() {
		var build db.Build
		var plan atc.Plan

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

			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := build.Start("engine", `{"meta":"data"}`, plan)
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())
		})

		It("creates Start event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusStarted))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   build.StartTime().Unix(),
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusStarted))
		})

		It("saves the public plan", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.PublicPlan()).To(Equal(plan.Public()))
		})
	})

	Describe("Finish", func() {
		var build db.Build
		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Finish event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusSucceeded))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusSucceeded,
				Time:   build.EndTime().Unix(),
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusSucceeded))
		})

		It("sets engine metadata to nil", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.EngineMetadata()).To(BeEmpty())
		})
	})

	Describe("Abort", func() {
		var build db.Build
		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.MarkAsAborted()
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusAborted))
		})
	})

	Describe("Events", func() {
		It("saves and emits status events", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			By("emitting a status event when started")
			started, err := build.Start("engine", `{"meta":"data"}`, atc.Plan{})
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
			err = build.Finish(db.BuildStatusSucceeded)
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
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))
		})
	})

	Describe("SaveEvent", func() {
		It("saves and propagates events correctly", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

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

			defer db.Close(eventsFrom1)

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
			}).Should(Equal(db.ErrBuildEventStreamClosed))
		})
	})

	Describe("SaveInput", func() {
		var pipeline db.Pipeline
		var job db.Job

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
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
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

				versionedResource := db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"some": "version",
					},
					Metadata: []db.ResourceMetadataField{
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
				err = build.SaveInput(db.BuildInput{
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

				versionedResource := db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"some": "version",
					},
					Metadata: []db.ResourceMetadataField{
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
				err = build.SaveInput(db.BuildInput{
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
		var pipeline db.Pipeline
		var job db.Job

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
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
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

				versionedResource := db.VersionedResource{
					Resource: "some-explicit-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"some": "version",
					},
					Metadata: []db.ResourceMetadataField{
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

				err = build.SaveOutput(db.VersionedResource{
					Resource: "some-implicit-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"some": "version",
					},
					Metadata: []db.ResourceMetadataField{
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

				versionedResource := db.VersionedResource{
					Resource: "some-explicit-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"some": "version",
					},
					Metadata: []db.ResourceMetadataField{
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

				err = build.SaveOutput(db.VersionedResource{
					Resource: "some-implicit-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"some": "version",
					},
					Metadata: []db.ResourceMetadataField{
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
			pipeline db.Pipeline
			job      db.Job
			vr1      db.VersionedResource
			vr2      db.VersionedResource
		)

		BeforeEach(func() {
			vr1 = db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version:  db.ResourceVersion{"ver": "1"},
			}

			vr2 = db.VersionedResource{
				Resource: "some-other-resource",
				Type:     "some-type",
				Version:  db.ResourceVersion{"ver": "2"},
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
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
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
			err = build.SaveInput(db.BuildInput{
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
			err = build.SaveInput(db.BuildInput{
				Name:              "some-dependent-input",
				VersionedResource: vr2,
			})
			Expect(err).NotTo(HaveOccurred())

			// save the dependent 'get's implicit output
			err = build.SaveOutput(vr2, false)
			Expect(err).NotTo(HaveOccurred())

			inputs, outputs, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(inputs).To(ConsistOf([]db.BuildInput{
				{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
			}))

			Expect(outputs).To(ConsistOf([]db.BuildOutput{
				{VersionedResource: vr2},
			}))

		})

		It("fails to save build output if resource does not exist", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			vr := db.VersionedResource{
				Resource: "unknown-resource",
				Type:     "some-type",
				Version:  db.ResourceVersion{"ver": "2"},
			}

			err = build.SaveOutput(vr, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("resource 'unknown-resource' not found"))
		})
	})

	Describe("Pipeline", func() {
		var (
			build           db.Build
			foundPipeline   db.Pipeline
			createdPipeline db.Pipeline
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
				}, db.ConfigVersion(1), db.PipelineUnpaused)
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
			build             db.Build
			err               error
			expectedBuildPrep db.BuildPreparation
		)
		BeforeEach(func() {
			expectedBuildPrep = db.BuildPreparation{
				BuildID:             123456789,
				PausedPipeline:      db.BuildPreparationStatusNotBlocking,
				PausedJob:           db.BuildPreparationStatusNotBlocking,
				MaxRunningBuilds:    db.BuildPreparationStatusNotBlocking,
				Inputs:              map[string]db.BuildPreparationStatus{},
				InputsSatisfied:     db.BuildPreparationStatusNotBlocking,
				MissingInputReasons: db.MissingInputReasons{},
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
					started, err := build.Start("some-engine", `{"meta":"data"}`, atc.Plan{})
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
				pipeline db.Pipeline
				job      db.Job
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
				}, db.ConfigVersion(1), db.PipelineUnpaused)
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

					versions, _, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					err = job.SaveNextInputMapping(algorithm.InputMapping{
						"some-input": {VersionID: versions[0].ID, FirstOccurrence: true},
					})
					Expect(err).NotTo(HaveOccurred())

					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
						"some-input": db.BuildPreparationStatusNotBlocking,
					}
				})

				Context("when the build is started", func() {
					BeforeEach(func() {
						started, err := build.Start("some-engine", `{"meta":"data"}`, atc.Plan{})
						Expect(started).To(BeTrue())
						Expect(err).NotTo(HaveOccurred())

						stillExists, err := build.Reload()
						Expect(stillExists).To(BeTrue())
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{}
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

						expectedBuildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
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

						expectedBuildPrep.PausedJob = db.BuildPreparationStatusBlocking
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

						expectedBuildPrep.MaxRunningBuilds = db.BuildPreparationStatusBlocking
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
					expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
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

					pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(2), db.PipelineUnpaused)
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

					versions, _, found, err := pipeline.GetResourceVersions("input1", db.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					err = job.SaveIndependentInputMapping(algorithm.InputMapping{
						"input1": {VersionID: versions[0].ID, FirstOccurrence: true},
					})
					Expect(err).NotTo(HaveOccurred())

					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
						"input1": db.BuildPreparationStatusNotBlocking,
						"input2": db.BuildPreparationStatusBlocking,
						"input3": db.BuildPreparationStatusBlocking,
						"input4": db.BuildPreparationStatusBlocking,
						"input5": db.BuildPreparationStatusBlocking,
						"input6": db.BuildPreparationStatusBlocking,
					}
					expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
					expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
						"input2": db.NoVersionsAvailable,
						"input3": db.NoVerionsSatisfiedPassedConstraints,
						"input4": fmt.Sprintf(db.PinnedVersionUnavailable, `{"version":"v4"}`),
						"input5": fmt.Sprintf(db.PinnedVersionUnavailable, `{"version":"v5"}`),
						"input6": db.NoVerionsSatisfiedPassedConstraints,
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
				build db.Build
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
				}, db.ConfigVersion(1), db.PipelineUnpaused)
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
		var build db.Build
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
			pipeline, _, err := team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			versionedResource := db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.ResourceVersion{
					"some": "version",
				},
				Metadata: []db.ResourceMetadataField{
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
			err = build.SaveInput(db.BuildInput{
				Name:              "some-input",
				VersionedResource: versionedResource,
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("uses provided build inputs", func() {
			someVersionedResource := db.VersionedResource{
				Resource: "some-other-resource",
				Type:     "some-other-type",
				Version: db.ResourceVersion{
					"some": "weird-version",
				},
				Metadata: []db.ResourceMetadataField{
					{
						Name:  "meta3",
						Value: "data3",
					},
				},
			}

			someWeirdResource := db.VersionedResource{
				Resource: "weird",
				Type:     "type",
			}

			err := build.UseInputs([]db.BuildInput{
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

	Describe("FinishWithError", func() {
		var cause error
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			cause = errors.New("disaster")
			err := build.FinishWithError(cause)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Error event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusErrored))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			Expect(events.Next()).To(Equal(envelope(event.Error{
				Message: "disaster",
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusErrored))
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
