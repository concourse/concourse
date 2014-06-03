package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	ProleBuilds "github.com/winston-ci/prole/api/builds"

	"github.com/winston-ci/winston/api"
	"github.com/winston-ci/winston/api/drainer"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/redisrunner"
)

var _ = Describe("API", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var server *httptest.Server
	var client *http.Client

	var drain *drainer.Drainer

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		drain = drainer.NewDrainer()

		handler, err := api.New(redis, drain)
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

	Describe("PUT /builds/:job/:build", func() {
		var build builds.Build
		var proleBuild ProleBuilds.Build

		var response *http.Response

		version1 := ProleBuilds.Version{"ver": "1"}
		version2 := ProleBuilds.Version{"ver": "2"}

		BeforeEach(func() {
			var err error

			build, err = redis.CreateBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			proleBuild = ProleBuilds.Build{
				Inputs: []ProleBuilds.Input{
					{
						Name:    "some-input",
						Type:    "git",
						Version: version1,
					},
					{
						Name:    "some-other-input",
						Type:    "git",
						Version: version2,
						Metadata: []ProleBuilds.MetadataField{
							{Name: "meta1", Value: "value1"},
							{Name: "meta2", Value: "value2"},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(proleBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/builds/some-job/1", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with status 'started'", func() {
			BeforeEach(func() {
				proleBuild.Status = ProleBuilds.StatusStarted
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.Status).Should(Equal(builds.StatusStarted))
			})

			It("saves the build's inputs", func() {
				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.Inputs).Should(Equal([]builds.Input{
					{
						Name:    "some-input",
						Version: builds.Version(version1),
					},
					{
						Name:    "some-other-input",
						Version: builds.Version(version2),
						Metadata: []builds.MetadataField{
							{Name: "meta1", Value: "value1"},
							{Name: "meta2", Value: "value2"},
						},
					},
				}))
			})

			It("saves each input's current version", func() {
				version, err := redis.GetCurrentVersion("some-job", "some-input")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(version).Should(Equal(builds.Version(version1)))

				version, err = redis.GetCurrentVersion("some-job", "some-other-input")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(version).Should(Equal(builds.Version(version2)))
			})
		})

		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				proleBuild.Status = ProleBuilds.StatusSucceeded

				proleBuild.Outputs = []ProleBuilds.Output{
					{
						Name: "some-output",

						Type:    "git",
						Version: ProleBuilds.Version{"ver": "123"},
					},
					{
						Name: "some-other-output",

						Type:    "git",
						Version: ProleBuilds.Version{"ver": "456"},
					},
				}
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.Status).Should(Equal(builds.StatusSucceeded))
			})

			It("does not save any the job's input's versions", func() {
				_, err := redis.GetCurrentVersion("some-job", "some-input")
				Ω(err).Should(HaveOccurred())

				_, err = redis.GetCurrentVersion("some-job", "some-other-input")
				Ω(err).Should(HaveOccurred())
			})

			It("saves each output version", func() {
				versions, err := redis.GetCommonOutputs([]string{"some-job"}, "some-output")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(versions).Should(Equal([]builds.Version{builds.Version{"ver": "123"}}))

				versions, err = redis.GetCommonOutputs([]string{"some-job"}, "some-other-output")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(versions).Should(Equal([]builds.Version{builds.Version{"ver": "456"}}))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				proleBuild.Status = ProleBuilds.StatusFailed
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.Status).Should(Equal(builds.StatusFailed))
			})

			It("does not save any the job's input's versions", func() {
				_, err := redis.GetCurrentVersion("some-job", "some-input")
				Ω(err).Should(HaveOccurred())

				_, err = redis.GetCurrentVersion("some-job", "some-other-input")
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				proleBuild.Status = ProleBuilds.StatusErrored
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				updatedBuild, err := redis.GetBuild("some-job", build.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(updatedBuild.Status).Should(Equal(builds.StatusErrored))
			})

			It("does not save any the job's input's versions", func() {
				_, err := redis.GetCurrentVersion("some-job", "some-input")
				Ω(err).Should(HaveOccurred())

				_, err = redis.GetCurrentVersion("some-job", "some-other-input")
				Ω(err).Should(HaveOccurred())
			})

			Context("when the build has been aborted", func() {
				BeforeEach(func() {
					err := redis.AbortBuild("some-job", build.ID)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("does not update the build's status", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))

					updatedBuild, err := redis.GetBuild("some-job", build.ID)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(updatedBuild.Status).Should(Equal(builds.StatusAborted))
				})
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
					typ, msg, err := outConn.ReadMessage()
					if err == io.EOF {
						break
					}

					Ω(err).ShouldNot(HaveOccurred())

					Ω(typ).Should(Equal(websocket.TextMessage))

					buf.Write(msg)
				}

				buf.Close()
			}()

			return buf
		}

		It("returns 101", func() {
			conn, response, err := websocket.DefaultDialer.Dial(endpoint, nil)
			Ω(err).ShouldNot(HaveOccurred())

			defer conn.Close()

			Ω(response.StatusCode).Should(Equal(http.StatusSwitchingProtocols))
		})

		Context("when draining", func() {
			Context("and input is being consumed", func() {
				var conn *websocket.Conn

				BeforeEach(func() {
					var err error
					conn, _, err = websocket.DefaultDialer.Dial(endpoint, nil)
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					conn.Close()
				})

				Context("and draining starts", func() {
					It("closes the connection", func(done Done) {
						defer close(done)

						drain.Drain()

						_, _, err := conn.ReadMessage()
						Ω(err).Should(HaveOccurred())
					}, 1)
				})
			})

			Context("and output is being consumed", func() {
				It("closes the outgoing connection", func() {
					output := outputSink()

					drain.Drain()

					Eventually(output.Closed).Should(BeTrue())
				})
			})
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

			It("transmits ansi escape characters as html", func() {
				sink := outputSink()

				err := conn.WriteMessage(websocket.BinaryMessage, []byte("some \x1b[1mmessage"))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(sink).Should(gbytes.Say(`some <span class="ansi-bold">message`))
			})

			Context("when there is a build log saved", func() {
				BeforeEach(func() {
					err := redis.AppendBuildLog("some-job", build.ID, []byte("some saved log"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("immediately returns it", func() {
					Eventually(outputSink()).Should(gbytes.Say("some saved log"))
				})
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

				It("saves the logs to the database", func() {
					err := conn.WriteMessage(websocket.BinaryMessage, []byte("some message"))
					Ω(err).ShouldNot(HaveOccurred())

					err = conn.WriteControl(websocket.CloseMessage, nil, time.Time{})
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(func() string {
						log, err := redis.BuildLog("some-job", build.ID)
						if err != nil {
							return ""
						}

						return string(log)
					}).Should(Equal("hello1hello2\nhello3some message"))
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
})
