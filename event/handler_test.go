package event_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/event"
	"github.com/concourse/atc/event/fakes"
	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Handler", func() {
	var (
		buildsDB *fakes.FakeBuildsDB

		server *httptest.Server
		client *http.Client
	)

	BeforeEach(func() {
		buildsDB = new(fakes.FakeBuildsDB)
		handler := NewHandler(buildsDB, 128, nil)

		server = httptest.NewServer(handler)

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

		capsCensor := func(ev sse.Event) (sse.Event, error) {
			ev.Name = strings.ToUpper(ev.Name)
			ev.Data = bytes.ToUpper(ev.Data)
			return ev, nil
		}

		Context("when the build is started", func() {
			var (
				turbineEndpoint *ghttp.Server

				returnedEvents []event.Event
			)

			BeforeEach(func() {
				turbineEndpoint = ghttp.NewServer()

				returnedEvents = []event.Event{}

				turbineEndpoint.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/builds/some-guid/events"),
						func(w http.ResponseWriter, r *http.Request) {
							flusher := w.(http.Flusher)

							w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
							w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Add("Connection", "keep-alive")

							w.WriteHeader(http.StatusOK)

							flusher.Flush()

							start := 0
							if r.Header.Get("Last-Event-ID") != "" {
								lastEvent, err := strconv.Atoi(r.Header.Get("Last-Event-ID"))
								Ω(err).ShouldNot(HaveOccurred())

								start = lastEvent + 1
							}

							for idx, e := range returnedEvents[start:] {
								payload, err := json.Marshal(e)
								Ω(err).ShouldNot(HaveOccurred())

								event := sse.Event{
									ID:   fmt.Sprintf("%d", idx+start),
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

				buildsDB.GetBuildReturns(db.Build{
					ID:       128,
					Guid:     "some-guid",
					Endpoint: turbineEndpoint.URL(),
					Status:   db.StatusStarted,
				}, nil)
			})

			Context("when the turbine returns events", func() {
				BeforeEach(func() {
					returnedEvents = append(
						returnedEvents,
						event.Version("1.0"),
						event.Start{Time: 1},
						event.End{},
					)
				})

				AfterEach(func() {
					turbineEndpoint.Close()
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("returns Content-Type as text/event-stream", func() {
					Ω(response.Header.Get("Content-Type")).Should(Equal("text/event-stream; charset=utf-8"))
					Ω(response.Header.Get("Cache-Control")).Should(Equal("no-cache, no-store, must-revalidate"))
					Ω(response.Header.Get("Connection")).Should(Equal("keep-alive"))
				})

				It("proxies them", func() {
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

				Context("when a censor is provided", func() {
					BeforeEach(func() {
						server.Config.Handler = NewHandler(buildsDB, 128, capsCensor)
					})

					It("filters the events through it", func() {
						reader := sse.NewReader(response.Body)

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "0",
							Name: "VERSION",
							Data: []byte(`"1.0"`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "1",
							Name: "START",
							Data: []byte(`{"TIME":1}`),
						}))

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "2",
							Name: "END",
							Data: []byte(`{}`),
						}))
					})
				})

				Context("when the Last-Event-ID header is given", func() {
					BeforeEach(func() {
						request.Header.Set("Last-Event-ID", "1")
					})

					It("forwards it to the turbine", func() {
						reader := sse.NewReader(response.Body)

						Ω(reader.Next()).Should(Equal(sse.Event{
							ID:   "2",
							Name: "end",
							Data: []byte(`{}`),
						}))
					})
				})
			})

			Context("when the turbine is unavailable", func() {
				BeforeEach(func() {
					turbineEndpoint.Close()
				})

				It("returns 503", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusServiceUnavailable))
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
					server.Config.Handler = NewHandler(buildsDB, 128, capsCensor)
				})

				It("filters the events through it", func() {
					reader := sse.NewReader(response.Body)

					Ω(reader.Next()).Should(Equal(sse.Event{
						ID:   "0",
						Name: "VERSION",
						Data: []byte(`"1.0"`),
					}))

					Ω(reader.Next()).Should(Equal(sse.Event{
						ID:   "1",
						Name: "START",
						Data: []byte(`{"TIME":1}`),
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
