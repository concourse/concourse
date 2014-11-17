package scheduler_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/concourse/turbine"
	"github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager/lagertest"
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

		lock   *dbfakes.FakeLock
		locker *fakes.FakeLocker
	)

	BeforeEach(func() {
		trackerDB = new(fakes.FakeTrackerDB)

		turbineServer = ghttp.NewServer()

		locker = new(fakes.FakeLocker)

		tracker = NewTracker(lagertest.NewTestLogger("test"), trackerDB, locker)

		lock = new(dbfakes.FakeLock)
		locker.AcquireWriteLockImmediatelyReturns(lock, nil)
	})

	AfterEach(func() {
		turbineServer.CloseClientConnections()
		turbineServer.Close()
	})

	Describe("TrackBuild", func() {
		var (
			build db.Build

			trackErr error
		)

		BeforeEach(func() {
			build = db.Build{
				ID:       1,
				Guid:     "some-guid",
				Endpoint: turbineServer.URL(),
			}
		})

		JustBeforeEach(func() {
			trackErr = tracker.TrackBuild(build)
		})

		Describe("locking", func() {
			BeforeEach(func() {
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
						},
					),
				)
			})

			It("grabs a lock before starting tracking, releases after", func() {
				Ω(locker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))

				lockedBuild := locker.AcquireWriteLockImmediatelyArgsForCall(0)
				Ω(lockedBuild).Should(Equal([]db.NamedLock{db.BuildTrackingLock(build.Guid)}))
				Ω(lock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when it can't grab the lock", func() {
				BeforeEach(func() {
					locker.AcquireWriteLockImmediatelyReturns(nil, errors.New("no lock for you"))
				})

				It("returns immediately", func() {
					Ω(locker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))

					lockedBuild := locker.AcquireWriteLockImmediatelyArgsForCall(0)
					Ω(lockedBuild).Should(Equal([]db.NamedLock{db.BuildTrackingLock(build.Guid)}))
					Ω(lock.ReleaseCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when the build has no endpoint", func() {
			BeforeEach(func() {
				build.Endpoint = ""
			})

			It("does not return an error", func() {
				Ω(trackErr).ShouldNot(HaveOccurred())
			})

			It("sets the build's status to errored", func() {
				// TODO some way of messaging this?

				Ω(trackerDB.SaveBuildStatusCallCount()).Should(Equal(1))

				buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
				Ω(buildID).Should(Equal(1))
				Ω(status).Should(Equal(db.StatusErrored))
			})
		})

		Context("when the build has no guid", func() {
			BeforeEach(func() {
				build.Guid = ""
			})

			It("does not return an error", func() {
				Ω(trackErr).ShouldNot(HaveOccurred())
			})

			It("sets the build's status to errored", func() {
				// TODO some way of messaging this?

				Ω(trackerDB.SaveBuildStatusCallCount()).Should(Equal(1))

				buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
				Ω(buildID).Should(Equal(1))
				Ω(status).Should(Equal(db.StatusErrored))
			})
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
					Eventually(trackerDB.SaveBuildEventCallCount).Should(BeNumerically(">=", count))

					event := events[count-1]

					payload, err := json.Marshal(event)
					Ω(err).ShouldNot(HaveOccurred())

					buildID, buildEvent := trackerDB.SaveBuildEventArgsForCall(count - 1)
					Ω(buildID).Should(Equal(1))
					Ω(buildEvent).Should(Equal(db.BuildEvent{
						ID:      count - 1,
						Type:    string(event.EventType()),
						Payload: string(payload),
					}))
				})

				Context("when saving the event fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						num := 0

						trackerDB.SaveBuildEventStub = func(int, db.BuildEvent) error {
							num++

							if num == count {
								return disaster
							}

							return nil
						}
					})

					It("returns an error", func() {
						Ω(trackErr).Should(Equal(disaster))
					})
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
									Status: turbine.StatusStarted,
									Time:   1234,
								})
							})

							itSavesTheEvent(2)

							It("saves the build's status", func() {
								Eventually(trackerDB.SaveBuildStatusCallCount).Should(Equal(1))

								buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
								Ω(buildID).Should(Equal(1))
								Ω(status).Should(Equal(db.StatusStarted))
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
									Status: turbine.StatusSucceeded,
									Time:   1234,
								})
							})

							itSavesTheEvent(2)

							It("saves the build's status", func() {
								Eventually(trackerDB.SaveBuildStatusCallCount).Should(Equal(1))

								buildID, status := trackerDB.SaveBuildStatusArgsForCall(0)
								Ω(buildID).Should(Equal(1))
								Ω(status).Should(Equal(db.StatusSucceeded))
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
								Input: turbine.Input{
									Name:     "some-input-name",
									Resource: "some-input-resource",
									Type:     "some-type",
									Source:   turbine.Source{"input-source": "some-source"},
									Version:  turbine.Version{"version": "input-version"},
									Metadata: []turbine.MetadataField{
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
								Ω(input).Should(Equal(db.BuildInput{
									Name: "some-input-name",
									VersionedResource: db.VersionedResource{
										Resource: "some-input-resource",
										Type:     "some-type",
										Source:   db.Source{"input-source": "some-source"},
										Version:  db.Version{"version": "input-version"},
										Metadata: []db.MetadataField{
											{Name: "input-meta", Value: "some-value"},
										},
									},
								}))
							})

							Context("and a successful status event appears", func() {
								BeforeEach(func() {
									events = append(events, event.Status{
										Status: turbine.StatusSucceeded,
									})
								})

								itSavesTheEvent(3)

								It("saves the build's input as an implicit output", func() {
									Eventually(trackerDB.SaveBuildOutputCallCount).Should(Equal(1))

									id, output := trackerDB.SaveBuildOutputArgsForCall(0)
									Ω(id).Should(Equal(1))
									Ω(output).Should(Equal(db.VersionedResource{
										Resource: "some-input-resource",
										Type:     "some-type",
										Source:   db.Source{"input-source": "some-source"},
										Version:  db.Version{"version": "input-version"},
										Metadata: []db.MetadataField{
											{Name: "input-meta", Value: "some-value"},
										},
									}))
								})
							})

							Context("and an output event appears for the same input resource", func() {
								BeforeEach(func() {
									events = append(events, event.Output{
										Output: turbine.Output{
											Name:    "some-input-resource", // TODO rename Output.Name to Output.Resource
											Type:    "some-type",
											Source:  turbine.Source{"input-source": "some-source"},
											Version: turbine.Version{"version": "explicit-input-version"},
											Metadata: []turbine.MetadataField{
												{Name: "input-meta", Value: "some-value"},
											},
										},
									})
								})

								itSavesTheEvent(3)

								Context("and a successful status event appears", func() {
									BeforeEach(func() {
										events = append(events, event.Status{
											Status: turbine.StatusSucceeded,
										})
									})

									itSavesTheEvent(4)

									It("saves the explicit output instead of the implicit one", func() {
										Eventually(trackerDB.SaveBuildOutputCallCount).Should(Equal(1))

										id, output := trackerDB.SaveBuildOutputArgsForCall(0)
										Ω(id).Should(Equal(1))
										Ω(output).Should(Equal(db.VersionedResource{
											Resource: "some-input-resource",
											Type:     "some-type",
											Source:   db.Source{"input-source": "some-source"},
											Version:  db.Version{"version": "explicit-input-version"},
											Metadata: []db.MetadataField{
												{Name: "input-meta", Value: "some-value"},
											},
										}))
									})
								})
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
								Output: turbine.Output{
									Name:    "some-output-name",
									Type:    "some-type",
									Source:  turbine.Source{"output-source": "some-source"},
									Version: turbine.Version{"version": "output-version"},
									Metadata: []turbine.MetadataField{
										{Name: "output-meta", Value: "some-value"},
									},
								},
							})
						})

						Context("and the build is for a job", func() {
							BeforeEach(func() {
								build.JobName = "lol"
							})

							It("does not save the output immediately", func() {
								Consistently(trackerDB.SaveBuildOutputCallCount).Should(BeZero())
							})

							Context("and a successful status event appears", func() {
								BeforeEach(func() {
									events = append(events, event.Status{
										Status: turbine.StatusSucceeded,
									})
								})

								It("saves the build's output", func() {
									Eventually(trackerDB.SaveBuildOutputCallCount).Should(Equal(1))

									id, output := trackerDB.SaveBuildOutputArgsForCall(0)
									Ω(id).Should(Equal(1))
									Ω(output).Should(Equal(db.VersionedResource{
										Resource: "some-output-name",
										Type:     "some-type",
										Source:   db.Source{"output-source": "some-source"},
										Version:  db.Version{"version": "output-version"},
										Metadata: []db.MetadataField{
											{Name: "output-meta", Value: "some-value"},
										},
									}))
								})
							})

							Context("and an errored status event appears", func() {
								BeforeEach(func() {
									events = append(events, event.Status{
										Status: turbine.StatusErrored,
									})
								})

								It("does not save the build's output", func() {
									Consistently(trackerDB.SaveBuildOutputCallCount).Should(BeZero())
								})
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

					Context("and an input event appears, with no resource present", func() {
						BeforeEach(func() {
							events = append(events, event.Input{
								Input: turbine.Input{
									Name:    "some-input-name",
									Type:    "some-type",
									Source:  turbine.Source{"input-source": "some-source"},
									Version: turbine.Version{"version": "input-version"},
									Metadata: []turbine.MetadataField{
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

							It("saves the build's input by its name, for backwards-compatibility", func() {
								Eventually(trackerDB.SaveBuildInputCallCount).Should(Equal(1))

								id, input := trackerDB.SaveBuildInputArgsForCall(0)
								Ω(id).Should(Equal(1))
								Ω(input).Should(Equal(db.BuildInput{
									Name: "some-input-name",
									VersionedResource: db.VersionedResource{
										Resource: "some-input-name",
										Type:     "some-type",
										Source:   db.Source{"input-source": "some-source"},
										Version:  db.Version{"version": "input-version"},
										Metadata: []db.MetadataField{
											{Name: "input-meta", Value: "some-value"},
										},
									},
								}))
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
						event.Status{Status: turbine.StatusSucceeded},
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
					go tracker.TrackBuild(build)
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
				Ω(status).Should(Equal(db.StatusErrored))
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
