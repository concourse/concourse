package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc/builds"
)

var _ = Describe("Builds API", func() {
	Describe("POST /api/v1/builds", func() {
		var turbineBuild tbuilds.Build

		var response *http.Response

		BeforeEach(func() {
			turbineBuild = tbuilds.Build{
				Config: tbuilds.Config{
					Run: tbuilds.RunConfig{
						Path: "ls",
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when creating a one-off build succeeds", func() {
			BeforeEach(func() {
				buildsDB.CreateOneOffBuildReturns(builds.Build{ID: 42}, nil)
			})

			Context("and building succeeds", func() {
				It("returns 201 Created", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusCreated))
				})

				It("returns the build ID", func() {
					var build builds.Build
					err := json.NewDecoder(response.Body).Decode(&build)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(build.ID).ShouldNot(BeZero())
				})

				It("executes a one-off build", func() {
					Ω(buildsDB.CreateOneOffBuildCallCount()).Should(Equal(1))

					Ω(builder.BuildCallCount()).Should(Equal(1))
					oneOff, tBuild := builder.BuildArgsForCall(0)
					Ω(oneOff).Should(Equal(builds.Build{ID: 42}))
					Ω(tBuild).Should(Equal(turbineBuild))
				})
			})

			Context("and building fails", func() {
				BeforeEach(func() {
					builder.BuildReturns(errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when creating a one-off build fails", func() {
			BeforeEach(func() {
				buildsDB.CreateOneOffBuildReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/events", func() {
		var conn *websocket.Conn

		JustBeforeEach(func() {
			var err error

			conn, _, err = websocket.DefaultDialer.Dial("ws://"+server.Listener.Addr().String()+"/api/v1/builds/128/events", nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := conn.Close()
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("emits events received for the build", func() {
			fanout := tracker.Register(128, gbytes.NewBuffer())

			sentMsg := json.RawMessage("123")
			err := fanout.WriteMessage(&sentMsg)
			Ω(err).ShouldNot(HaveOccurred())

			var receivedMsg json.RawMessage
			err = conn.ReadJSON(&receivedMsg)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(receivedMsg).Should(Equal(sentMsg))
		})

		It("continuously pings the connection", func() {
			gotPing := make(chan struct{}, 10)

			conn.SetPingHandler(func(string) error {
				gotPing <- struct{}{}
				return nil
			})

			// must be reading to see pings; try for 3 * ping interval and give up,
			// and check that we saw at least 2 pings

			conn.SetReadDeadline(time.Now().Add(3 * pingInterval))

			var receivedMsg json.RawMessage
			err := conn.ReadJSON(&receivedMsg)
			Ω(err).Should(HaveOccurred())

			Ω(gotPing).Should(Receive())
			Ω(gotPing).Should(Receive())
		})
	})

	Describe("POST /api/v1/builds/:build_id/abort", func() {
		var (
			abortTarget *ghttp.Server

			response *http.Response
		)

		BeforeEach(func() {
			abortTarget = ghttp.NewServer()
			abortTarget.AppendHandlers(ghttp.VerifyRequest("POST", "/"))
		})

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds/128/abort", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			abortTarget.Close()
		})

		Context("when the build can be aborted", func() {
			BeforeEach(func() {
				buildsDB.AbortBuildReturns(abortTarget.URL(), nil)
			})

			It("aborts the build via its abort callback", func() {
				Ω(abortTarget.ReceivedRequests()).Should(HaveLen(1))
			})

			Context("and the abort callback returns a status code", func() {
				BeforeEach(func() {
					abortTarget.SetHandler(0, func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusTeapot)
					})
				})

				It("forwards it", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusTeapot))
				})
			})

			Context("and the abort callback fails", func() {
				BeforeEach(func() {
					abortTarget.SetHandler(0, func(w http.ResponseWriter, r *http.Request) {
						abortTarget.HTTPTestServer.CloseClientConnections()
					})
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when the build cannot be aborted", func() {
			BeforeEach(func() {
				buildsDB.AbortBuildReturns("", errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})
})
