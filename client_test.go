package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Client", func() {
	var (
		api      string
		username string
		password string
		cert     string
		insecure bool
	)

	BeforeEach(func() {
		api = "f"
		username = ""
		password = ""
		cert = ""
		insecure = false
	})

	Describe("#NewClient", func() {
		It("returns back an ATC Client", func() {
			target := rc.NewTarget(api, username, password, cert, insecure)
			client, err := NewClient(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})

		It("errors when passed target props with an invalid url", func() {
			target := rc.NewTarget("", username, password, cert, insecure)
			_, err := NewClient(target)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API is blank"))
		})
	})

	Describe("#Send", func() {
		var (
			atcServer *ghttp.Server
			client    Client
		)

		BeforeEach(func() {
			var err error
			atcServer = ghttp.NewServer()
			client, err = NewClient(
				rc.NewTarget(atcServer.URL(), "", "", "", false),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			atcServer.Close()
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
			err := client.Send(Request{
				RequestName: atc.GetBuild,
				Params:      map[string]string{"build_id": "foo"},
			}, Response{
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
					Type:         "check",
					Name:         "bob",
					BuildID:      1,
					WorkerName:   "abc",
				},
				{
					ID:           "second-container",
					PipelineName: "my-special-pipeline",
					Type:         "check",
					Name:         "alice",
					BuildID:      1,
					WorkerName:   "def",
				},
			}
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, "type=check"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedResponse),
				),
			)
			var containers []atc.Container
			err := client.Send(Request{
				RequestName: atc.ListContainers,
				Queries:     map[string]string{"type": "check"},
			}, Response{
				Result: &containers,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
			Expect(containers).To(Equal(expectedResponse))
		})

		Context("Sending Headers", func() {
			Context("Basic Auth", func() {
				BeforeEach(func() {
					var err error
					atcServer = ghttp.NewServer()

					username = "foo"
					password = "bar"
					target := rc.NewTarget(atcServer.URL(), username, password, cert, insecure)
					client, err = NewClient(target)
					Expect(err).NotTo(HaveOccurred())

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/builds/foo"),
							ghttp.VerifyBasicAuth(username, password),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}),
						),
					)
				})

				It("sets the username and password if given", func() {
					err := client.Send(Request{
						RequestName: atc.GetBuild,
						Params:      map[string]string{"build_id": "foo"},
					}, Response{
						Result: &atc.Build{},
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("Any Header", func() {
				BeforeEach(func() {
					var err error
					atcServer = ghttp.NewServer()

					target := rc.NewTarget(atcServer.URL(), username, password, cert, insecure)
					client, err = NewClient(target)
					Expect(err).NotTo(HaveOccurred())

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
					err := client.Send(Request{
						RequestName: atc.GetBuild,
						Params:      map[string]string{"build_id": "foo"},
						Headers: map[string][]string{
							"accept-encoding": {"application/banana"},
							"foo":             {"bar", "baz"},
						},
					}, Response{
						Result: &atc.Build{},
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Describe("Response Headers", func() {
			BeforeEach(func() {
				var err error
				atcServer = ghttp.NewServer()

				target := rc.NewTarget(atcServer.URL(), username, password, cert, insecure)
				client, err = NewClient(target)
				Expect(err).NotTo(HaveOccurred())

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/builds/foo"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
				)

			})

			It("returns the response headers in Headers", func() {
				responseHeaders := map[string][]string{}

				err := client.Send(Request{
					RequestName: atc.GetBuild,
					Params:      map[string]string{"build_id": "foo"},
				}, Response{
					Result:  &atc.Build{},
					Headers: &responseHeaders,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(responseHeaders[atc.ConfigVersionHeader]).To(Equal([]string{"42"}))
			})
		})

		Describe("Different status codes", func() {
			Describe("204 no content", func() {
				BeforeEach(func() {
					var err error
					atcServer = ghttp.NewServer()

					target := rc.NewTarget(atcServer.URL(), username, password, cert, insecure)
					client, err = NewClient(target)
					Expect(err).NotTo(HaveOccurred())

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/foo"),
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)

				})

				It("sets the username and password if given", func() {
					err := client.Send(Request{
						RequestName: atc.DeletePipeline,
						Params:      map[string]string{"pipeline_name": "foo"},
					},
						Response{},
					)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Describe("Non-2XX response", func() {
				BeforeEach(func() {
					var err error
					atcServer = ghttp.NewServer()

					target := rc.NewTarget(atcServer.URL(), username, password, cert, insecure)
					client, err = NewClient(target)
					Expect(err).NotTo(HaveOccurred())

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/foo"),
							ghttp.RespondWith(http.StatusInternalServerError, "problem"),
						),
					)
				})

				It("returns back UnexpectedResponseError", func() {
					err := client.Send(Request{
						RequestName: atc.DeletePipeline,
						Params:      map[string]string{"pipeline_name": "foo"},
					},
						Response{},
					)
					Expect(err).To(HaveOccurred())
					ure, ok := err.(UnexpectedResponseError)
					Expect(ok).To(BeTrue())
					Expect(ure.StatusCode).To(Equal(http.StatusInternalServerError))
					Expect(ure.Body).To(Equal("problem"))
				})
			})

			Describe("404 response", func() {
				BeforeEach(func() {
					var err error
					atcServer = ghttp.NewServer()

					target := rc.NewTarget(atcServer.URL(), username, password, cert, insecure)
					client, err = NewClient(target)
					Expect(err).NotTo(HaveOccurred())

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/foo"),
							ghttp.RespondWith(http.StatusNotFound, "problem"),
						),
					)
				})

				It("returns back ResourceNotFoundError", func() {
					err := client.Send(Request{
						RequestName: atc.DeletePipeline,
						Params:      map[string]string{"pipeline_name": "foo"},
					},
						Response{},
					)
					Expect(err).To(HaveOccurred())
					_, ok := err.(ResourceNotFoundError)
					Expect(ok).To(BeTrue())
					Expect(err.Error()).To(Equal("Resource Not Found"))
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
							Location: &atc.Location{
								ID:       4,
								ParentID: 0,
							},
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
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("serializes the given body and sends it in the request body", func() {
				err := client.Send(Request{
					RequestName: atc.CreateBuild,
					Body:        plan,
				},
					Response{},
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
			})
		})
	})
})
