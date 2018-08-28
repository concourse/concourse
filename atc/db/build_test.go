package db_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/event"
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

	Describe("Drain", func() {
		It("defaults drain to false in the beginning", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(build.IsDrained()).To(BeFalse())
		})

		It("has drain set to true after a drain and a reload", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.SetDrained(true)
			Expect(err).NotTo(HaveOccurred())

			var drained bool
			drained = build.IsDrained()
			Expect(drained).To(BeTrue())

			build.Reload()
			drained = build.IsDrained()
			Expect(drained).To(BeTrue())
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

	Describe("TrackedBy", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates build status", func() {
			Expect(build.TrackedBy("http://1.2.3.4:8080")).To(Succeed())

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build.Tracker()).To(Equal("http://1.2.3.4:8080"))
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
		var resourceConfig db.ResourceConfig

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
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
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())
		})

		It("saves the build's input", func() {
			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig.SaveVersions([]atc.Version{atc.Version{"some": "version"}})
			Expect(err).ToNot(HaveOccurred())

			rcv, found, err := resourceConfig.FindVersion(atc.Version{"some": "version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = build.SaveInput(db.BuildInput{
				Name:                    "some-input",
				ResourceConfigVersionID: rcv.ID(),
				Version:                 atc.Version{"some": "version"},
			})
			Expect(err).ToNot(HaveOccurred())

			actualBuildInput, _, err := build.Resources()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildInput)).To(Equal(1))
			Expect(actualBuildInput[0].ResourceConfigVersionID).To(Equal(rcv.ID()))
			Expect(actualBuildInput[0].Version).To(Equal(atc.Version{"some": "version"}))
		})
	})

	Describe("SaveOutput", func() {
		var pipeline db.Pipeline
		var job db.Job
		var resourceConfig db.ResourceConfig

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
						Name:   "some-explicit-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "explicit-source"},
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

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "explicit-source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := pipeline.Resource("some-explicit-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())
		})

		It("can save a build's output", func() {
			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build.SaveOutput(resourceConfig, atc.Version{"some": "version"}, []db.ResourceConfigMetadataField{
				{
					Name:  "meta1",
					Value: "data1",
				},
				{
					Name:  "meta2",
					Value: "data2",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			rcv, found, err := resourceConfig.FindVersion(atc.Version{"some": "version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualBuildOutput, err := build.ResourceConfigVersions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildOutput)).To(Equal(1))
			Expect(actualBuildOutput[0]).To(Equal(rcv.ID()))
		})
	})

	Describe("Resources", func() {
		var (
			pipeline        db.Pipeline
			job             db.Job
			resourceConfig1 db.ResourceConfig
			resourceConfig2 db.ResourceConfig
		)

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			// XXX: DELETE WHEN NOT NEEDED
			// vr2 = db.VersionedResource{
			// 	Resource: "some-other-resource",
			// 	Type:     "some-type",
			// 	Version:  db.ResourceVersion{"ver": "2"},
			// }

			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
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
						Source: atc.Source{"some": "source-2"},
					},
				},
			}

			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfig1, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resourceConfig2, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source-2"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resource1, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource2, found, err := pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource1.SetResourceConfig(resourceConfig1.ID())
			Expect(err).ToNot(HaveOccurred())

			err = resource2.SetResourceConfig(resourceConfig2.ID())
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig1.SaveVersions([]atc.Version{atc.Version{"ver": "1"}})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig2.SaveVersions([]atc.Version{atc.Version{"ver": "2"}})
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns build inputs and outputs", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			rcv1, found, err := resourceConfig1.FindVersion(atc.Version{"ver": "1"})
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// save a normal 'get'
			err = build.SaveInput(db.BuildInput{
				Name:                    "some-input",
				Version:                 atc.Version{"ver": "1"},
				ResourceConfigVersionID: rcv1.ID(),
			})
			Expect(err).NotTo(HaveOccurred())

			// save explicit output from 'put'
			err = build.SaveOutput(resourceConfig2, atc.Version{"ver": "2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			// XXX: ADD OUTPUTS
			inputs, _, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(ConsistOf([]db.BuildInput{
				{Name: "some-input", Version: atc.Version{"ver": "1"}, ResourceConfigVersionID: rcv1.ID(), FirstOccurrence: true},
			}))

			// XXX: FIX PLS
			// Expect(outputs).To(ConsistOf([]db.BuildOutput{
			// 	{VersionedResource: vr2},
			// }))
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
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}
					_, err = brt.FindOrCreate(setupTx)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfig.SaveVersions([]atc.Version{{"version": "v5"}})
					Expect(err).NotTo(HaveOccurred())

					rcv, found, err := resourceConfig.FindVersion(atc.Version{"version": "v5"})
					Expect(found).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(algorithm.InputMapping{
						"some-input": {VersionID: rcv.ID(), FirstOccurrence: true},
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
									{
										Get:     "input1",
										Version: &atc.VersionConfig{Pinned: atc.Version{"version": "v1"}},
									},
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
							{Name: "input1", Type: "some-type", Source: atc.Source{"some": "source-1"}},
							{Name: "input2", Type: "some-type", Source: atc.Source{"some": "source-2"}},
							{Name: "input3", Type: "some-type", Source: atc.Source{"some": "source-3"}},
							{Name: "input4", Type: "some-type", Source: atc.Source{"some": "source-4"}},
							{Name: "input5", Type: "some-type", Source: atc.Source{"some": "source-5"}},
							{Name: "input6", Type: "some-type", Source: atc.Source{"some": "source-6"}},
						},
					}

					pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(2), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}
					_, err = brt.FindOrCreate(setupTx)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceConfig1, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source-1"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfig1.SaveVersions([]atc.Version{{"version": "v1"}})
					Expect(err).NotTo(HaveOccurred())

					resourceConfig6, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source-6"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					resource6, found, err := pipeline.Resource("input6")
					Expect(found).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())

					err = resource6.SetResourceConfig(resourceConfig6.ID())
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfig6.SaveVersions([]atc.Version{{"version": "v6"}})
					Expect(err).NotTo(HaveOccurred())

					job, found, err := pipeline.Job("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					versions, _, found, err := resourceConfig1.Versions(db.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					err = job.SaveIndependentInputMapping(algorithm.InputMapping{
						"input1": {VersionID: versions[0].ID(), FirstOccurrence: true},
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
						"input3": db.NoVersionsSatisfiedPassedConstraints,
						"input4": fmt.Sprintf(db.PinnedVersionUnavailable, `{"version":"v4"}`),
						"input5": fmt.Sprintf(db.PinnedVersionUnavailable, `{"version":"v5"}`),
						"input6": db.NoVersionsSatisfiedPassedConstraints,
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

	Describe("UseInputs", func() {
		var build db.Build
		var pipeline db.Pipeline

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
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
						Type:   "some-other-type",
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "weird",
						Type:   "type",
						Source: atc.Source{"some": "source"},
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig.SaveVersions([]atc.Version{atc.Version{"some": "version"}})
			Expect(err).ToNot(HaveOccurred())

			rcv, found, err := resourceConfig.FindVersion(atc.Version{"some": "version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = build.SaveInput(db.BuildInput{
				Name:                    "some-input",
				ResourceConfigVersionID: rcv.ID(),
				Version:                 atc.Version{"some": "version"},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("uses provided build inputs", func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-other-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-other-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resource, found, err := pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig.SaveVersions([]atc.Version{atc.Version{"some": "weird-version"}})
			Expect(err).ToNot(HaveOccurred())

			rcv, found, err := resourceConfig.FindVersion(atc.Version{"some": "weird-version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx2, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt2 := db.BaseResourceType{
				Name: "type",
			}
			_, err = brt2.FindOrCreate(setupTx2)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx2.Commit()).To(Succeed())

			weirdRC, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			weirdResource, found, err := pipeline.Resource("weird")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = weirdResource.SetResourceConfig(weirdRC.ID())
			Expect(err).ToNot(HaveOccurred())

			err = weirdRC.SaveVersions([]atc.Version{atc.Version{"weird": "version"}})
			Expect(err).ToNot(HaveOccurred())

			weirdRCV, found, err := weirdRC.FindVersion(atc.Version{"weird": "version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = build.UseInputs([]db.BuildInput{
				{
					Name:                    "some-other-input",
					ResourceConfigVersionID: rcv.ID(),
					Version:                 atc.Version{"some": "weird-version"},
				},
				{
					Name:                    "some-weird-input",
					ResourceConfigVersionID: weirdRCV.ID(),
					Version:                 atc.Version{"werid": "version"},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			actualBuildInput, _, err := build.Resources()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildInput)).To(Equal(2))
			Expect(actualBuildInput[0].Name).To(Equal("some-other-input"))
			Expect(actualBuildInput[0].Version).To(Equal(atc.Version{"some": "weird-version"}))
			Expect(actualBuildInput[0].ResourceConfigVersionID).To(Equal(rcv.ID()))
			Expect(actualBuildInput[1].Name).To(Equal("some-weird-input"))
			Expect(actualBuildInput[1].Version).To(Equal(atc.Version{"weird": "version"}))
			Expect(actualBuildInput[1].ResourceConfigVersionID).To(Equal(weirdRCV.ID()))
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
