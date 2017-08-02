package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/go-concourse/concourse/eventstream"
	. "github.com/concourse/go-concourse/concourse/internal"
	"github.com/google/jsonapi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

var _ = Describe("ATC Connection", func() {
	var (
		atcServer *ghttp.Server

		connection Connection

		tracing bool
	)

	BeforeEach(func() {
		atcServer = ghttp.NewServer()

		connection = NewConnection(atcServer.URL(), nil, tracing)
	})

	Describe("#Send", func() {
		It("is robust to trailing slash in the target", func() {
			badConnection := NewConnection(atcServer.URL()+"/", nil, tracing)
			expectedURL := "/api/v1/builds/foo"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}),
				),
			)
			var build atc.Build
			err := badConnection.Send(Request{
				RequestName: atc.GetBuild,
				Params:      rata.Params{"build_id": "foo"},
			}, &Response{
				Result: &build,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
		})

		It("can ignore responses", func() {
			badConnection := NewConnection(atcServer.URL(), nil, tracing)

			expectedURL := "/api/v1/builds/foo"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Build{}),
				),
			)

			err := badConnection.Send(Request{
				RequestName: atc.GetBuild,
				Params:      rata.Params{"build_id": "foo"},
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
		})

		It("uses the given http connection", func() {
			basicAuthConnection := NewConnection(
				atcServer.URL(),
				&http.Client{
					Transport: BasicAuthTransport{
						Username: "some username",
						Password: "some password",
					},
				},
				tracing,
			)

			expectedURL := "/api/v1/builds/foo"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.VerifyBasicAuth("some username", "some password"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}),
				),
			)
			var build atc.Build
			err := basicAuthConnection.Send(Request{
				RequestName: atc.GetBuild,
				Params:      rata.Params{"build_id": "foo"},
			}, &Response{
				Result: &build,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
		})

		It("makes a request to the given route", func() {
			expectedURL := "/api/v1/builds/foo"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}),
				),
			)
			var build atc.Build
			err := connection.Send(Request{
				RequestName: atc.GetBuild,
				Params:      rata.Params{"build_id": "foo"},
			}, &Response{
				Result: &build,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
		})

		It("makes a request with the given parameters to the given route", func() {
			expectedURL := "/api/v1/containers"
			expectedResponse := []atc.Container{
				{
					ID:           "first-container",
					PipelineName: "my-special-pipeline",
					ResourceName: "bob",
					BuildID:      1,
					WorkerName:   "abc",
				},
				{
					ID:           "second-container",
					PipelineName: "my-special-pipeline",
					JobName:      "my-special-job",
					Type:         "task",
					StepName:     "alice",
					BuildID:      1,
					WorkerName:   "def",
				},
			}
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, "pipeline_name=my-special-pipeline"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedResponse),
				),
			)
			var containers []atc.Container
			err := connection.Send(Request{
				RequestName: atc.ListContainers,
				Query:       url.Values{"pipeline_name": {"my-special-pipeline"}},
			}, &Response{
				Result: &containers,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
			Expect(containers).To(Equal(expectedResponse))
		})

		Context("when trying to retrieve the body of the request", func() {
			var expectedBytes []byte

			BeforeEach(func() {
				expectedURL := "/api/v1/cli"
				expectedBytes = []byte{0, 1, 0}

				atcServer.RouteToHandler("GET", expectedURL,
					ghttp.CombineHandlers(
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)
							w.Write(expectedBytes)
						},
					),
				)
			})

			It("does not close the request body, and returns the body back through the response object", func() {
				response := Response{}
				err := connection.Send(Request{
					RequestName:        atc.DownloadCLI,
					ReturnResponseBody: true,
				},
					&response,
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(atcServer.ReceivedRequests())).To(Equal(1))

				respBody, ok := response.Result.(io.ReadCloser)
				Expect(ok).To(BeTrue())
				Expect(respBody.Close()).NotTo(HaveOccurred())
			})
		})

		Context("Request Headers", func() {
			BeforeEach(func() {
				atcServer = ghttp.NewServer()

				connection = NewConnection(atcServer.URL(), nil, tracing)

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/builds/foo"),
						ghttp.VerifyHeaderKV("Accept-Encoding", "application/banana"),
						ghttp.VerifyHeaderKV("foo", "bar", "baz"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}),
					),
				)
			})

			It("sets the header and it's values on the request", func() {
				err := connection.Send(Request{
					RequestName: atc.GetBuild,
					Params:      rata.Params{"build_id": "foo"},
					Header: http.Header{
						"Accept-Encoding": {"application/banana"},
						"Foo":             {"bar", "baz"},
					},
				}, &Response{
					Result: &atc.Build{},
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("Response Headers", func() {
			BeforeEach(func() {
				atcServer = ghttp.NewServer()

				connection = NewConnection(atcServer.URL(), nil, tracing)

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/builds/foo"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
				)
			})

			It("returns the response headers in Headers", func() {
				responseHeaders := http.Header{}

				err := connection.Send(Request{
					RequestName:        atc.GetBuild,
					Params:             rata.Params{"build_id": "foo"},
					ReturnResponseBody: true,
				}, &Response{
					Result:  &atc.Build{},
					Headers: &responseHeaders,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(responseHeaders.Get(atc.ConfigVersionHeader)).To(Equal("42"))
			})
		})

		Describe("Different status codes", func() {
			Describe("204 no content", func() {
				BeforeEach(func() {
					atcServer = ghttp.NewServer()

					connection = NewConnection(atcServer.URL(), nil, tracing)

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)
				})

				It("sets the username and password if given", func() {
					err := connection.Send(Request{
						RequestName: atc.DeletePipeline,
						Params: rata.Params{
							"pipeline_name": "foo",
							"team_name":     atc.DefaultTeamName,
						},
					}, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Describe("Non-2XX response", func() {
				BeforeEach(func() {
					atcServer = ghttp.NewServer()

					connection = NewConnection(atcServer.URL(), nil, tracing)

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
							ghttp.RespondWith(http.StatusInternalServerError, "problem"),
						),
					)
				})

				It("returns back UnexpectedResponseError", func() {
					err := connection.Send(Request{
						RequestName: atc.DeletePipeline,
						Params: rata.Params{
							"pipeline_name": "foo",
							"team_name":     atc.DefaultTeamName,
						},
					}, nil)

					Expect(err).To(HaveOccurred())
					ure, ok := err.(UnexpectedResponseError)
					Expect(ok).To(BeTrue())
					Expect(ure.StatusCode).To(Equal(http.StatusInternalServerError))
					Expect(ure.Body).To(Equal("problem"))
				})
			})

			Describe("401 response", func() {
				BeforeEach(func() {
					atcServer = ghttp.NewServer()

					connection = NewConnection(atcServer.URL(), nil, tracing)

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
							ghttp.RespondWith(http.StatusUnauthorized, "problem"),
						),
					)
				})

				It("returns back ErrUnauthorized", func() {
					err := connection.Send(Request{
						RequestName: atc.DeletePipeline,
						Params: rata.Params{
							"pipeline_name": "foo",
							"team_name":     atc.DefaultTeamName,
						},
					}, nil)

					Expect(err).To(Equal(ErrUnauthorized))
				})
			})

			Describe("403 response", func() {
				BeforeEach(func() {
					atcServer = ghttp.NewServer()

					connection = NewConnection(atcServer.URL(), nil, tracing)

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
							ghttp.RespondWith(http.StatusForbidden, "problem"),
						),
					)
				})

				It("returns back ErrForbidden", func() {
					err := connection.Send(Request{
						RequestName: atc.DeletePipeline,
						Params: rata.Params{
							"pipeline_name": "foo",
							"team_name":     atc.DefaultTeamName,
						},
					}, nil)

					Expect(err).To(Equal(ErrForbidden))
				})
			})

			Describe("404 response", func() {
				Context("when the response does not contain JSONAPI errors", func() {
					BeforeEach(func() {
						atcServer = ghttp.NewServer()

						connection = NewConnection(atcServer.URL(), nil, tracing)

						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
								ghttp.RespondWith(http.StatusNotFound, "problem"),
							),
						)
					})

					It("returns back ResourceNotFoundError", func() {
						err := connection.Send(Request{
							RequestName: atc.DeletePipeline,
							Params: rata.Params{
								"pipeline_name": "foo",
								"team_name":     atc.DefaultTeamName,
							},
						}, nil)

						Expect(err).To(HaveOccurred())
						_, ok := err.(ResourceNotFoundError)
						Expect(ok).To(BeTrue())
						Expect(err.Error()).To(Equal("resource not found"))
					})
				})

				Context("when the response contains JSONAPI errors", func() {
					BeforeEach(func() {
						atcServer = ghttp.NewServer()

						connection = NewConnection(atcServer.URL(), nil, tracing)

						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
								ghttp.RespondWithJSONEncoded(http.StatusNotFound, jsonapi.ErrorsPayload{[]*jsonapi.ErrorObject{
									{Detail: "One error message's detail."},
									{Detail: "Some other error message detail."},
								}}),
							),
						)
					})

					It("returns back a ResourceNotFoundError with the given error details", func() {
						err := connection.Send(Request{
							RequestName: atc.DeletePipeline,
							Params: rata.Params{
								"pipeline_name": "foo",
								"team_name":     atc.DefaultTeamName,
							},
						}, nil)

						Expect(err).To(HaveOccurred())
						_, ok := err.(ResourceNotFoundError)
						Expect(ok).To(BeTrue())
						Expect(err.Error()).To(Equal("One error message's detail. Some other error message detail."))
					})

				})
			})
		})

		Describe("Request Body", func() {
			var plan atc.Plan

			BeforeEach(func() {
				plan = atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Aggregate: &atc.AggregatePlan{},
						},
						Next: atc.Plan{
							ID: "some-guid",
							Task: &atc.TaskPlan{
								Name:       "one-off",
								Privileged: true,
								Config:     &atc.TaskConfig{},
							},
						},
					},
				}

				expectedURL := "/api/v1/builds"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL),
						ghttp.VerifyJSONRepresenting(plan),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Config{}),
					),
				)
			})

			It("sends it in the request body and sets the response's created flag to true", func() {
				buffer := &bytes.Buffer{}
				err := json.NewEncoder(buffer).Encode(plan)
				if err != nil {
					Fail(fmt.Sprintf("Unable to marshal plan: %s", err))
				}

				response := Response{
					Result: &atc.Config{},
				}
				err = connection.Send(Request{
					RequestName: atc.CreateBuild,
					Body:        buffer,
					Header: http.Header{
						"Content-Type": {"application/json"},
					},
				},
					&response,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Created).To(BeTrue())
				Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
			})
		})
	})

	Describe("#ConnectToEventStream", func() {
		buildID := "3"
		var streaming chan struct{}
		var eventsChan chan atc.Event

		BeforeEach(func() {
			streaming = make(chan struct{})
			eventsChan = make(chan atc.Event)

			eventsHandler := func() http.HandlerFunc {
				return ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/builds/%s/events", buildID)),
					func(w http.ResponseWriter, r *http.Request) {
						flusher := w.(http.Flusher)

						w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
						w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
						w.Header().Add("Connection", "keep-alive")

						w.WriteHeader(http.StatusOK)

						flusher.Flush()

						close(streaming)

						id := 0

						for e := range eventsChan {
							payload, err := json.Marshal(event.Message{Event: e})
							Expect(err).NotTo(HaveOccurred())

							event := sse.Event{
								ID:   fmt.Sprintf("%d", id),
								Name: "event",
								Data: payload,
							}

							err = event.Write(w)
							Expect(err).NotTo(HaveOccurred())

							flusher.Flush()

							id++
						}

						err := sse.Event{
							Name: "end",
						}.Write(w)
						Expect(err).NotTo(HaveOccurred())
					},
				)
			}

			atcServer.AppendHandlers(
				eventsHandler(),
			)
		})

		It("returns an EventSource that can stream events", func() {
			eventSource, err := connection.ConnectToEventStream(
				Request{
					RequestName: atc.BuildEvents,
					Params:      rata.Params{"build_id": buildID},
				})
			Expect(err).NotTo(HaveOccurred())

			events := eventstream.NewSSEEventStream(eventSource)

			Eventually(streaming).Should(BeClosed())

			eventsChan <- event.Log{Payload: "sup"}

			nextEvent, err := events.NextEvent()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextEvent).To(Equal(event.Log{Payload: "sup"}))

			close(eventsChan)

			_, err = events.NextEvent()
			Expect(err).To(MatchError(io.EOF))
		})
	})
})

type BasicAuthTransport struct {
	Username string
	Password string
}

func (bat BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(bat.Username, bat.Password)
	return http.DefaultTransport.RoundTrip(req)
}
