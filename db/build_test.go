package db_test

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"

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
