package db_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var teamDB db.TeamDB
	var pipelineDB db.PipelineDB
	var pipeline db.SavedPipeline
	var pipelineConfig atc.Config

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus)
		teamDB = teamDBFactory.GetTeamDB(atc.DefaultTeamName)

		pipelineConfig = atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
				{
					Name: "some-other-job",
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
				},
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
		pipeline, _, err = teamDB.SaveConfig("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus)
		pipelineDB = pipelineDBFactory.Build(pipeline)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("IsOneOff", func() {
		It("returns true for one off build", func() {
			oneOffBuild, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(oneOffBuild.IsOneOff()).To(BeTrue())
		})

		It("returns false for builds that belong to pipeline", func() {
			pipelineBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(pipelineBuild.IsOneOff()).To(BeFalse())
		})
	})

	Describe("Reload", func() {
		It("updates the model", func() {
			build, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			started, err := build.Start("engine", "metadata")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			Expect(build.Status()).To(Equal(db.StatusPending))

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.StatusStarted))
		})
	})

	Describe("SaveEvent", func() {
		It("saves and propagates events correctly", func() {
			build, err := teamDB.CreateOneOffBuild()
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
			}).Should(Equal(db.ErrBuildEventStreamClosed))
		})
	})

	Describe("Events", func() {
		It("saves and emits status events", func() {
			build, err := teamDB.CreateOneOffBuild()
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
			err = build.Finish(db.StatusSucceeded)
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

	Describe("SaveInput", func() {
		It("can get a build's input", func() {
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())

			expectedBuildInput, err := pipelineDB.SaveInput(build.ID(), db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.Version{
						"some": "version",
					},
					Metadata: []db.MetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
					PipelineID: pipeline.ID,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			actualBuildInput, err := build.GetVersionedResources()
			expectedBuildInput.CheckOrder = 0
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildInput)).To(Equal(1))
			Expect(actualBuildInput[0]).To(Equal(expectedBuildInput))
		})
	})

	Describe("SaveOutput", func() {
		It("can get a build's output", func() {
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())

			expectedBuildOutput, err := pipelineDB.SaveOutput(build.ID(), db.VersionedResource{
				Resource: "some-explicit-resource",
				Type:     "some-type",
				Version: db.Version{
					"some": "version",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
				PipelineID: pipeline.ID,
			}, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = pipelineDB.SaveOutput(build.ID(), db.VersionedResource{
				Resource: "some-implicit-resource",
				Type:     "some-type",
				Version: db.Version{
					"some": "version",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
				PipelineID: pipeline.ID,
			}, false)
			Expect(err).ToNot(HaveOccurred())

			actualBuildOutput, err := build.GetVersionedResources()
			expectedBuildOutput.CheckOrder = 0
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildOutput)).To(Equal(1))
			Expect(actualBuildOutput[0]).To(Equal(expectedBuildOutput))
		})
	})

	Describe("GetResources", func() {
		It("can get (no) resources from a one-off build", func() {
			oneOffBuild, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			inputs, outputs, err := oneOffBuild.GetResources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(BeEmpty())
			Expect(outputs).To(BeEmpty())
		})
	})

	Describe("build operations", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Start", func() {
			JustBeforeEach(func() {
				started, err := build.Start("engine", "metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("creates Start event", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.StatusStarted))

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
				Expect(build.Status()).To(Equal(db.StatusStarted))
			})
		})

		Describe("Abort", func() {
			JustBeforeEach(func() {
				err := build.Abort()
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates build status", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.StatusAborted))
			})
		})

		Describe("Finish", func() {
			JustBeforeEach(func() {
				err := build.Finish(db.StatusSucceeded)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates Finish event", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.StatusSucceeded))

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
				Expect(build.Status()).To(Equal(db.StatusSucceeded))
			})
		})

		Describe("MarkAsFailed", func() {
			var cause error

			JustBeforeEach(func() {
				cause = errors.New("disaster")
				err := build.MarkAsFailed(cause)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates Error event", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.StatusErrored))

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
				Expect(build.Status()).To(Equal(db.StatusErrored))
			})
		})
	})

	Describe("GetBuildPreparation", func() {
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
				build, err = teamDB.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				expectedBuildPrep.BuildID = build.ID()
			})

			It("returns build preparation", func() {
				buildPrep, found, err := build.GetPreparation()
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

				It("doesn't return build preparation", func() {
					_, found, err := build.GetPreparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("for job build", func() {
			BeforeEach(func() {
				build, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				expectedBuildPrep.BuildID = build.ID()
			})

			Context("when inputs are satisfied", func() {
				BeforeEach(func() {
					err = pipelineDB.SaveResourceVersions(
						atc.ResourceConfig{
							Name: "some-resource",
							Type: "some-type",
						},
						[]atc.Version{
							{"version": "v5"},
						},
					)
					Expect(err).NotTo(HaveOccurred())

					versions, _, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					pipelineDB.SaveNextInputMapping(algorithm.InputMapping{
						"some-input": {VersionID: versions[0].ID, FirstOccurrence: true},
					}, "some-job")

					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
						"some-input": db.BuildPreparationStatusNotBlocking,
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
					})

					It("doesn't return build preparation", func() {
						_, found, err := build.GetPreparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeFalse())
					})
				})

				Context("when pipeline is paused", func() {
					BeforeEach(func() {
						err := pipelineDB.Pause()
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
					})

					It("returns build preparation with paused pipeline", func() {
						buildPrep, found, err := build.GetPreparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when job is paused", func() {
					BeforeEach(func() {
						err := pipelineDB.PauseJob("some-job")
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.PausedJob = db.BuildPreparationStatusBlocking
					})

					It("returns build preparation with paused pipeline", func() {
						buildPrep, found, err := build.GetPreparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when max running builds is reached", func() {
					BeforeEach(func() {
						err := pipelineDB.SetMaxInFlightReached("some-job", true)
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.MaxRunningBuilds = db.BuildPreparationStatusBlocking
					})

					It("returns build preparation with max in flight reached", func() {
						buildPrep, found, err := build.GetPreparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})

				Context("when max running builds is de-reached", func() {
					BeforeEach(func() {
						err := pipelineDB.SetMaxInFlightReached("some-job", true)
						Expect(err).NotTo(HaveOccurred())

						err = pipelineDB.SetMaxInFlightReached("some-job", false)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns build preparation with max in flight not reached", func() {
						buildPrep, found, err := build.GetPreparation()
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
					buildPrep, found, err := build.GetPreparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})

			Context("when some inputs are not satisfied", func() {
				BeforeEach(func() {
					pipelineConfig = atc.Config{
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

					pipeline, _, err = teamDB.SaveConfig("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
					Expect(err).NotTo(HaveOccurred())

					err = pipelineDB.SaveResourceVersions(
						atc.ResourceConfig{
							Name: "input1",
							Type: "some-type",
						},
						[]atc.Version{
							{"version": "v1"},
						},
					)
					Expect(err).NotTo(HaveOccurred())

					err = pipelineDB.SaveResourceVersions(
						atc.ResourceConfig{
							Name: "input6",
							Type: "some-type",
						},
						[]atc.Version{
							{"version": "v6"},
						},
					)
					Expect(err).NotTo(HaveOccurred())

					versions, _, found, err := pipelineDB.GetResourceVersions("input1", db.Page{Limit: 1})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))

					pipelineDB.SaveIndependentInputMapping(algorithm.InputMapping{
						"input1": {VersionID: versions[0].ID, FirstOccurrence: true},
					}, "some-job")

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
					buildPrep, found, err := build.GetPreparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})
		})

		Context("for job that is still checking resources", func() {
			var build1, build2 db.Build
			BeforeEach(func() {
				pipelineConfig = atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{Get: "input1"},
								{Get: "input2"},
							},
						},
					},
				}

				pipeline, _, err = teamDB.SaveConfig("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				build1, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, created, err := pipelineDB.LeaseResourceCheckingForJob(
					lagertest.NewTestLogger("build-preparation"),
					"some-job",
					5*time.Minute,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(created).To(BeTrue())

				build2, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns inputs satisfied blocking for checked build", func() {
				expectedBuildPrep.BuildID = build1.ID()
				expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
					"input1": db.BuildPreparationStatusBlocking,
					"input2": db.BuildPreparationStatusBlocking,
				}
				expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
				expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
					"input1": db.NoVersionsAvailable,
					"input2": db.NoVersionsAvailable,
				}

				buildPrep, found, err := build1.GetPreparation()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(buildPrep).To(Equal(expectedBuildPrep))
			})

			It("returns inputs satisfied unknown for checking build", func() {
				expectedBuildPrep.BuildID = build2.ID()
				expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusUnknown

				buildPrep, found, err := build2.GetPreparation()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(buildPrep).To(Equal(expectedBuildPrep))
			})
		})
	})

	Describe("GetConfig", func() {
		It("returns config of build pipeline", func() {
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())

			actualConfig, actualConfigVersion, err := build.GetConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(actualConfig).To(Equal(pipelineConfig))
			Expect(actualConfigVersion).To(Equal(db.ConfigVersion(1)))
		})
	})

	Describe("GetPipeline", func() {
		Context("when build belongs to pipeline", func() {
			It("returns the pipeline", func() {
				build, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).ToNot(HaveOccurred())

				buildPipeline, err := build.GetPipeline()
				Expect(err).NotTo(HaveOccurred())
				Expect(buildPipeline).To(Equal(pipeline))
			})
		})

		Context("when build is one off", func() {
			It("returns empty pipeline", func() {
				build, err := teamDB.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				buildPipeline, err := build.GetPipeline()
				Expect(err).NotTo(HaveOccurred())
				Expect(buildPipeline).To(Equal(db.SavedPipeline{}))
			})
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
