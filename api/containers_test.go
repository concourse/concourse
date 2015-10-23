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
	"strconv"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	workerfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {
	var (
		pipelineName1 = "pipeline-1"
		type1         = db.ContainerTypeCheck
		name1         = "name-1"
		buildID1      = 1234
		containerID1  = "dh93mvi"
	)

	var (
		req *http.Request

		fakeContainer1              db.Container
		expectedPresentedContainer1 atc.Container
	)

	BeforeEach(func() {
		fakeContainer1 = db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				PipelineName: pipelineName1,
				Type:         type1,
				Name:         name1,
				BuildID:      buildID1,
			},
			Handle: containerID1,
		}

		expectedPresentedContainer1 = atc.Container{
			ID:           containerID1,
			PipelineName: pipelineName1,
			Type:         type1.String(),
			Name:         name1,
			BuildID:      buildID1,
		}
	})

	Describe("GET /api/v1/containers", func() {
		BeforeEach(func() {
			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers", nil)
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
			var (
				fakeContainer2 db.Container

				expectedPresentedContainer2 atc.Container
			)

			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)

				fakeContainer1 = db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						PipelineName: pipelineName1,
						Type:         type1,
						Name:         name1,
						BuildID:      buildID1,
					},
					Handle: containerID1,
				}

				fakeContainer2 = db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						PipelineName: "pipeline-2",
						Type:         db.ContainerTypePut,
						Name:         "name-2",
						BuildID:      4321,
					},
					Handle: "cfvwser",
				}

				expectedPresentedContainer2 = atc.Container{
					ID:           "cfvwser",
					PipelineName: "pipeline-2",
					Type:         db.ContainerTypePut.String(),
					Name:         "name-2",
					BuildID:      4321,
				}
			})

			Context("with no params", func() {

				Context("when no errors are returned", func() {
					var (
						fakeContainers              []db.Container
						expectedPresentedContainers []atc.Container
					)
					BeforeEach(func() {
						fakeContainers = []db.Container{
							fakeContainer1,
							fakeContainer2,
						}
						expectedPresentedContainers = []atc.Container{
							expectedPresentedContainer1,
							expectedPresentedContainer2,
						}
						containerDB.FindContainersByIdentifierReturns(fakeContainers, nil)
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

						b, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						var returned []atc.Container
						err = json.Unmarshal(b, &returned)
						Expect(err).NotTo(HaveOccurred())

						Expect(len(returned)).To(Equal(len(expectedPresentedContainers)))
						for i, _ := range returned {
							expected := expectedPresentedContainers[i]
							actual := returned[i]

							Expect(actual.PipelineName).To(Equal(expected.PipelineName))
							Expect(actual.Type).To(Equal(expected.Type))
							Expect(actual.Name).To(Equal(expected.Name))
							Expect(actual.BuildID).To(Equal(expected.BuildID))
							Expect(actual.ID).To(Equal(expected.ID))
						}
					})
				})

				Context("when no containers are found", func() {
					BeforeEach(func() {
						containerDB.FindContainersByIdentifierReturns([]db.Container{}, nil)
					})

					It("returns 200", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns an empty array", func() {
						response, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						b, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						var returned []atc.Container
						err = json.Unmarshal(b, &returned)
						Expect(err).NotTo(HaveOccurred())

						Expect(returned).To(BeEmpty())
					})
				})

				Context("when there is an error", func() {
					var (
						expectedErr error
					)

					BeforeEach(func() {
						expectedErr = errors.New("some error")
						containerDB.FindContainersByIdentifierReturns([]db.Container{}, expectedErr)
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
						"pipeline_name": []string{pipelineName1},
					}.Encode()
				})

				It("calls db.Containers with the queried pipeline name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.ContainerIdentifier{
						PipelineName: pipelineName1,
					}
					Expect(containerDB.FindContainersByIdentifierCallCount()).To(Equal(1))
					Expect(containerDB.FindContainersByIdentifierArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with type", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"type": []string{string(type1)},
					}.Encode()
				})

				It("calls db.Containers with the queried type", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.ContainerIdentifier{
						Type: type1,
					}
					Expect(containerDB.FindContainersByIdentifierCallCount()).To(Equal(1))
					Expect(containerDB.FindContainersByIdentifierArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with name", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"name": []string{string(name1)},
					}.Encode()
				})

				It("calls db.Containers with the queried name", func() {
					_, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					expectedArgs := db.ContainerIdentifier{
						Name: name1,
					}
					Expect(containerDB.FindContainersByIdentifierCallCount()).To(Equal(1))
					Expect(containerDB.FindContainersByIdentifierArgsForCall(0)).To(Equal(expectedArgs))
				})
			})

			Describe("querying with build-id", func() {
				Context("when the buildID can be parsed as an int", func() {
					BeforeEach(func() {
						buildID1String := strconv.Itoa(buildID1)

						req.URL.RawQuery = url.Values{
							"build-id": []string{buildID1String},
						}.Encode()
					})

					It("calls db.Containers with the queried build id", func() {
						_, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())

						expectedArgs := db.ContainerIdentifier{
							BuildID: buildID1,
						}
						Expect(containerDB.FindContainersByIdentifierCallCount()).To(Equal(1))
						Expect(containerDB.FindContainersByIdentifierArgsForCall(0)).To(Equal(expectedArgs))
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

							Expect(containerDB.FindContainersByIdentifierCallCount()).To(Equal(0))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/containers/:id", func() {
		const (
			containerID = "23sxrfu"
		)

		BeforeEach(func() {
			containerDB.GetContainerReturns(fakeContainer1, true, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers/"+containerID, nil)
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
			})

			Context("when the container is not found", func() {
				BeforeEach(func() {
					containerDB.GetContainerReturns(db.Container{}, false, nil)
				})

				It("returns 404 Not Found", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the container is found", func() {
				BeforeEach(func() {
					containerDB.GetContainerReturns(fakeContainer1, true, nil)
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

					Expect(containerDB.GetContainerCallCount()).To(Equal(1))
					Expect(containerDB.GetContainerArgsForCall(0)).To(Equal(containerID))
				})

				It("returns the container", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					var actual atc.Container
					err = json.Unmarshal(b, &actual)
					Expect(err).NotTo(HaveOccurred())

					expected := expectedPresentedContainer1

					Expect(actual.PipelineName).To(Equal(expected.PipelineName))
					Expect(actual.Type).To(Equal(expected.Type))
					Expect(actual.Name).To(Equal(expected.Name))
					Expect(actual.BuildID).To(Equal(expected.BuildID))
					Expect(actual.ID).To(Equal(expected.ID))
				})

			})
			Context("when there is an error", func() {
				var (
					expectedErr error
				)

				BeforeEach(func() {
					expectedErr = errors.New("some error")
					containerDB.GetContainerReturns(db.Container{}, false, expectedErr)
				})

				It("returns 500", func() {
					response, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("POST /api/v1/containers/:id/hijack", func() {
		var (
			requestPayload string

			response *http.Response

			clientConn   net.Conn
			clientReader *bufio.Reader

			clientEnc *json.Encoder
			clientDec *json.Decoder
		)

		BeforeEach(func() {
			requestPayload = `{"path":"ls", "user": "root"}`
		})

		JustBeforeEach(func() {
			var err error

			hijackReq, err := http.NewRequest(
				"POST",
				server.URL+"/api/v1/containers/"+containerID1+"/hijack",
				bytes.NewBufferString(requestPayload),
			)
			Expect(err).NotTo(HaveOccurred())

			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			Expect(err).NotTo(HaveOccurred())

			client := httputil.NewClientConn(conn, nil)

			response, err = client.Do(hijackReq)
			Expect(err).NotTo(HaveOccurred())

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

			Context("and the worker client returns a container", func() {
				var (
					fakeDBContainer db.Container
					fakeContainer   *workerfakes.FakeContainer
				)

				BeforeEach(func() {
					fakeDBContainer = db.Container{}
					containerDB.GetContainerReturns(fakeDBContainer, true, nil)

					fakeContainer = new(workerfakes.FakeContainer)
					fakeWorkerClient.LookupContainerReturns(fakeContainer, true, nil)
				})

				Context("when running the process succeeds", func() {
					var (
						fakeProcess *gfakes.FakeProcess
						processExit chan int
					)

					BeforeEach(func() {
						processExit = make(chan int)

						fakeProcess = new(gfakes.FakeProcess)
						fakeProcess.WaitStub = func() (int, error) {
							return <-processExit, nil
						}

						fakeContainer.RunReturns(fakeProcess, nil)
					})

					AfterEach(func() {
						close(processExit)
					})

					It("hijacks the build", func() {
						Eventually(fakeContainer.RunCallCount).Should(Equal(1))

						_, lookedUpID := fakeWorkerClient.LookupContainerArgsForCall(0)
						Expect(lookedUpID).To(Equal(containerID1))

						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "ls",
							User: "root",
						}))

						Expect(io.Stdin).NotTo(BeNil())
						Expect(io.Stdout).NotTo(BeNil())
						Expect(io.Stderr).NotTo(BeNil())
					})

					Context("when stdin is sent over the API", func() {
						JustBeforeEach(func() {
							err := clientEnc.Encode(atc.HijackInput{
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

					Context("when the process prints to stdout", func() {
						JustBeforeEach(func() {
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)

							_, err := fmt.Fprintf(io.Stdout, "some stdout\n")
							Expect(err).NotTo(HaveOccurred())
						})

						It("forwards it to the response", func() {
							var hijackOutput atc.HijackOutput
							err := clientDec.Decode(&hijackOutput)
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
							err := clientDec.Decode(&hijackOutput)
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
							err := clientDec.Decode(&hijackOutput)
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
							err := clientEnc.Encode(atc.HijackInput{
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
								err := clientDec.Decode(&hijackOutput)
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
							err := clientDec.Decode(&hijackOutput)
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
					containerDB.GetContainerReturns(db.Container{}, false, nil)
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
				})
			})

			Context("when the db request fails", func() {
				BeforeEach(func() {
					fakeErr := errors.New("error")
					containerDB.GetContainerReturns(db.Container{}, false, fakeErr)
				})
				It("returns 500 internal error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})

			})

			Context("when the request payload is invalid", func() {
				BeforeEach(func() {
					requestPayload = "ß"
				})

				It("returns 400 Bad Request", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
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
