package event_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/fakes"
	. "github.com/concourse/atc/event"
	"github.com/concourse/atc/event/fakes"
	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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

		server = httptest.NewServer(NewHandler(buildsDB, 128, fakeEngine, nil))

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

		weirdCensor := func(ev event.Event) event.Event {
			switch e := ev.(type) {
			case event.Version:
				return event.Version("1." + e)
			case event.Start:
				e.Time *= 2
				return e
			}

			return ev
		}

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
						returnedEvents := []event.Event{
							event.Version("1.0"),
							event.Start{Time: 1},
							event.End{},
						}

						fakeEventSource = new(enginefakes.FakeEventSource)

						fakeBuild.SubscribeStub = func(from uint) (engine.EventSource, error) {
							fakeEventSource.NextStub = func() (event.Event, error) {
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

					It("emits them", func() {
						reader := sse.NewReader(response.Body)

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "0",
							Name: "version",
							Data: []byte(`"1.0"`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "1",
							Name: "start",
							Data: []byte(`{"time":1}`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "2",
							Name: "end",
							Data: []byte(`{}`),
						}))
					})

					It("closes the event source", func() {
						Eventually(fakeEventSource.CloseCallCount).Should(Equal(1))
					})

					Context("when a censor is provided", func() {
						BeforeEach(func() {
							server.Config.Handler = NewHandler(buildsDB, 128, fakeEngine, weirdCensor)
						})

						It("filters the events through it", func() {
							reader := sse.NewReader(response.Body)

							Ω(reader.Next()).Should(Equal(sse.Event{
								ID:   "0",
								Name: "version",
								Data: []byte(`"1.1.0"`),
							}))

							Ω(reader.Next()).Should(Equal(sse.Event{
								ID:   "1",
								Name: "start",
								Data: []byte(`{"time":2}`),
							}))

							Ω(reader.Next()).Should(Equal(sse.Event{
								ID:   "2",
								Name: "end",
								Data: []byte(`{}`),
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
						Type:    "version",
						Payload: `"1.0"`,
					},
					{
						Type:    "start",
						Payload: `{"time":1}`,
					},
					{
						Type:    "end",
						Payload: `{}`,
					},
				}, nil)
			})

			It("returns the build's events from the database", func() {
				reader := sse.NewReader(response.Body)

				Ω(reader.Next()).Should(Equal(sse.Event{
					ID:   "0",
					Name: "version",
					Data: []byte(`"1.0"`),
				}))

				Ω(reader.Next()).Should(Equal(sse.Event{
					ID:   "1",
					Name: "start",
					Data: []byte(`{"time":1}`),
				}))
			})

			Context("when a censor is provided", func() {
				BeforeEach(func() {
					server.Config.Handler = NewHandler(buildsDB, 128, fakeEngine, weirdCensor)
				})

				It("filters the events through it", func() {
					reader := sse.NewReader(response.Body)

					Ω(reader.Next()).Should(Equal(sse.Event{
						ID:   "0",
						Name: "version",
						Data: []byte(`"1.1.0"`),
					}))

					Ω(reader.Next()).Should(Equal(sse.Event{
						ID:   "1",
						Name: "start",
						Data: []byte(`{"time":2}`),
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
						Name: "end",
						Data: []byte(`{}`),
					}))
				})

				Context("but the id reaches the end", func() {
					BeforeEach(func() {
						request.Header.Set("Last-Event-ID", "2")
					})

					It("returns no events", func() {
						reader := sse.NewReader(response.Body)

						_, err := reader.Next()
						Ω(err).Should(Equal(io.EOF))
					})
				})
			})
		})
	})
})
