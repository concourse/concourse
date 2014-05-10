package api_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/winston-ci/winston/api"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/redisrunner"
)

var _ = Describe("API", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var server *httptest.Server
	var client *http.Client

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		handler, err := api.New(redis)
		Ω(err).ShouldNot(HaveOccurred())

		server = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
		redisRunner.Stop()
	})

	Describe("PUT /builds/:job/:build/result", func() {
		var build builds.Build
		var status string

		var response *http.Response

		BeforeEach(func() {
			var err error

			build, err = redis.CreateBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			reqPayload := bytes.NewBufferString(fmt.Sprintf(`{"status":%q}`, status))

			req, err := http.NewRequest("PUT", server.URL+"/builds/some-job/1/result", reqPayload)
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				status = "succeeded"
			})

			It("updates the build's state", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.State).Should(Equal(builds.BuildStateSucceeded))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				status = "failed"
			})

			It("updates the build's state", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.State).Should(Equal(builds.BuildStateFailed))
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				status = "errored"
			})

			It("updates the build's state", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.State).Should(Equal(builds.BuildStateErrored))
			})
		})
	})

	Describe("/builds/:job/:build/log/input", func() {
		var build builds.Build

		var endpoint string

		var conn *websocket.Conn
		var response *http.Response

		BeforeEach(func() {
			var err error

			build, err = redis.CreateBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			endpoint = fmt.Sprintf(
				"ws://%s/builds/%s/%d/log/input",
				server.Listener.Addr().String(),
				"some-job",
				build.ID,
			)
		})

		It("returns 101", func() {
			conn, response, err := websocket.DefaultDialer.Dial(endpoint, nil)
			Ω(err).ShouldNot(HaveOccurred())

			defer conn.Close()

			Ω(response.StatusCode).Should(Equal(http.StatusSwitchingProtocols))
		})

		Context("when messages are written", func() {
			BeforeEach(func() {
				var err error

				conn, response, err = websocket.DefaultDialer.Dial(endpoint, nil)
				Ω(err).ShouldNot(HaveOccurred())

				err = conn.WriteMessage(websocket.BinaryMessage, []byte("hello1"))
				Ω(err).ShouldNot(HaveOccurred())

				err = conn.WriteMessage(websocket.BinaryMessage, []byte("hello2\n"))
				Ω(err).ShouldNot(HaveOccurred())

				err = conn.WriteMessage(websocket.BinaryMessage, []byte("hello3"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				conn.Close()
			})

			outputSink := func() *gbytes.Buffer {
				outEndpoint := fmt.Sprintf(
					"ws://%s/builds/%s/%d/log/output",
					server.Listener.Addr().String(),
					"some-job",
					build.ID,
				)

				outConn, outResponse, err := websocket.DefaultDialer.Dial(outEndpoint, nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outResponse.StatusCode).Should(Equal(http.StatusSwitchingProtocols))

				buf := gbytes.NewBuffer()

				go func() {
					defer GinkgoRecover()

					for {
						_, msg, err := outConn.ReadMessage()
						if err == io.EOF {
							break
						}

						Ω(err).ShouldNot(HaveOccurred())

						buf.Write(msg)
					}

					buf.Close()
				}()

				return buf
			}

			It("presents them to /builds/{job}/{id}/logs/output", func() {
				Eventually(outputSink()).Should(gbytes.Say("hello1hello2\nhello3"))
			})

			It("streams them to all open connections to /build/{job}/{id}/logs/output", func() {
				sink1 := outputSink()
				sink2 := outputSink()

				err := conn.WriteMessage(websocket.BinaryMessage, []byte("some message"))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(sink1).Should(gbytes.Say("some message"))
				Eventually(sink2).Should(gbytes.Say("some message"))
			})

			Context("and the input stream closes", func() {
				It("closes the log buffer", func() {
					err := conn.WriteMessage(websocket.BinaryMessage, []byte("some message"))
					Ω(err).ShouldNot(HaveOccurred())

					sink := outputSink()

					err = conn.WriteControl(websocket.CloseMessage, nil, time.Time{})
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sink).Should(gbytes.Say("some message"))
					Eventually(sink.Closed).Should(BeTrue())
				})
			})

			Context("and a second sink attaches", func() {
				It("flushes the buffer and immediately closes", func() {
					err := conn.WriteMessage(websocket.BinaryMessage, []byte("some message"))
					Ω(err).ShouldNot(HaveOccurred())

					err = conn.WriteControl(websocket.CloseMessage, nil, time.Time{})
					Ω(err).ShouldNot(HaveOccurred())

					sink := outputSink()
					Eventually(sink).Should(gbytes.Say("some message"))
					Eventually(sink.Closed).Should(BeTrue())
				})
			})
		})
	})
})
