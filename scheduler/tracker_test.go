package scheduler_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Tracker", func() {
	var (
		trackerDB *fakes.FakeTrackerDB

		tracker BuildTracker

		turbineServer *ghttp.Server
	)

	BeforeEach(func() {
		trackerDB = new(fakes.FakeTrackerDB)

		turbineServer = ghttp.NewServer()

		tracker = NewTracker(trackerDB)
	})

	AfterEach(func() {
		turbineServer.CloseClientConnections()
		turbineServer.Close()
	})

	Describe("TrackBuild", func() {
		var (
			build builds.Build

			trackErr error
		)

		BeforeEach(func() {
			build = builds.Build{
				ID:       1,
				Guid:     "some-guid",
				Endpoint: turbineServer.URL(),
			}
		})

		JustBeforeEach(func() {
			trackErr = tracker.TrackBuild(build)
		})

		Context("when the build's turbine returns events", func() {
			var (
				events []event.Event
			)

			BeforeEach(func() {
				events = []event.Event{}

				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/builds/some-guid/events"),
						func(w http.ResponseWriter, r *http.Request) {
							flusher := w.(http.Flusher)

							w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
							w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Add("Connection", "keep-alive")

							w.WriteHeader(http.StatusOK)

							flusher.Flush()

							for id, e := range events {
								payload, err := json.Marshal(e)
								Ω(err).ShouldNot(HaveOccurred())

								event := sse.Event{
									ID:   fmt.Sprintf("%d", id),
									Name: string(e.EventType()),
									Data: []byte(payload),
								}

								err = event.Write(w)
								if err != nil {
									return
								}

								flusher.Flush()
							}
						},
					),
				)
			})

			itSavesTheEvent := func(count int) {
				It("saves the event", func() {
					Eventually(trackerDB.AppendBuildEventCallCount).Should(BeNumerically(">=", count))

					event := events[count-1]

					payload, err := json.Marshal(event)
					Ω(err).ShouldNot(HaveOccurred())

					buildID, buildEvent := trackerDB.AppendBuildEventArgsForCall(count - 1)
					Ω(buildID).Should(Equal(1))
					Ω(buildEvent).Should(Equal(db.BuildEvent{
						Type:    string(event.EventType()),
						Payload: string(payload),
					}))
				})
			}

			for _, v1version := range []string{"1.0", "1.1"} {
				version := v1version

				Context("and they're v"+version, func() {
					BeforeEach(func() {
						events = append(events, event.Version(version))
					})

					itSavesTheEvent(1)

					Context("and a status event appears", func() {
						Context("and it's started", func() {
							BeforeEach(func() {
								events = append(events, event.Status{
									Status: tbuilds.StatusStarted,
									Time:   1234,
								})
							})

							itSavesTheEvent(2)

							It("saves the build's status", func() {
								Eventually(trackerDB.SaveBuildStatusCallCount).Should(Equal(1))

								buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
								Ω(buildID).Should(Equal(1))
								Ω(status).Should(Equal(builds.StatusStarted))
							})

							It("saves the build's start time", func() {
								Eventually(trackerDB.SaveBuildStartTimeCallCount).Should(Equal(1))

								buildID, startTime := trackerDB.SaveBuildStartTimeArgsForCall(0)
								Ω(buildID).Should(Equal(1))
								Ω(startTime.Unix()).Should(Equal(int64(1234)))
							})
						})

						Context("and it's completed", func() {
							BeforeEach(func() {
								events = append(events, event.Status{
									Status: tbuilds.StatusSucceeded,
									Time:   1234,
								})
							})

							itSavesTheEvent(2)

							It("saves the build's status", func() {
								Eventually(trackerDB.SaveBuildStatusCallCount).Should(Equal(1))

								buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
								Ω(buildID).Should(Equal(1))
								Ω(status).Should(Equal(builds.StatusSucceeded))
							})

							It("saves the build's end time", func() {
								Eventually(trackerDB.SaveBuildEndTimeCallCount).Should(Equal(1))

								buildID, startTime := trackerDB.SaveBuildEndTimeArgsForCall(0)
								Ω(buildID).Should(Equal(1))
								Ω(startTime.Unix()).Should(Equal(int64(1234)))
							})
						})
					})

					Context("and an input event appears", func() {
						BeforeEach(func() {
							events = append(events, event.Input{
								Input: tbuilds.Input{
									Name:    "some-input-name",
									Type:    "some-type",
									Source:  tbuilds.Source{"input-source": "some-source"},
									Version: tbuilds.Version{"version": "input-version"},
									Metadata: []tbuilds.MetadataField{
										{Name: "input-meta", Value: "some-value"},
									},
								},
							})
						})

						itSavesTheEvent(2)

						Context("and the build is for a job", func() {
							BeforeEach(func() {
								build.JobName = "lol"
							})

							It("saves the build's input", func() {
								Eventually(trackerDB.SaveBuildInputCallCount).Should(Equal(1))

								id, input := trackerDB.SaveBuildInputArgsForCall(0)
								Ω(id).Should(Equal(1))
								Ω(input).Should(Equal(builds.VersionedResource{
									Name:    "some-input-name",
									Type:    "some-type",
									Source:  builds.Source{"input-source": "some-source"},
									Version: builds.Version{"version": "input-version"},
									Metadata: []builds.MetadataField{
										{Name: "input-meta", Value: "some-value"},
									},
								}))
							})
						})

						Context("and the build is not for a job (i.e. one-off)", func() {
							BeforeEach(func() {
								build.JobName = ""
							})

							It("does not save an input", func() {
								Consistently(trackerDB.SaveBuildInputCallCount).Should(Equal(0))
							})
						})
					})

					Context("and an output event appears", func() {
						BeforeEach(func() {
							events = append(events, event.Output{
								Output: tbuilds.Output{
									Name:    "some-output-name",
									Type:    "some-type",
									Source:  tbuilds.Source{"output-source": "some-source"},
									Version: tbuilds.Version{"version": "output-version"},
									Metadata: []tbuilds.MetadataField{
										{Name: "output-meta", Value: "some-value"},
									},
								},
							})
						})

						Context("and the build is for a job", func() {
							BeforeEach(func() {
								build.JobName = "lol"
							})

							It("saves the build's output", func() {
								Eventually(trackerDB.SaveBuildOutputCallCount).Should(Equal(1))

								id, output := trackerDB.SaveBuildOutputArgsForCall(0)
								Ω(id).Should(Equal(1))
								Ω(output).Should(Equal(builds.VersionedResource{
									Name:    "some-output-name",
									Type:    "some-type",
									Source:  builds.Source{"output-source": "some-source"},
									Version: builds.Version{"version": "output-version"},
									Metadata: []builds.MetadataField{
										{Name: "output-meta", Value: "some-value"},
									},
								}))
							})
						})

						Context("and the build is not for a job (i.e. one-off)", func() {
							BeforeEach(func() {
								build.JobName = ""
							})

							It("does not save an output", func() {
								Consistently(trackerDB.SaveBuildOutputCallCount).Should(Equal(0))
							})
						})
					})

					Context("and an end event appears", func() {
						var buildDeleted <-chan struct{}

						BeforeEach(func() {
							events = append(events, event.End{})

							deleted := make(chan struct{})
							buildDeleted = deleted

							turbineServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("DELETE", "/builds/some-guid"),
									func(w http.ResponseWriter, r *http.Request) {
										close(deleted)
									},
								),
							)
						})

						It("deletes the build from the turbine", func() {
							Eventually(buildDeleted).Should(BeClosed())
						})
					})
				})
			}

			Context("and they're some other version", func() {
				BeforeEach(func() {
					events = append(
						events,
						event.Version("2.0"),
						event.Status{Status: tbuilds.StatusSucceeded},
					)
				})

				itSavesTheEvent(1)
				itSavesTheEvent(2)

				It("ignores the events", func() {
					Consistently(trackerDB.SaveBuildStatusCallCount).Should(Equal(0))
				})
			})

			Context("and the build is already being tracked", func() {
				BeforeEach(func() {
					tracker.TrackBuild(build)
				})

				It("does not track twice", func() {
					Ω(turbineServer.ReceivedRequests()).Should(HaveLen(1))
				})
			})
		})

		Context("when the build's turbine returns 404", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.RespondWith(http.StatusNotFound, ""),
				)
			})

			It("does not return an error", func() {
				Ω(trackErr).ShouldNot(HaveOccurred())
			})

			It("sets the build's status to errored", func() {
				// TODO some way of messaging this?

				Ω(trackerDB.SaveBuildStatusCallCount()).Should(Equal(1))

				buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
				Ω(buildID).Should(Equal(1))
				Ω(status).Should(Equal(builds.StatusErrored))
			})
		})

		Context("when the build's turbine is inaccessible", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						turbineServer.CloseClientConnections()
					},
				)
			})

			It("returns an error", func() {
				Ω(trackErr).Should(HaveOccurred())
			})

			It("does not update the build's status, as the turbine may be temporarily AWOL (i.e. rolling update)", func() {
				Ω(trackerDB.SaveBuildStatusCallCount()).Should(Equal(0))
			})
		})
	})
})
