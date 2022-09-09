package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	. "github.com/concourse/concourse/atc/testhelpers"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Containers API", func() {
	var (
		stepType         = db.ContainerTypeTask
		stepName         = "some-step"
		pipelineID       = 1111
		jobID            = 2222
		buildID          = 3333
		workingDirectory = "/tmp/build/my-favorite-guid"
		attempt          = "1.5"
		user             = "snoopy"

		req *http.Request

		fakeContainer1 *dbfakes.FakeContainer
		fakeContainer2 *dbfakes.FakeContainer
	)

	BeforeEach(func() {
		fakeContainer1 = new(dbfakes.FakeContainer)
		fakeContainer1.HandleReturns("some-handle")
		fakeContainer1.StateReturns("container-state")
		fakeContainer1.WorkerNameReturns("some-worker-name")
		fakeContainer1.MetadataReturns(db.ContainerMetadata{
			Type: stepType,

			StepName: stepName,
			Attempt:  attempt,

			PipelineID: pipelineID,
			JobID:      jobID,
			BuildID:    buildID,

			WorkingDirectory: workingDirectory,
			User:             user,
		})

		fakeContainer2 = new(dbfakes.FakeContainer)
		fakeContainer2.HandleReturns("some-other-handle")
		fakeContainer2.WorkerNameReturns("some-other-worker-name")
		fakeContainer2.MetadataReturns(db.ContainerMetadata{
			Type: stepType,

			StepName: stepName + "-other",
			Attempt:  attempt + ".1",

			PipelineID: pipelineID + 1,
			JobID:      jobID + 1,
			BuildID:    buildID + 1,

			WorkingDirectory: workingDirectory + "/other",
			User:             user + "-other",
		})
	})

	Describe("GET /api/v1/teams/a-team/containers", func() {
		BeforeEach(func() {
			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/containers", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("with no params", func() {
				Context("when no errors are returned", func() {
					BeforeEach(func() {
						dbTeam.ContainersReturns([]db.Container{fakeContainer1, fakeContainer2}, nil)
					})

					It("returns 200", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type application/json", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
									"worker_name": "some-worker-name",
									"type": "task",
									"step_name": "some-step",
									"attempt": "1.5",
									"pipeline_id": 1111,
									"job_id": 2222,
									"state": "container-state",
									"build_id": 3333,
									"working_directory": "/tmp/build/my-favorite-guid",
									"user": "snoopy"
								},
								{
									"id": "some-other-handle",
									"worker_name": "some-other-worker-name",
									"type": "task",
									"step_name": "some-step-other",
									"attempt": "1.5.1",
									"pipeline_id": 1112,
									"job_id": 2223,
									"build_id": 3334,
									"working_directory": "/tmp/build/my-favorite-guid/other",
									"user": "snoopy-other"
								}
							]
						`))
					})
				})

				Context("when no containers are found", func() {
					BeforeEach(func() {
						dbTeam.ContainersReturns([]db.Container{}, nil)
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
						dbTeam.ContainersReturns(nil, expectedErr)
					})

					It("returns 500", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Describe("querying with pipeline id", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"pipeline_id": []string{strconv.Itoa(pipelineID)},
					}.Encode()
				})

				It("queries with it in the metadata", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(dbTeam.FindContainersByMetadataCallCount()).To(Equal(1))

					meta := dbTeam.FindContainersByMetadataArgsForCall(0)
					Expect(meta).To(Equal(db.ContainerMetadata{
						PipelineID: pipelineID,
					}))
				})
			})

			Describe("querying with pipeline instance vars", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"vars": []string{`{"branch":"master"}`},
					}.Encode()
				})

				It("queries with it in the metadata", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					meta := dbTeam.FindContainersByMetadataArgsForCall(0)
					Expect(meta).To(Equal(db.ContainerMetadata{
						PipelineInstanceVars: `{"branch":"master"}`,
					}))
				})
			})

			Describe("querying with job id", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"job_id": []string{strconv.Itoa(jobID)},
					}.Encode()
				})

				It("queries with it in the metadata", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(dbTeam.FindContainersByMetadataCallCount()).To(Equal(1))

					meta := dbTeam.FindContainersByMetadataArgsForCall(0)
					Expect(meta).To(Equal(db.ContainerMetadata{
						JobID: jobID,
					}))
				})
			})

			Describe("querying with type", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"type": []string{string(stepType)},
					}.Encode()
				})

				It("queries with it in the metadata", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					meta := dbTeam.FindContainersByMetadataArgsForCall(0)
					Expect(meta).To(Equal(db.ContainerMetadata{
						Type: stepType,
					}))
				})
			})

			Describe("querying with step name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"step_name": []string{stepName},
					}.Encode()
				})

				It("queries with it in the metadata", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					meta := dbTeam.FindContainersByMetadataArgsForCall(0)
					Expect(meta).To(Equal(db.ContainerMetadata{
						StepName: stepName,
					}))
				})
			})

			Describe("querying with build id", func() {
				Context("when the buildID can be parsed as an int", func() {
					BeforeEach(func() {
						buildIDString := strconv.Itoa(buildID)

						req.URL.RawQuery = url.Values{
							"build_id": []string{buildIDString},
						}.Encode()
					})

					It("queries with it in the metadata", func() {
						_, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						meta := dbTeam.FindContainersByMetadataArgsForCall(0)
						Expect(meta).To(Equal(db.ContainerMetadata{
							BuildID: buildID,
						}))
					})

					Context("when the buildID fails to be parsed as an int", func() {
						BeforeEach(func() {
							req.URL.RawQuery = url.Values{
								"build_id": []string{"not-an-int"},
							}.Encode()
						})

						It("returns 400 Bad Request", func() {
							response, _ := client.Do(req)
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})

						It("does not lookup containers", func() {
							_, _ = client.Do(req)
							Expect(dbTeam.FindContainersByMetadataCallCount()).To(Equal(0))
						})
					})
				})
			})

			Describe("querying with attempts", func() {
				Context("when the attempts can be parsed as a slice of int", func() {
					BeforeEach(func() {
						req.URL.RawQuery = url.Values{
							"attempt": []string{attempt},
						}.Encode()
					})

					It("queries with it in the metadata", func() {
						_, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						meta := dbTeam.FindContainersByMetadataArgsForCall(0)
						Expect(meta).To(Equal(db.ContainerMetadata{
							Attempt: attempt,
						}))
					})
				})
			})

			Describe("querying with type 'check'", func() {
				BeforeEach(func() {
					rawInstanceVars, _ := json.Marshal(atc.InstanceVars{"branch": "master"})
					req.URL.RawQuery = url.Values{
						"type":          []string{"check"},
						"resource_name": []string{"some-resource"},
						"pipeline_name": []string{"some-pipeline"},
						"vars":          []string{string(rawInstanceVars)},
					}.Encode()
				})

				It("queries with check properties", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					_, pipelineRef, resourceName := dbTeam.FindCheckContainersArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{Name: "some-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}))
					Expect(resourceName).To(Equal("some-resource"))
				})
			})
		})
	})

	Describe("GET /api/v1/containers/:id", func() {
		var handle = "some-handle"

		BeforeEach(func() {
			dbTeam.FindContainerByHandleReturns(fakeContainer1, true, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/containers/"+handle, nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when the container is not found", func() {
				BeforeEach(func() {
					dbTeam.FindContainerByHandleReturns(nil, false, nil)
				})

				It("returns 404 Not Found", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the container is found", func() {
				BeforeEach(func() {
					dbTeam.FindContainerByHandleReturns(fakeContainer1, true, nil)
				})

				Context("when the container is within the team", func() {
					BeforeEach(func() {
						dbTeam.IsCheckContainerReturns(false, nil)
						dbTeam.IsContainerWithinTeamReturns(true, nil)
					})

					It("returns 200 OK", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type application/json", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("performs lookup by id", func() {
						_, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(dbTeam.FindContainerByHandleCallCount()).To(Equal(1))
						Expect(dbTeam.FindContainerByHandleArgsForCall(0)).To(Equal(handle))
					})

					It("returns the container", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
	 					{
	 						"id": "some-handle",
							"state": "container-state",
	 						"worker_name": "some-worker-name",
	 						"type": "task",
	 						"step_name": "some-step",
	 						"attempt": "1.5",
	 						"pipeline_id": 1111,
	 						"job_id": 2222,
	 						"build_id": 3333,
	 						"working_directory": "/tmp/build/my-favorite-guid",
	 						"user": "snoopy"
	 					}
	 				`))
					})
				})

				Context("when the container is not within the team", func() {
					BeforeEach(func() {
						dbTeam.IsCheckContainerReturns(false, nil)
						dbTeam.IsContainerWithinTeamReturns(false, nil)
					})

					It("returns 404 Not Found", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when there is an error", func() {
				var (
					expectedErr error
				)

				BeforeEach(func() {
					expectedErr = errors.New("some error")
					dbTeam.FindContainerByHandleReturns(nil, false, expectedErr)
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
			handle = "some-handle"

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
			wsURL.Path = "/api/v1/teams/a-team/containers/" + handle + "/hijack"

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
				_ = conn.Close()
			}
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("and the worker pool returns a container", func() {
				var (
					container *runtimetest.Container
				)

				waitForHijack := func() *runtimetest.Process {
					Eventually(container.RunningProcesses).Should(HaveLen(1))
					return container.RunningProcesses()[0]
				}

				BeforeEach(func() {
					container = runtimetest.NewContainer().WithProcess(
						runtime.ProcessSpec{
							Path: "ls",
							User: "snoopy",
						},
						runtimetest.ProcessStub{},
					)
					fakeWorkerPool.LocateContainerReturns(container, runtimetest.NewWorker("worker"), true, nil)
				})

				Context("when the container is a check container", func() {
					BeforeEach(func() {
						dbTeam.IsCheckContainerReturns(true, nil)
					})

					Context("when the user is not admin", func() {
						BeforeEach(func() {
							expectBadHandshake = true

							fakeAccess.IsAdminReturns(false)
						})

						It("returns Forbidden", func() {
							Expect(response.StatusCode).To(Equal(http.StatusForbidden))
						})
					})

					Context("when the user is an admin", func() {
						BeforeEach(func() {
							fakeAccess.IsAdminReturns(true)
						})

						Context("when the container is not within the team", func() {
							BeforeEach(func() {
								expectBadHandshake = true

								dbTeam.IsContainerWithinTeamReturns(false, nil)
							})

							It("returns 404 not found", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when the container is within the team", func() {
							var processExit chan int

							BeforeEach(func() {
								dbTeam.IsContainerWithinTeamReturns(true, nil)

								processExit = make(chan int)
								// capture in scope of Call to avoid data race
								exit := processExit

								container.ProcessDefs[0].Stub.Call = func(_ context.Context, _ *runtimetest.Process) (runtime.ProcessResult, error) {
									return runtime.ProcessResult{ExitStatus: <-exit}, nil
								}
							})

							AfterEach(func() {
								close(processExit)
							})

							It("should try to hijack the container", func() {
								waitForHijack()
							})
						})
					})
				})

				Context("when the container is a build step container", func() {
					BeforeEach(func() {
						dbTeam.IsCheckContainerReturns(false, nil)
					})

					Context("when the container is not within the team", func() {
						BeforeEach(func() {
							expectBadHandshake = true

							dbTeam.IsContainerWithinTeamReturns(false, nil)
						})

						It("returns 404 not found", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when the container is within the team", func() {
						BeforeEach(func() {
							dbTeam.IsContainerWithinTeamReturns(true, nil)
						})

						Context("when the call to lookup the container returns an error", func() {
							BeforeEach(func() {
								expectBadHandshake = true

								fakeWorkerPool.LocateContainerReturns(nil, nil, false, errors.New("nope"))
							})

							It("returns 500 internal error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("when the container could not be found on the worker client", func() {
							BeforeEach(func() {
								expectBadHandshake = true

								fakeWorkerPool.LocateContainerReturns(nil, nil, false, nil)
							})

							It("returns 404 Not Found", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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

						Context("when running the process fails", func() {
							BeforeEach(func() {
								// unsetting the expected process means that running will fail
								container.ProcessDefs = nil
							})

							It("receives the error in the output", func() {
								var hijackOutput atc.HijackOutput
								err := conn.ReadJSON(&hijackOutput)
								Expect(err).ToNot(HaveOccurred())
								Expect(hijackOutput.Error).ToNot(BeEmpty())
							})
						})

						Context("when running the process succeeds", func() {
							var processExit chan int

							BeforeEach(func() {
								processExit = make(chan int)
								// capture in scope of Call to avoid data race
								exit := processExit

								container.ProcessDefs[0].Stub.Call = func(_ context.Context, _ *runtimetest.Process) (runtime.ProcessResult, error) {
									return runtime.ProcessResult{ExitStatus: <-exit}, nil
								}
							})

							AfterEach(func() {
								close(processExit)
							})

							It("did not check if the user is admin", func() {
								Expect(fakeAccess.IsAdminCallCount()).To(Equal(0))
							})

							It("hijacks the build", func() {
								waitForHijack()

								_, lookedUpTeamID, lookedUpHandle := fakeWorkerPool.LocateContainerArgsForCall(0)
								Expect(lookedUpTeamID).To(Equal(734))
								Expect(lookedUpHandle).To(Equal(handle))
							})

							It("updates the last hijack value", func() {
								waitForHijack()

								Expect(container.DBContainer_.UpdateLastHijackCallCount()).To(Equal(1))
							})

							Context("when the hijack timer elapses", func() {
								JustBeforeEach(func() {
									fakeClock.WaitForWatcherAndIncrement(time.Second)
								})

								It("updates the last hijack value again", func() {
									Eventually(container.DBContainer_.UpdateLastHijackCallCount).Should(Equal(2))
								})
							})

							Context("when stdin is sent over the API", func() {
								JustBeforeEach(func() {
									err := conn.WriteJSON(atc.HijackInput{
										Stdin: []byte("some stdin\n"),
									})
									Expect(err).NotTo(HaveOccurred())
								})

								It("forwards the payload to the process", func() {
									process := waitForHijack()

									receivedStdin, err := bufio.NewReader(process.Stdin()).ReadBytes('\n')
									Expect(err).NotTo(HaveOccurred())
									Expect(receivedStdin).To(Equal([]byte("some stdin\n")))

									Expect(interceptTimeout.ResetCallCount()).To(Equal(1))
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
									process := waitForHijack()

									_, err := process.Stdin().Read(make([]byte, 10))
									Expect(err).To(Equal(io.EOF))
								})
							})

							Context("when the process prints to stdout", func() {
								JustBeforeEach(func() {
									process := waitForHijack()
									_, err := fmt.Fprintf(process.Stdout(), "some stdout\n")
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
									process := waitForHijack()

									_, err := fmt.Fprintf(process.Stderr(), "some stderr\n")
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

								It("closes the process' stdin pipe", func() {
									process := waitForHijack()

									c := make(chan bool, 1)

									go func() {
										var b []byte
										_, err := process.Stdin().Read(b)
										if err != nil {
											c <- true
										}
									}()

									Eventually(c, 2*time.Second).Should(Receive())
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
									process := waitForHijack()

									Eventually(process.TTY).Should(Equal(&runtime.TTYSpec{
										WindowSize: runtime.WindowSize{
											Columns: 123,
											Rows:    456,
										},
									}))
								})
							})

							Context("when waiting on the process fails", func() {
								BeforeEach(func() {
									container.ProcessDefs[0].Stub.Call = nil
									container.ProcessDefs[0].Stub.Err = "oh no!"
								})

								It("forwards the error to the response", func() {
									waitForHijack()

									var hijackOutput atc.HijackOutput
									err := conn.ReadJSON(&hijackOutput)
									Expect(err).NotTo(HaveOccurred())

									Expect(hijackOutput).To(Equal(atc.HijackOutput{
										Error: "oh no!",
									}))
								})
							})

							Context("when intercept timeout channel sends a value", func() {
								var (
									interceptTimeoutChannel chan time.Time
								)

								BeforeEach(func() {
									interceptTimeoutChannel = make(chan time.Time)
									interceptTimeout.ChannelReturns(interceptTimeoutChannel)
								})

								It("exits with timeout error", func() {
									interceptTimeout.ErrorReturns(errors.New("too slow"))
									interceptTimeoutChannel <- time.Time{}

									var hijackOutput atc.HijackOutput
									err := conn.ReadJSON(&hijackOutput)
									Expect(err).NotTo(HaveOccurred())

									Expect(hijackOutput.Error).To(Equal("too slow"))
								})
							})
						})
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				expectBadHandshake = true

				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/containers/destroying", func() {
		BeforeEach(func() {
			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers/destroying", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")

			fakeAccess.IsAuthenticatedReturns(true)
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not attempt to find the worker", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(BeZero())
			})
		})

		Context("when authenticated as system", func() {
			BeforeEach(func() {
				fakeAccess.IsSystemReturns(true)
			})

			Context("with no params", func() {
				It("returns 400 Bad Request", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeContainerRepository.FindDestroyingContainersCallCount()).To(Equal(0))
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("querying with worker name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"worker_name": []string{"some-worker-name"},
					}.Encode()
				})

				Context("when there is an error", func() {
					BeforeEach(func() {
						fakeContainerRepository.FindDestroyingContainersReturns(nil, errors.New("some error"))
					})

					It("returns 500", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when no containers are found", func() {
					BeforeEach(func() {
						fakeContainerRepository.FindDestroyingContainersReturns([]string{}, nil)
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

					Context("when containers are found", func() {
						BeforeEach(func() {
							fakeContainerRepository.FindDestroyingContainersReturns([]string{
								"handle1",
								"handle2",
							}, nil)
						})
						It("returns container handles array", func() {
							response, err := client.Do(req)
							Expect(err).NotTo(HaveOccurred())

							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`
								["handle1", "handle2"]
							`))
						})
					})
				})

				It("queries with it in the worker name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeContainerRepository.FindDestroyingContainersCallCount()).To(Equal(1))

					workerName := fakeContainerRepository.FindDestroyingContainersArgsForCall(0)
					Expect(workerName).To(Equal("some-worker-name"))
				})
			})
		})
	})

	Describe("PUT /api/v1/containers/report", func() {
		var response *http.Response
		var body io.Reader
		var err error

		BeforeEach(func() {
			body = bytes.NewBufferString(`
				[
					"handle1",
					"handle2"
				]
			`)
		})

		JustBeforeEach(func() {
			req, err = http.NewRequest("PUT", server.URL+"/api/v1/containers/report", body)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err = client.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated as system", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsSystemReturns(true)
			})

			Context("with no params", func() {
				It("returns 404", func() {
					response, err = client.Do(req)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeDestroyer.DestroyContainersCallCount()).To(Equal(0))
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})

				It("returns Content-Type application/json", func() {
					response, err = client.Do(req)
					Expect(err).NotTo(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})
			})

			Context("querying with worker name", func() {
				JustBeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"worker_name": []string{"some-worker-name"},
					}.Encode()
				})

				Context("with invalid json", func() {
					BeforeEach(func() {
						body = bytes.NewBufferString(`{}`)
					})

					It("returns 400", func() {
						response, err = client.Do(req)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("when there is an error", func() {
					BeforeEach(func() {
						fakeDestroyer.DestroyContainersReturns(errors.New("some error"))
					})

					It("returns 500", func() {
						response, err = client.Do(req)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when containers are destroyed", func() {
					BeforeEach(func() {
						fakeDestroyer.DestroyContainersReturns(nil)
					})

					It("returns 204", func() {
						response, err = client.Do(req)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.StatusCode).To(Equal(http.StatusNoContent))
					})
				})

				It("queries with it in the worker name", func() {
					_, err = client.Do(req)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeDestroyer.DestroyContainersCallCount()).To(Equal(1))

					workerName, handles := fakeDestroyer.DestroyContainersArgsForCall(0)
					Expect(workerName).To(Equal("some-worker-name"))
					Expect(handles).To(Equal([]string{"handle1", "handle2"}))
				})

				It("marks containers as missing", func() {
					_, err = client.Do(req)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeContainerRepository.UpdateContainersMissingSinceCallCount()).To(Equal(1))

					workerName, handles := fakeContainerRepository.UpdateContainersMissingSinceArgsForCall(0)
					Expect(workerName).To(Equal("some-worker-name"))
					Expect(handles).To(Equal([]string{"handle1", "handle2"}))
				})
			})
		})
	})
})
