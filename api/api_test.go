package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"code.google.com/p/go.net/websocket"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/handler/fakes"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/logfanout"
	logfakes "github.com/concourse/atc/logfanout/fakes"
)

var _ = Describe("API", func() {
	var buildDB *fakes.FakeBuildDB
	var logDB *logfakes.FakeLogDB

	var server *httptest.Server
	var client *http.Client

	var tracker *logfanout.Tracker

	BeforeEach(func() {
		buildDB = new(fakes.FakeBuildDB)
		logDB = new(logfakes.FakeLogDB)

		tracker = logfanout.NewTracker(logDB)

		handler, err := api.New(lagertest.NewTestLogger("api"), buildDB, tracker)
		Ω(err).ShouldNot(HaveOccurred())

		server = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("PUT /builds/:job/:build", func() {
		var build builds.Build
		var turbineBuild TurbineBuilds.Build

		var response *http.Response

		version1 := TurbineBuilds.Version{"ver": "1"}
		version2 := TurbineBuilds.Version{"ver": "2"}

		BeforeEach(func() {
			build = builds.Build{
				ID: 42,
			}

			buildDB.GetBuildReturns(build, nil)

			turbineBuild = TurbineBuilds.Build{
				Inputs: []TurbineBuilds.Input{
					{
						Name:    "some-input",
						Type:    "git",
						Version: version1,
					},
					{
						Name:    "some-other-input",
						Type:    "git",
						Version: version2,
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "meta1", Value: "value1"},
							{Name: "meta2", Value: "value2"},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/builds/some-job/42", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with status 'started'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusStarted
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusStarted))
			})

			It("saves the build's inputs", func() {
				Ω(buildDB.SaveBuildInputCallCount()).Should(Equal(2))

				job, id, input := buildDB.SaveBuildInputArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(input).Should(Equal(builds.VersionedResource{
					Name:     "some-input",
					Type:     "git",
					Version:  builds.Version(version1),
					Metadata: []builds.MetadataField{},
				}))

				job, id, input = buildDB.SaveBuildInputArgsForCall(1)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(input).Should(Equal(builds.VersionedResource{
					Name:    "some-other-input",
					Type:    "git",
					Version: builds.Version(version2),
					Metadata: []builds.MetadataField{
						{Name: "meta1", Value: "value1"},
						{Name: "meta2", Value: "value2"},
					},
				}))
			})
		})

		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusSucceeded

				turbineBuild.Outputs = []TurbineBuilds.Output{
					{
						Name: "some-output",
						Type: "git",

						Source:  TurbineBuilds.Source{"source": "1"},
						Version: TurbineBuilds.Version{"ver": "123"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "meta1", Value: "value1"},
						},
					},
					{
						Name: "some-other-output",
						Type: "git",

						Source:  TurbineBuilds.Source{"source": "2"},
						Version: TurbineBuilds.Version{"ver": "456"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "meta2", Value: "value2"},
						},
					},
				}
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusSucceeded))
			})

			It("saves each output version", func() {
				Ω(buildDB.SaveBuildOutputCallCount()).Should(Equal(2))

				job, id, vr := buildDB.SaveBuildOutputArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(vr).Should(Equal(builds.VersionedResource{
					Name:    "some-output",
					Type:    "git",
					Source:  config.Source{"source": "1"},
					Version: builds.Version{"ver": "123"},
					Metadata: []builds.MetadataField{
						{Name: "meta1", Value: "value1"},
					},
				}))

				job, id, vr = buildDB.SaveBuildOutputArgsForCall(1)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(vr).Should(Equal(builds.VersionedResource{
					Name:    "some-other-output",
					Type:    "git",
					Source:  config.Source{"source": "2"},
					Version: builds.Version{"ver": "456"},
					Metadata: []builds.MetadataField{
						{Name: "meta2", Value: "value2"},
					},
				}))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusFailed
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusFailed))
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusErrored
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusErrored))
			})

			Context("when the build has been aborted", func() {
				BeforeEach(func() {
					build.Status = builds.StatusAborted
					buildDB.GetBuildReturns(build, nil)
				})

				It("does not update the build's status", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))

					Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

					job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
					Ω(job).Should(Equal("some-job"))
					Ω(id).Should(Equal(42))
					Ω(status).Should(Equal(builds.StatusAborted))
				})
			})
		})
	})

	Describe("/builds/:job/:build/log/input", func() {
		var writtenBuildLogs *gbytes.Buffer

		var build builds.Build

		var endpoint string

		var conn io.ReadWriteCloser

		var sinks *sync.WaitGroup

		BeforeEach(func() {
			build = builds.Build{
				ID: 42,
			}

			buildDB.GetBuildReturns(build, nil)

			writtenBuildLogs = gbytes.NewBuffer()

			logDB.AppendBuildLogStub = func(job string, build int, d []byte) error {
				writtenBuildLogs.Write(d)
				return nil
			}

			endpoint = fmt.Sprintf(
				"ws://%s/builds/%s/%d/log/input",
				server.Listener.Addr().String(),
				"some-job",
				build.ID,
			)

			sinks = new(sync.WaitGroup)
		})

		AfterEach(func() {
			sinks.Wait()
		})

		outputSink := func() *gbytes.Buffer {
			buf := gbytes.NewBuffer()

			fanout := tracker.Register("some-job", build.ID, buf)

			sinks.Add(1)
			go func() {
				defer sinks.Done()
				defer GinkgoRecover()

				err := fanout.Attach(buf)
				Ω(err).ShouldNot(HaveOccurred())

				buf.Close()
			}()

			return buf
		}

		Context("when draining", func() {
			Context("and input is being consumed", func() {
				var conn io.ReadWriteCloser

				BeforeEach(func() {
					var err error
					conn, err = websocket.Dial(endpoint, "", "http://0.0.0.0")
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					conn.Close()
				})

				Context("and draining starts", func() {
					It("closes the connection", func(done Done) {
						defer close(done)

						tracker.Drain()

						_, err := conn.Read([]byte{})
						Ω(err).Should(HaveOccurred())
					}, 1)
				})
			})

			Context("and output is being consumed", func() {
				It("closes the outgoing connection", func() {
					output := outputSink()

					Consistently(output.Closed).ShouldNot(BeTrue())

					tracker.Drain()

					Eventually(output.Closed).Should(BeTrue())
				})
			})
		})

		Context("when messages are written", func() {
			BeforeEach(func() {
				var err error

				conn, err = websocket.Dial(endpoint, "", "http://0.0.0.0")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = conn.Write([]byte("hello1"))
				Ω(err).ShouldNot(HaveOccurred())

				_, err = conn.Write([]byte("hello2\n"))
				Ω(err).ShouldNot(HaveOccurred())

				_, err = conn.Write([]byte("hello3"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				conn.Close()
			})

			It("presents them to anyone attached", func() {
				Eventually(outputSink()).Should(gbytes.Say("hello1hello2\nhello3"))
			})

			It("multiplexes to all attached sinks", func() {
				sink1 := outputSink()
				sink2 := outputSink()

				_, err := conn.Write([]byte("some message"))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(sink1).Should(gbytes.Say("some message"))
				Eventually(sink2).Should(gbytes.Say("some message"))
			})

			Context("when there is a build log saved", func() {
				BeforeEach(func() {
					logDB.BuildLogReturns([]byte("some saved log"), nil)
				})

				It("immediately returns it", func() {
					Eventually(outputSink()).Should(gbytes.Say("some saved log"))
				})
			})

			Context("and the input stream closes", func() {
				It("closes the log buffer", func() {
					_, err := conn.Write([]byte("some message"))
					Ω(err).ShouldNot(HaveOccurred())

					sink := outputSink()

					Eventually(sink).Should(gbytes.Say("some message"))

					err = conn.Close()
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sink.Closed).Should(BeTrue())
				})

				It("saves the logs to the database", func() {
					_, err := conn.Write([]byte("some message"))
					Ω(err).ShouldNot(HaveOccurred())

					err = conn.Close()
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(writtenBuildLogs.Contents).Should(Equal([]byte("hello1hello2\nhello3some message")))
				})

				Context("and a second sink attaches", func() {
					It("flushes the buffer and immediately closes", func() {
						_, err := conn.Write([]byte("some message"))
						Ω(err).ShouldNot(HaveOccurred())

						err = conn.Close()
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
