package api_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/fakes"
)

var _ = Describe("Builds API", func() {
	Describe("POST /api/v1/builds", func() {
		var plan atc.Plan

		var response *http.Response

		BeforeEach(func() {
			plan = atc.Plan{
				Execute: &atc.ExecutePlan{
					Config: &atc.BuildConfig{
						Run: atc.BuildRunConfig{
							Path: "ls",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(plan)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when creating a one-off build succeeds", func() {
				BeforeEach(func() {
					buildsDB.CreateOneOffBuildReturns(db.Build{
						ID:      42,
						Name:    "1",
						JobName: "job1",
						Status:  db.StatusStarted,
					}, nil)
				})

				Context("and building succeeds", func() {
					It("returns 201 Created", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusCreated))
					})

					It("returns the build", func() {
						body, err := ioutil.ReadAll(response.Body)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(body).Should(MatchJSON(`{
							"id": 42,
							"name": "1",
							"job_name": "job1",
							"status": "started",
							"url": "/jobs/job1/builds/1"
						}`))
					})

					It("executes a one-off build", func() {
						Ω(buildsDB.CreateOneOffBuildCallCount()).Should(Equal(1))

						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
						oneOff, builtPlan := fakeEngine.CreateBuildArgsForCall(0)
						Ω(oneOff).Should(Equal(db.Build{
							ID:      42,
							Name:    "1",
							JobName: "job1",
							Status:  db.StatusStarted,
						}))
						Ω(builtPlan).Should(Equal(plan))
					})
				})

				Context("and building fails", func() {
					BeforeEach(func() {
						fakeEngine.CreateBuildReturns(nil, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when creating a one-off build fails", func() {
				BeforeEach(func() {
					buildsDB.CreateOneOffBuildReturns(db.Build{}, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not trigger a build", func() {
				Ω(buildsDB.CreateOneOffBuildCallCount()).Should(BeZero())
				Ω(fakeEngine.CreateBuildCallCount()).Should(BeZero())
			})
		})
	})

	Describe("GET /api/v1/builds", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/builds")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting all builds succeeds", func() {
			BeforeEach(func() {
				buildsDB.GetAllBuildsReturns([]db.Build{
					{
						ID:      3,
						Name:    "2",
						JobName: "job2",
						Status:  db.StatusStarted,
					},
					{
						ID:      1,
						Name:    "1",
						JobName: "job1",
						Status:  db.StatusSucceeded,
					},
				}, nil)
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns all builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`[
					{
						"id": 3,
						"name": "2",
						"job_name": "job2",
						"status": "started",
						"url": "/jobs/job2/builds/2"
					},
					{
						"id": 1,
						"name": "1",
						"job_name": "job1",
						"status": "succeeded",
						"url": "/jobs/job1/builds/1"
					}
				]`))
			})
		})

		Context("when getting all builds fails", func() {
			BeforeEach(func() {
				buildsDB.GetAllBuildsReturns(nil, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/events", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error

			buildsDB.GetBuildReturns(db.Build{
				ID:      128,
				JobName: "some-job",
			}, nil)

			request, err = http.NewRequest("GET", server.URL+"/api/v1/builds/128/events", nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("returns 200", func() {
				Ω(response.StatusCode).Should(Equal(200))
			})

			It("serves the request via the event handler with no censor", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(string(body)).Should(Equal("fake event handler factory was here"))

				Ω(constructedEventHandler.db).Should(Equal(buildsDB))
				Ω(constructedEventHandler.buildID).Should(Equal(128))
				Ω(constructedEventHandler.censor).Should(BeFalse())
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			Context("and the build is private", func() {
				BeforeEach(func() {
					configDB.GetConfigReturns(atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "some-job", Public: false},
						},
					}, 1, nil)
				})

				It("returns 401", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the build is public", func() {
				BeforeEach(func() {
					configDB.GetConfigReturns(atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "some-job", Public: true},
						},
					}, 1, nil)
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(200))
				})

				It("serves the request via the event handler with a censor", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(string(body)).Should(Equal("fake event handler factory was here"))

					Ω(constructedEventHandler.db).Should(Equal(buildsDB))
					Ω(constructedEventHandler.buildID).Should(Equal(128))
					Ω(constructedEventHandler.censor).Should(BeTrue())
				})
			})
		})
	})

	Describe("POST /api/v1/builds/:build_id/abort", func() {
		var (
			abortTarget *ghttp.Server

			response *http.Response
		)

		BeforeEach(func() {
			abortTarget = ghttp.NewServer()

			abortTarget.AppendHandlers(
				ghttp.VerifyRequest("POST", "/builds/some-guid/abort"),
			)

			buildsDB.GetBuildReturns(db.Build{
				ID:     128,
				Status: db.StatusStarted,
			}, nil)
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

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("and the engine returns a build", func() {
				var fakeBuild *enginefakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(enginefakes.FakeBuild)
					fakeEngine.LookupBuildReturns(fakeBuild, nil)
				})

				It("aborts the build", func() {
					Ω(fakeBuild.AbortCallCount()).Should(Equal(1))
				})

				Context("and aborting succeeds", func() {
					BeforeEach(func() {
						fakeBuild.AbortReturns(nil)
					})

					It("returns 204", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusNoContent))
					})
				})

				Context("and aborting fails", func() {
					BeforeEach(func() {
						fakeBuild.AbortReturns(errors.New("oh no!"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("and the engine returns no build", func() {
				BeforeEach(func() {
					fakeEngine.LookupBuildReturns(nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not abort the build", func() {
				Ω(abortTarget.ReceivedRequests()).Should(BeEmpty())
			})
		})
	})

	Describe("POST /api/v1/builds/:build_id/hijack", func() {
		var (
			requestPayload string
			stepType       string
			stepName       string

			response *http.Response

			clientConn   net.Conn
			clientReader *bufio.Reader

			clientEnc *json.Encoder
			clientDec *json.Decoder
		)

		BeforeEach(func() {
			requestPayload = `{"path":"ls"}`
			stepType = "execute"
			stepName = "build"

			buildsDB.GetBuildReturns(db.Build{
				ID: 128,
			}, nil)
		})

		JustBeforeEach(func() {
			var err error

			hijackReq, err := http.NewRequest(
				"POST",
				server.URL+"/api/v1/builds/128/hijack",
				bytes.NewBufferString(requestPayload),
			)
			Ω(err).ShouldNot(HaveOccurred())

			hijackReq.URL.RawQuery = url.Values{
				"type": []string{stepType},
				"name": []string{stepName},
			}.Encode()

			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			Ω(err).ShouldNot(HaveOccurred())

			client := httputil.NewClientConn(conn, nil)

			response, err = client.Do(hijackReq)
			Ω(err).ShouldNot(HaveOccurred())

			clientConn, clientReader = client.Hijack()

			clientEnc = json.NewEncoder(clientConn)
			clientDec = json.NewDecoder(clientReader)
		})

		AfterEach(func() {
			clientConn.Close()
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("and the engine returns a build", func() {
				var fakeBuild *enginefakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(enginefakes.FakeBuild)
					fakeEngine.LookupBuildReturns(fakeBuild, nil)
				})

				Context("when hijacking succeeds", func() {
					var (
						fakeProcess *enginefakes.FakeHijackedProcess
						processExit chan int
					)

					BeforeEach(func() {
						processExit = make(chan int)

						fakeProcess = new(enginefakes.FakeHijackedProcess)
						fakeProcess.WaitStub = func() (int, error) {
							return <-processExit, nil
						}

						fakeBuild.HijackReturns(fakeProcess, nil)
					})

					AfterEach(func() {
						close(processExit)
					})

					It("hijacks the build", func() {
						Eventually(fakeBuild.HijackCallCount).Should(Equal(1))

						Ω(fakeEngine.LookupBuildArgsForCall(0)).Should(Equal(db.Build{
							ID: 128,
						}))

						target, spec, io := fakeBuild.HijackArgsForCall(0)
						Ω(target).Should(Equal(engine.HijackTarget{
							Type: engine.HijackTargetType(stepType),
							Name: stepName,
						}))
						Ω(spec).Should(Equal(atc.HijackProcessSpec{
							Path: "ls",
						}))
						Ω(io.Stdin).ShouldNot(BeNil())
						Ω(io.Stdout).ShouldNot(BeNil())
						Ω(io.Stderr).ShouldNot(BeNil())
					})

					Context("when stdin is sent over the API", func() {
						JustBeforeEach(func() {
							err := clientEnc.Encode(atc.HijackInput{
								Stdin: []byte("some stdin\n"),
							})
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("forwards the payload to the process", func() {
							_, _, io := fakeBuild.HijackArgsForCall(0)
							Ω(bufio.NewReader(io.Stdin).ReadBytes('\n')).Should(Equal([]byte("some stdin\n")))
						})
					})

					Context("when the process prints to stdout", func() {
						JustBeforeEach(func() {
							Eventually(fakeBuild.HijackCallCount).Should(Equal(1))

							_, _, io := fakeBuild.HijackArgsForCall(0)

							_, err := fmt.Fprintf(io.Stdout, "some stdout\n")
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("forwards it to the response", func() {
							var hijackOutput atc.HijackOutput
							err := clientDec.Decode(&hijackOutput)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(hijackOutput).Should(Equal(atc.HijackOutput{
								Stdout: []byte("some stdout\n"),
							}))
						})
					})

					Context("when the process prints to stderr", func() {
						JustBeforeEach(func() {
							Eventually(fakeBuild.HijackCallCount).Should(Equal(1))

							_, _, io := fakeBuild.HijackArgsForCall(0)

							_, err := fmt.Fprintf(io.Stderr, "some stderr\n")
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("forwards it to the response", func() {
							var hijackOutput atc.HijackOutput
							err := clientDec.Decode(&hijackOutput)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(hijackOutput).Should(Equal(atc.HijackOutput{
								Stderr: []byte("some stderr\n"),
							}))
						})
					})

					Context("when the process exits", func() {
						JustBeforeEach(func() {
							Eventually(processExit).Should(BeSent(123))
						})

						It("forwards its exit status to the response", func() {
							var hijackOutput atc.HijackOutput
							err := clientDec.Decode(&hijackOutput)
							Ω(err).ShouldNot(HaveOccurred())

							exitStatus := 123
							Ω(hijackOutput).Should(Equal(atc.HijackOutput{
								ExitStatus: &exitStatus,
							}))
						})
					})

					Context("when new tty settings are sent over the API", func() {
						JustBeforeEach(func() {
							err := clientEnc.Encode(atc.HijackInput{
								TTYSpec: &atc.HijackTTYSpec{
									WindowSize: atc.HijackWindowSize{
										Columns: 123,
										Rows:    456,
									},
								},
							})
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("forwards it to the process", func() {
							Eventually(fakeProcess.SetTTYCallCount).Should(Equal(1))

							Ω(fakeProcess.SetTTYArgsForCall(0)).Should(Equal(atc.HijackTTYSpec{
								WindowSize: atc.HijackWindowSize{
									Columns: 123,
									Rows:    456,
								},
							}))
						})

						Context("and setting the TTY on the process fails", func() {
							BeforeEach(func() {
								fakeProcess.SetTTYReturns(errors.New("oh no!"))
							})

							It("forwards the error to the response", func() {
								var hijackOutput atc.HijackOutput
								err := clientDec.Decode(&hijackOutput)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(hijackOutput).Should(Equal(atc.HijackOutput{
									Error: "oh no!",
								}))
							})
						})
					})

					Context("when waiting on the process fails", func() {
						BeforeEach(func() {
							fakeProcess.WaitReturns(0, errors.New("oh no!"))
						})

						It("forwards the error to the response", func() {
							var hijackOutput atc.HijackOutput
							err := clientDec.Decode(&hijackOutput)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(hijackOutput).Should(Equal(atc.HijackOutput{
								Error: "oh no!",
							}))
						})
					})
				})
			})

			Context("when the build cannot be found via the engine", func() {
				BeforeEach(func() {
					fakeEngine.LookupBuildReturns(nil, errors.New("oh no!"))
				})

				It("returns 404 Not Found", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
				})
			})

			Context("when the build cannot be found in the database", func() {
				BeforeEach(func() {
					buildsDB.GetBuildReturns(db.Build{}, errors.New("oh no!"))
				})

				It("returns 404 Not Found", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
				})
			})

			Context("when the request payload is invalid", func() {
				BeforeEach(func() {
					requestPayload = "ß"
				})

				It("returns Bad Request", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not hijack the build", func() {
				Ω(fakeEngine.LookupBuildCallCount()).Should(BeZero())
			})
		})
	})
})
