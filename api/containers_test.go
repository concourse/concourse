package api_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"github.com/gorilla/websocket"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker/workerfakes"
)

var _ = Describe("Containers API", func() {
	var (
		pipelineName     = "some-pipeline"
		jobName          = "some-job"
		stepType         = db.ContainerTypeTask
		stepName         = "some-step"
		resourceName     = "some-resource"
		buildID          = 1234
		buildName        = "2"
		handle           = "some-handle"
		workerName       = "some-worker-guid"
		workingDirectory = "/tmp/build/my-favorite-guid"
		envVariables     = []string{"VAR1=VAL1"}
		attempts         = []int{1, 5}
		user             = "snoopy"

		req *http.Request

		fakeContainer1 db.SavedContainer
	)

	BeforeEach(func() {
		fakeContainer1 = db.SavedContainer{
			Container: db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					BuildID: buildID,
				},
				ContainerMetadata: db.ContainerMetadata{
					StepName:             stepName,
					PipelineName:         pipelineName,
					JobName:              jobName,
					BuildName:            buildName,
					Type:                 stepType,
					WorkerName:           workerName,
					WorkingDirectory:     workingDirectory,
					EnvironmentVariables: envVariables,
					Attempts:             attempts,
					User:                 user,
					Handle:               handle,
				},
			},
		}
	})

	Describe("GET /api/v1/containers", func() {
		BeforeEach(func() {
			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
			teamDBFactory.GetTeamDBReturns(teamDB)
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", 42, true, true)
			})

			Context("with no params", func() {
				Context("when no errors are returned", func() {
					var (
						fakeContainer2 db.SavedContainer
						fakeContainers []db.SavedContainer
					)

					BeforeEach(func() {
						fakeContainer2 = db.SavedContainer{
							Container: db.Container{
								ContainerMetadata: db.ContainerMetadata{
									PipelineName: "some-other-pipeline",
									Type:         db.ContainerTypeCheck,
									ResourceName: "some-resource",
									WorkerName:   "some-other-worker-guid",
									Handle:       "some-other-handle",
								},
							},
						}

						fakeContainers = []db.SavedContainer{
							fakeContainer1,
							fakeContainer2,
						}
						teamDB.FindContainersByDescriptorsReturns(fakeContainers, nil)
					})

					It("returns 200", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type application/json", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns all containers", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
							[
								{
									"id": "some-handle",
									"worker_name": "some-worker-guid",
									"pipeline_name": "some-pipeline",
									"job_name": "some-job",
									"build_name": "2",
									"build_id": 1234,
									"step_type": "task",
									"step_name": "some-step",
									"working_directory": "/tmp/build/my-favorite-guid",
									"env_variables": ["VAR1=VAL1"],
									"attempt": [1,5],
									"user": "snoopy"
								},
								{
									"id": "some-other-handle",
									"worker_name": "some-other-worker-guid",
									"pipeline_name": "some-other-pipeline",
									"resource_name": "some-resource"
								}
							]
						`))
					})
				})

				Context("when no containers are found", func() {
					BeforeEach(func() {
						teamDB.FindContainersByDescriptorsReturns([]db.SavedContainer{}, nil)
					})

					It("returns 200", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns an empty array", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
						  []
						`))
					})
				})

				Context("when there is an error", func() {
					var (
						expectedErr error
					)

					BeforeEach(func() {
						expectedErr = errors.New("some error")
						teamDB.FindContainersByDescriptorsReturns([]db.SavedContainer{}, expectedErr)
					})

					It("returns 500", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Describe("querying with pipeline name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"pipeline_name": []string{pipelineName},
					}.Encode()
				})

				It("queries the db via the pipeline name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							PipelineName: pipelineName,
						},
					}
					Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
					Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with job name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"job_name": []string{jobName},
					}.Encode()
				})

				It("calls db.Containers with the queried job name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							JobName: jobName,
						},
					}
					Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
					Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with type", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"type": []string{string(stepType)},
					}.Encode()
				})

				It("calls db.Containers with the queried type", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Type: stepType,
						},
					}
					Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
					Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with resource name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"resource_name": []string{string(resourceName)},
					}.Encode()
				})

				It("calls db.Containers with the queried resource name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							ResourceName: resourceName,
						},
					}
					Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
					Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with step name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"step_name": []string{string(stepName)},
					}.Encode()
				})

				It("calls db.Containers with the queried step name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							StepName: stepName,
						},
					}
					Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
					Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with build name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"build_name": []string{buildName},
					}.Encode()
				})

				It("calls db.Containers with the queried build name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							BuildName: buildName,
						},
					}
					Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
					Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with build-id", func() {
				Context("when the buildID can be parsed as an int", func() {
					BeforeEach(func() {
						buildIDString := strconv.Itoa(buildID)

						req.URL.RawQuery = url.Values{
							"build-id": []string{buildIDString},
						}.Encode()
					})

					It("calls db.Containers with the queried build id", func() {
						_, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						expectedArgs := db.Container{
							ContainerIdentifier: db.ContainerIdentifier{
								BuildID: buildID,
							},
						}
						Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
						Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
					})

					Context("when the buildID fails to be parsed as an int", func() {
						BeforeEach(func() {
							req.URL.RawQuery = url.Values{
								"build-id": []string{"not-an-int"},
							}.Encode()
						})

						It("returns 400 Bad Request", func() {
							response, _ := client.Do(req)
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})

						It("does not lookup containers", func() {
							client.Do(req)

							Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(0))
						})
					})
				})
			})

			Describe("querying with attempts", func() {
				Context("when the attempts can be parsed as a slice of int", func() {
					BeforeEach(func() {
						attemptsString := "[1,5]"

						req.URL.RawQuery = url.Values{
							"attempt": []string{attemptsString},
						}.Encode()
					})

					It("calls db.Containers with the queried attempts", func() {
						_, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						expectedArgs := db.Container{
							ContainerMetadata: db.ContainerMetadata{
								Attempts: attempts,
							},
						}
						Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(1))
						Expect(teamDB.FindContainersByDescriptorsArgsForCall(0)).To(Equal(expectedArgs))
					})

					Context("when the attempts fails to be parsed as a slice of int", func() {
						BeforeEach(func() {
							req.URL.RawQuery = url.Values{
								"attempt": []string{"not-a-slice"},
							}.Encode()
						})

						It("returns 400 Bad Request", func() {
							response, _ := client.Do(req)
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})

						It("does not lookup containers", func() {
							client.Do(req)

							Expect(teamDB.FindContainersByDescriptorsCallCount()).To(Equal(0))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/containers/:id", func() {
		BeforeEach(func() {
			teamDB.GetContainerReturns(fakeContainer1, true, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers/"+handle, nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", 42, true, true)
			})

			Context("when the container is not found", func() {
				BeforeEach(func() {
					teamDB.GetContainerReturns(db.SavedContainer{}, false, nil)
				})

				It("returns 404 Not Found", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the container is found", func() {
				BeforeEach(func() {
					teamDB.GetContainerReturns(fakeContainer1, true, nil)
				})

				It("returns 200 OK", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type application/json", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("performs lookup by id", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(teamDB.GetContainerCallCount()).To(Equal(1))
					Expect(teamDB.GetContainerArgsForCall(0)).To(Equal(handle))
				})

				It("returns the container", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`
						{
							"pipeline_name": "some-pipeline",
							"step_type": "task",
							"step_name": "some-step",
							"job_name": "some-job",
							"build_id": 1234,
							"build_name": "2",
							"id": "some-handle",
							"worker_name": "some-worker-guid",
							"working_directory": "/tmp/build/my-favorite-guid",
							"env_variables": ["VAR1=VAL1"],
							"attempt": [1,5],
							"user": "snoopy"
						}
					`))
				})
			})

			Context("when there is an error", func() {
				var (
					expectedErr error
				)

				BeforeEach(func() {
					expectedErr = errors.New("some error")
					teamDB.GetContainerReturns(db.SavedContainer{}, false, expectedErr)
				})

				It("returns 500", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("GET /api/v1/containers/:id/hijack", func() {
		var (
			requestPayload string

			conn     *websocket.Conn
			response *http.Response

			expectBadHandshake bool
		)

		BeforeEach(func() {
			expectBadHandshake = false
			requestPayload = `{"path":"ls", "user": "snoopy"}`
		})

		JustBeforeEach(func() {
			wsURL, err := url.Parse(server.URL)
			Expect(err).NotTo(HaveOccurred())

			wsURL.Scheme = "ws"
			wsURL.Path = "/api/v1/containers/" + handle + "/hijack"

			dialer := websocket.Dialer{}
			conn, response, err = dialer.Dial(wsURL.String(), nil)
			if !expectBadHandshake {
				Expect(err).NotTo(HaveOccurred())

				writer, err := conn.NextWriter(websocket.TextMessage)
				Expect(err).NotTo(HaveOccurred())

				_, err = writer.Write([]byte(requestPayload))
				Expect(err).NotTo(HaveOccurred())

				err = writer.Close()
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			if !expectBadHandshake {
				conn.Close()
			}
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", 42, true, true)
			})

			Context("and the worker client returns a container", func() {
				var (
					fakeDBContainer db.SavedContainer
					fakeContainer   *workerfakes.FakeContainer
				)

				BeforeEach(func() {
					fakeDBContainer = db.SavedContainer{}
					teamDB.GetContainerReturns(fakeDBContainer, true, nil)

					fakeContainer = new(workerfakes.FakeContainer)
					fakeWorkerClient.LookupContainerReturns(fakeContainer, true, nil)
				})

				Context("when the call to lookup the container returns an error", func() {
					BeforeEach(func() {
						fakeWorkerClient.LookupContainerReturns(nil, false, errors.New("nope"))
					})

					It("closes the websocket connection with an error", func() {
						_, _, err := conn.ReadMessage()

						Expect(websocket.IsCloseError(err, 1011)).To(BeTrue()) // internal server error
						Expect(err).To(MatchError(ContainSubstring("failed to lookup container")))
					})
				})

				Context("when the container could not be found on the worker client", func() {
					BeforeEach(func() {
						fakeWorkerClient.LookupContainerReturns(nil, false, nil)
					})

					It("closes the websocket connection with an error", func() {
						_, _, err := conn.ReadMessage()

						Expect(websocket.IsCloseError(err, 1011)).To(BeTrue()) // internal server error
						Expect(err).To(MatchError(ContainSubstring("could not find container")))
					})
				})

				Context("when the request payload is invalid", func() {
					BeforeEach(func() {
						requestPayload = "ß"
					})

					It("closes the connection with an error", func() {
						_, _, err := conn.ReadMessage()

						Expect(websocket.IsCloseError(err, 1003)).To(BeTrue()) // unsupported data
						Expect(err).To(MatchError(ContainSubstring("malformed process spec")))
					})
				})

				Context("when running the process succeeds", func() {
					var (
						fakeProcess *gfakes.FakeProcess
						processExit chan int
					)

					BeforeEach(func() {
						exit := make(chan int)
						processExit = exit

						fakeProcess = new(gfakes.FakeProcess)
						fakeProcess.WaitStub = func() (int, error) {
							return <-exit, nil
						}

						fakeContainer.RunReturns(fakeProcess, nil)
					})

					AfterEach(func() {
						close(processExit)
					})

					It("hijacks the build", func() {
						Eventually(fakeContainer.RunCallCount).Should(Equal(1))

						_, lookedUpID := fakeWorkerClient.LookupContainerArgsForCall(0)
						Expect(lookedUpID).To(Equal(handle))

						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "ls",
							User: "snoopy",
						}))

						Expect(io.Stdin).NotTo(BeNil())
						Expect(io.Stdout).NotTo(BeNil())
						Expect(io.Stderr).NotTo(BeNil())
					})

					Context("when stdin is sent over the API", func() {
						JustBeforeEach(func() {
							err := conn.WriteJSON(atc.HijackInput{
								Stdin: []byte("some stdin\n"),
							})
							Expect(err).NotTo(HaveOccurred())
						})

						It("forwards the payload to the process", func() {
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)
							Expect(bufio.NewReader(io.Stdin).ReadBytes('\n')).To(Equal([]byte("some stdin\n")))
						})
					})

					Context("when stdin is closed via the API", func() {
						JustBeforeEach(func() {
							err := conn.WriteJSON(atc.HijackInput{
								Closed: true,
							})
							Expect(err).NotTo(HaveOccurred())
						})

						It("closes the process's stdin", func() {
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, ioConfig := fakeContainer.RunArgsForCall(0)
							_, err := ioConfig.Stdin.Read(make([]byte, 10))
							Expect(err).To(Equal(io.EOF))
						})
					})

					Context("when the process prints to stdout", func() {
						JustBeforeEach(func() {
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)

							_, err := fmt.Fprintf(io.Stdout, "some stdout\n")
							Expect(err).NotTo(HaveOccurred())
						})

						It("forwards it to the response", func() {
							var hijackOutput atc.HijackOutput
							err := conn.ReadJSON(&hijackOutput)
							Expect(err).NotTo(HaveOccurred())

							Expect(hijackOutput).To(Equal(atc.HijackOutput{
								Stdout: []byte("some stdout\n"),
							}))
						})
					})

					Context("when the process prints to stderr", func() {
						JustBeforeEach(func() {
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)

							_, err := fmt.Fprintf(io.Stderr, "some stderr\n")
							Expect(err).NotTo(HaveOccurred())
						})

						It("forwards it to the response", func() {
							var hijackOutput atc.HijackOutput
							err := conn.ReadJSON(&hijackOutput)
							Expect(err).NotTo(HaveOccurred())

							Expect(hijackOutput).To(Equal(atc.HijackOutput{
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
							err := conn.ReadJSON(&hijackOutput)
							Expect(err).NotTo(HaveOccurred())

							exitStatus := 123
							Expect(hijackOutput).To(Equal(atc.HijackOutput{
								ExitStatus: &exitStatus,
							}))

						})

						It("releases the container", func() {
							Eventually(fakeContainer.ReleaseCallCount).Should(Equal(1))
						})
					})

					Context("when new tty settings are sent over the API", func() {
						JustBeforeEach(func() {
							err := conn.WriteJSON(atc.HijackInput{
								TTYSpec: &atc.HijackTTYSpec{
									WindowSize: atc.HijackWindowSize{
										Columns: 123,
										Rows:    456,
									},
								},
							})
							Expect(err).NotTo(HaveOccurred())
						})

						It("forwards it to the process", func() {
							Eventually(fakeProcess.SetTTYCallCount).Should(Equal(1))

							Expect(fakeProcess.SetTTYArgsForCall(0)).To(Equal(garden.TTYSpec{
								WindowSize: &garden.WindowSize{
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
								err := conn.ReadJSON(&hijackOutput)
								Expect(err).NotTo(HaveOccurred())

								Expect(hijackOutput).To(Equal(atc.HijackOutput{
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
							err := conn.ReadJSON(&hijackOutput)
							Expect(err).NotTo(HaveOccurred())

							Expect(hijackOutput).To(Equal(atc.HijackOutput{
								Error: "oh no!",
							}))
						})
					})
				})
			})

			Context("when the container cannot be found", func() {
				BeforeEach(func() {
					expectBadHandshake = true

					teamDB.GetContainerReturns(db.SavedContainer{}, false, nil)
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
				})
			})

			Context("when the db request fails", func() {
				BeforeEach(func() {
					expectBadHandshake = true

					fakeErr := errors.New("error")
					teamDB.GetContainerReturns(db.SavedContainer{}, false, fakeErr)
				})

				It("returns 500 internal error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				expectBadHandshake = true

				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not hijack the build", func() {
				Expect(fakeEngine.LookupBuildCallCount()).To(BeZero())
			})
		})
	})
})
