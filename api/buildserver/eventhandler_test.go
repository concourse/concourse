package buildserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/buildserver/fakes"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeEvent struct {
	Value string `json:"value"`
}

func (e fakeEvent) EventType() atc.EventType { return "fake" }
func (fakeEvent) Version() atc.EventVersion  { return "42.0" }

var _ = Describe("Handler", func() {
	var (
		buildsDB *fakes.FakeBuildsDB

		server *httptest.Server
		client *http.Client
	)

	BeforeEach(func() {
		buildsDB = new(fakes.FakeBuildsDB)

		server = httptest.NewServer(NewEventHandler(buildsDB, 128))

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
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when subscribing to the build succeeds", func() {
			var fakeEventSource *dbfakes.FakeEventSource

			BeforeEach(func() {
				returnedEvents := []atc.Event{
					fakeEvent{"e1"},
					fakeEvent{"e2"},
					fakeEvent{"e3"},
				}

				fakeEventSource = new(dbfakes.FakeEventSource)

				buildsDB.GetBuildEventsStub = func(buildID int, from uint) (db.EventSource, error) {
					Expect(buildID).To(Equal(128))

					fakeEventSource.NextStub = func() (atc.Event, error) {
						defer GinkgoRecover()

						Expect(fakeEventSource.CloseCallCount()).To(Equal(0))

						if from >= uint(len(returnedEvents)) {
							return nil, db.ErrEndOfBuildEventStream
						}

						from++

						return returnedEvents[from-1], nil
					}

					return fakeEventSource, nil
				}
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns Content-Type as text/event-stream", func() {
				Expect(response.Header.Get("Content-Type")).To(Equal("text/event-stream; charset=utf-8"))
				Expect(response.Header.Get("Cache-Control")).To(Equal("no-cache, no-store, must-revalidate"))
				Expect(response.Header.Get("Connection")).To(Equal("keep-alive"))
			})

			It("returns the protocol version as X-ATC-Stream-Version", func() {
				Expect(response.Header.Get("X-ATC-Stream-Version")).To(Equal("2.0"))
			})

			It("emits them, followed by an end event", func() {
				reader := sse.NewReadCloser(response.Body)

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "0",
					Name: "event",
					Data: []byte(`{"data":{"value":"e1"},"event":"fake","version":"42.0"}`),
				}))

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "1",
					Name: "event",
					Data: []byte(`{"data":{"value":"e2"},"event":"fake","version":"42.0"}`),
				}))

				Expect(reader.Next()).To(Equal(sse.Event{
					ID:   "2",
					Name: "event",
					Data: []byte(`{"data":{"value":"e3"},"event":"fake","version":"42.0"}`),
				}))

				Expect(reader.Next()).To(Equal(sse.Event{
					Name: "end",
					Data: []byte{},
				}))

			})

			It("closes the event source", func() {
				Eventually(fakeEventSource.CloseCallCount).Should(Equal(1))
			})

			Context("when the Last-Event-ID header is given", func() {
				BeforeEach(func() {
					request.Header.Set("Last-Event-ID", "1")
				})

				It("starts subscribing from after the id", func() {
					_, from := buildsDB.GetBuildEventsArgsForCall(0)
					Expect(from).To(Equal(uint(2)))
				})
			})
		})

		Context("when subscribing to it fails", func() {
			BeforeEach(func() {
				buildsDB.GetBuildEventsReturns(nil, errors.New("nope"))
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
