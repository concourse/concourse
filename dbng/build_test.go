package dbng_test

import (
	"encoding/json"

	"github.com/concourse/atc"
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
		})

		Context("when a job build", func() {
			It("saves the build's input", func() {
				build, err := pipeline.CreateJobBuild("some-job")
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
		})
		Context("when a job build", func() {
			It("can get a build's output", func() {
				build, err := pipeline.CreateJobBuild("some-job")
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

				build, err = createdPipeline.CreateJobBuild("some-job")
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
