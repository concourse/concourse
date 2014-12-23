package buildserver_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/buildserver/fakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/fakes"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeEvent struct {
	Value string `json:"value"`
}

func (e fakeEvent) EventType() atc.EventType { return "fake" }
func (fakeEvent) Version() atc.EventVersion  { return "42.0" }
func (e fakeEvent) Censored() atc.Event {
	e.Value = "censored " + e.Value
	return e
}

var _ = Describe("Handler", func() {
	var (
		buildsDB   *fakes.FakeBuildsDB
		fakeEngine *enginefakes.FakeEngine

		server *httptest.Server
		client *http.Client
	)

	BeforeEach(func() {
		buildsDB = new(fakes.FakeBuildsDB)
		fakeEngine = new(enginefakes.FakeEngine)

		server = httptest.NewServer(NewEventHandler(buildsDB, 128, fakeEngine, false))

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Describe("GET", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("GET", server.URL, nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when the build is started", func() {
			BeforeEach(func() {
				buildsDB.GetBuildReturns(db.Build{
					ID:             128,
					Engine:         "some-engine",
					EngineMetadata: "some-metadata",
					Status:         db.StatusStarted,
				}, nil)
			})

			Context("when the engine returns a build", func() {
				var fakeBuild *enginefakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(enginefakes.FakeBuild)
					fakeEngine.LookupBuildReturns(fakeBuild, nil)
				})

				Context("and subscribing to it succeeds", func() {
					var fakeEventSource *enginefakes.FakeEventSource

					BeforeEach(func() {
						returnedEvents := []atc.Event{
							fakeEvent{"e1"},
							fakeEvent{"e2"},
							fakeEvent{"e3"},
						}

						fakeEventSource = new(enginefakes.FakeEventSource)

						fakeBuild.SubscribeStub = func(from uint) (engine.EventSource, error) {
							fakeEventSource.NextStub = func() (atc.Event, error) {
								defer GinkgoRecover()

								Ω(fakeEventSource.CloseCallCount()).Should(Equal(0))

								if from >= uint(len(returnedEvents)) {
									return nil, engine.ErrEndOfStream
								}

								from++

								return returnedEvents[from-1], nil
							}

							return fakeEventSource, nil
						}
					})

					It("returns 200", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusOK))
					})

					It("returns Content-Type as text/event-stream", func() {
						Ω(response.Header.Get("Content-Type")).Should(Equal("text/event-stream; charset=utf-8"))
						Ω(response.Header.Get("Cache-Control")).Should(Equal("no-cache, no-store, must-revalidate"))
						Ω(response.Header.Get("Connection")).Should(Equal("keep-alive"))
					})

					It("returns the protocol version as X-ATC-Stream-Version", func() {
						Ω(response.Header.Get("X-ATC-Stream-Version")).Should(Equal("2.0"))
					})

					It("emits them, followed by an end event", func() {
						reader := sse.NewReader(response.Body)

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "0",
							Name: "event",
							Data: []byte(`{"data":{"value":"e1"},"event":"fake","version":"42.0"}`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "1",
							Name: "event",
							Data: []byte(`{"data":{"value":"e2"},"event":"fake","version":"42.0"}`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "2",
							Name: "event",
							Data: []byte(`{"data":{"value":"e3"},"event":"fake","version":"42.0"}`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							Name: "end",
							Data: []byte{},
						}))
					})

					It("closes the event source", func() {
						Eventually(fakeEventSource.CloseCallCount).Should(Equal(1))
					})

					Context("when told to censor", func() {
						BeforeEach(func() {
							server.Config.Handler = NewEventHandler(buildsDB, 128, fakeEngine, true)
						})

						It("filters the events through it", func() {
							reader := sse.NewReader(response.Body)

							Ω(reader.Next()).Should(Equal(sse.Event{
								ID:   "0",
								Name: "event",
								Data: []byte(`{"data":{"value":"censored e1"},"event":"fake","version":"42.0"}`),
							}))

							Ω(reader.Next()).Should(Equal(sse.Event{
								ID:   "1",
								Name: "event",
								Data: []byte(`{"data":{"value":"censored e2"},"event":"fake","version":"42.0"}`),
							}))

							Ω(reader.Next()).Should(Equal(sse.Event{
								ID:   "2",
								Name: "event",
								Data: []byte(`{"data":{"value":"censored e3"},"event":"fake","version":"42.0"}`),
							}))

							Ω(reader.Next()).Should(Equal(sse.Event{
								Name: "end",
								Data: []byte{},
							}))
						})
					})

					Context("when the Last-Event-ID header is given", func() {
						BeforeEach(func() {
							request.Header.Set("Last-Event-ID", "1")
						})

						It("starts subscribing from after the id", func() {
							Ω(fakeBuild.SubscribeArgsForCall(0)).Should(Equal(uint(2)))
						})
					})
				})

				Context("when subscribing to it fails", func() {
					BeforeEach(func() {
						fakeBuild.SubscribeReturns(nil, errors.New("nope"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when looking up the build fails", func() {
				BeforeEach(func() {
					fakeEngine.LookupBuildReturns(nil, errors.New("nope"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when the build is completed", func() {
			BeforeEach(func() {
				buildsDB.GetBuildReturns(db.Build{
					ID:     128,
					Status: db.StatusSucceeded,
				}, nil)

				buildsDB.GetBuildEventsReturns([]db.BuildEvent{
					{
						Type:    "initialize",
						Payload: `{"config":{"params":{"SECRET":"lol"},"run":{"path":"ls"}}}`,
						Version: "1.0",
					},
					{
						Type:    "start",
						Payload: `{"time":1}`,
						Version: "1.0",
					},
					{
						Type:    "status",
						Payload: `{"status":"succeeded","time":123}`,
						Version: "1.0",
					},
				}, nil)
			})

			It("returns the build's events from the database, followed by an end event", func() {
				reader := sse.NewReader(response.Body)

				Ω(reader.Next()).Should(Equal(sse.Event{
					ID:   "0",
					Name: "event",
					Data: []byte(`{"data":{"config":{"params":{"SECRET":"lol"},"run":{"path":"ls"}}},"event":"initialize","version":"1.0"}`),
				}))

				Ω(reader.Next()).Should(Equal(sse.Event{
					ID:   "1",
					Name: "event",
					Data: []byte(`{"data":{"time":1},"event":"start","version":"1.0"}`),
				}))

				Ω(reader.Next()).Should(Equal(sse.Event{
					ID:   "2",
					Name: "event",
					Data: []byte(`{"data":{"status":"succeeded","time":123},"event":"status","version":"1.0"}`),
				}))

				Ω(reader.Next()).Should(Equal(sse.Event{
					Name: "end",
					Data: []byte{},
				}))
			})

			Context("when told to censor", func() {
				BeforeEach(func() {
					server.Config.Handler = NewEventHandler(buildsDB, 128, fakeEngine, true)
				})

				It("censors the events", func() {
					reader := sse.NewReader(response.Body)

					Ω(reader.Next()).Should(Equal(sse.Event{
						ID:   "0",
						Name: "event",
						Data: []byte(`{"data":{"config":{"run":{"path":"ls"}}},"event":"initialize","version":"1.0"}`),
					}))
				})
			})

			Context("when the Last-Event-ID header is given", func() {
				BeforeEach(func() {
					request.Header.Set("Last-Event-ID", "1")
				})

				It("offsets the events from the database", func() {
					reader := sse.NewReader(response.Body)

					Ω(reader.Next()).Should(Equal(sse.Event{
						ID:   "2",
						Name: "event",
						Data: []byte(`{"data":{"status":"succeeded","time":123},"event":"status","version":"1.0"}`),
					}))
				})

				Context("but the id reaches the end", func() {
					BeforeEach(func() {
						request.Header.Set("Last-Event-ID", "3")
					})

					It("returns an end event and ends the stream", func() {
						reader := sse.NewReader(response.Body)

						Ω(reader.Next()).Should(Equal(sse.Event{
							Name: "end",
							Data: []byte{},
						}))

						_, err := reader.Next()
						Ω(err).Should(Equal(io.EOF))
					})
				})
			})
		})
	})
})
