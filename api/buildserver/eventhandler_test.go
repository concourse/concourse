package buildserver_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/event"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func fakeEvent(payload string) event.Envelope {
	msg := json.RawMessage(payload)
	return event.Envelope{
		Data:    &msg,
		Event:   "fake",
		Version: "42.0",
	}
}

var _ = Describe("Handler", func() {
	var (
		build *dbfakes.FakeBuild

		server *httptest.Server
	)

	BeforeEach(func() {
		build = new(dbfakes.FakeBuild)

		server = httptest.NewServer(NewEventHandler(lagertest.NewTestLogger("test"), build))
	})

	Describe("GET", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("GET", server.URL, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when subscribing to the build succeeds", func() {
			var fakeEventSource *dbfakes.FakeEventSource
			var returnedEvents []event.Envelope

			BeforeEach(func() {
				returnedEvents = []event.Envelope{
					fakeEvent(`{"event":1}`),
					fakeEvent(`{"event":2}`),
					fakeEvent(`{"event":3}`),
				}

				fakeEventSource = new(dbfakes.FakeEventSource)

				build.EventsStub = func(from uint) (db.EventSource, error) {
					fakeEventSource.NextStub = func() (event.Envelope, error) {
						defer GinkgoRecover()

						Expect(fakeEventSource.CloseCallCount()).To(Equal(0))

						if from >= uint(len(returnedEvents)) {
							return event.Envelope{}, db.ErrEndOfBuildEventStream
						}

						from++

						return returnedEvents[from-1], nil
					}

					return fakeEventSource, nil
				}
			})

			AfterEach(func() {
				Eventually(fakeEventSource.CloseCallCount, 30*time.Second).Should(Equal(1))
			})

			JustBeforeEach(func() {
				var err error

				client := &http.Client{
					Transport: &http.Transport{},
				}
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets the events from the right build, starting at 0", func() {
				_ = response.Body.Close()
				Eventually(build.EventsCallCount).Should(Equal(1))
				actualFrom := build.EventsArgsForCall(0)
				Expect(actualFrom).To(BeZero())
			})

			It("returns 200", func() {
				_ = response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns Content-Type as text/event-stream", func() {
				_ = response.Body.Close()
				Expect(response.Header.Get("Content-Type")).To(Equal("text/event-stream; charset=utf-8"))
				Expect(response.Header.Get("Cache-Control")).To(Equal("no-cache, no-store, must-revalidate"))
				Expect(response.Header.Get("X-Accel-Buffering")).To(Equal("no"))
				Expect(response.Header.Get("Connection")).NotTo(Equal("keep-alive"))
			})

			It("returns the protocol version as X-ATC-Stream-Version", func() {
				_ = response.Body.Close()
				Expect(response.Header.Get("X-ATC-Stream-Version")).To(Equal("2.0"))
			})

			It("emits them, followed by an end event", func() {
				defer db.Close(response.Body)
				reader := sse.NewReadCloser(response.Body)

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "0",
					Name: "event",
					Data: []byte(`{"data":{"event":1},"event":"fake","version":"42.0"}`),
				}))

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "1",
					Name: "event",
					Data: []byte(`{"data":{"event":2},"event":"fake","version":"42.0"}`),
				}))

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "2",
					Name: "event",
					Data: []byte(`{"data":{"event":3},"event":"fake","version":"42.0"}`),
				}))

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "3",
					Name: "end",
					Data: []byte{},
				}))
			})

			Context("when the Last-Event-ID header is given", func() {
				BeforeEach(func() {
					request.Header.Set("Last-Event-ID", "1")
				})

				It("starts subscribing from after the id", func() {
					_ = response.Body.Close()
					Eventually(build.EventsCallCount).Should(Equal(1))
					actualFrom := build.EventsArgsForCall(0)
					Expect(actualFrom).To(Equal(uint(2)))
				})
			})
		})

		Context("when the eventsource returns an error", func() {
			var fakeEventSource *dbfakes.FakeEventSource
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("a coffee machine")

				fakeEventSource = new(dbfakes.FakeEventSource)

				from := 0
				fakeEventSource.NextStub = func() (event.Envelope, error) {
					defer GinkgoRecover()

					Expect(fakeEventSource.CloseCallCount()).To(Equal(0))

					from++

					if from == 1 {
						return fakeEvent(`{"event":1}`), nil
					} else {
						return event.Envelope{}, disaster
					}
				}

				build.EventsReturns(fakeEventSource, nil)
			})

			AfterEach(func() {
				Eventually(fakeEventSource.CloseCallCount, 30*time.Second).Should(Equal(1))
			})

			JustBeforeEach(func() {
				var err error

				client := &http.Client{
					Transport: &http.Transport{},
				}
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("just stops sending events", func() {
				reader := sse.NewReadCloser(response.Body)

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "0",
					Name: "event",
					Data: []byte(`{"data":{"event":1},"event":"fake","version":"42.0"}`),
				}))

				_, err := reader.Next()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(io.EOF))
			})
		})

		Context("when the event stream never ends", func() {
			var fakeEventSource *dbfakes.FakeEventSource
			BeforeEach(func() {
				fakeEventSource = new(dbfakes.FakeEventSource)
				fakeEventSource.NextReturns(fakeEvent(`{"event":1}`), nil)
				build.EventsReturns(fakeEventSource, nil)
			})

			JustBeforeEach(func() {
				var err error

				client := &http.Client{
					Transport: &http.Transport{},
				}
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when request accepts gzip", func() {
				BeforeEach(func() {
					request.Header.Set("Accept-Encoding", "gzip")
				})

				It("closes the event stream when connection is closed", func() {
					err := response.Body.Close()
					Expect(err).NotTo(HaveOccurred())
					Eventually(fakeEventSource.CloseCallCount, 30*time.Second).Should(Equal(1))
				})
			})
		})

		Context("when subscribing to it fails", func() {
			BeforeEach(func() {
				build.EventsReturns(nil, errors.New("nope"))
			})

			JustBeforeEach(func() {
				var err error

				client := &http.Client{
					Transport: &http.Transport{},
				}
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
