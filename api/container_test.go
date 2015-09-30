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

const (
	pipelineName1 = "pipeline-1"
	type1         = db.ContainerTypeCheck
	name1         = "name-1"
	buildID1      = 1234
	containerID1  = "dh93mvi"
)

var _ = Describe("Pipelines API", func() {
	var (
		req *http.Request

		fakeContainer1              db.ContainerInfo
		expectedPresentedContainer1 atc.Container
	)

	BeforeEach(func() {
		fakeContainer1 = db.ContainerInfo{
			Handle:       containerID1,
			PipelineName: pipelineName1,
			Type:         type1,
			Name:         name1,
			BuildID:      buildID1,
		}

		expectedPresentedContainer1 = atc.Container{
			ID:           containerID1,
			PipelineName: pipelineName1,
			Type:         type1.ToString(),
			Name:         name1,
			BuildID:      buildID1,
		}
	})

	Describe("GET /api/v1/containers", func() {
		BeforeEach(func() {
			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers", nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			var (
				fakeContainer2 db.ContainerInfo

				expectedPresentedContainer2 atc.Container
			)

			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)

				fakeContainer1 = db.ContainerInfo{
					Handle:       containerID1,
					PipelineName: pipelineName1,
					Type:         type1,
					Name:         name1,
					BuildID:      buildID1,
				}

				fakeContainer2 = db.ContainerInfo{
					Handle:       "cfvwser",
					PipelineName: "pipeline-2",
					Type:         db.ContainerTypePut,
					Name:         "name-2",
					BuildID:      4321,
				}

				expectedPresentedContainer2 = atc.Container{
					ID:           "cfvwser",
					PipelineName: "pipeline-2",
					Type:         db.ContainerTypePut.ToString(),
					Name:         "name-2",
					BuildID:      4321,
				}
			})

			Context("with no params", func() {

				Context("when no errors are returned", func() {
					var (
						fakeContainers              []db.ContainerInfo
						expectedPresentedContainers []atc.Container
					)
					BeforeEach(func() {
						fakeContainers = []db.ContainerInfo{
							fakeContainer1,
							fakeContainer2,
						}
						expectedPresentedContainers = []atc.Container{
							expectedPresentedContainer1,
							expectedPresentedContainer2,
						}
						containerDB.FindContainerInfosByIdentifierReturns(fakeContainers, true, nil)
					})

					It("returns 200", func() {
						response, err := client.Do(req)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response.StatusCode).Should(Equal(http.StatusOK))
					})

					It("returns Content-Type application/json", func() {
						response, err := client.Do(req)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response.Header.Get("Content-Type")).Should(Equal("application/json"))
					})

					It("returns all containers", func() {
						response, err := client.Do(req)
						Ω(err).ShouldNot(HaveOccurred())

						b, err := ioutil.ReadAll(response.Body)
						Ω(err).ShouldNot(HaveOccurred())

						var returned []atc.Container
						err = json.Unmarshal(b, &returned)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(len(returned)).To(Equal(len(expectedPresentedContainers)))
						for i, _ := range returned {
							expected := expectedPresentedContainers[i]
							actual := returned[i]

							Ω(actual.PipelineName).To(Equal(expected.PipelineName))
							Ω(actual.Type).To(Equal(expected.Type))
							Ω(actual.Name).To(Equal(expected.Name))
							Ω(actual.BuildID).To(Equal(expected.BuildID))
							Ω(actual.ID).To(Equal(expected.ID))
						}
					})
				})

				Context("when no containers are found", func() {
					BeforeEach(func() {
						containerDB.FindContainerInfosByIdentifierReturns([]db.ContainerInfo{}, false, nil)
					})

					It("returns 404", func() {
						response, err := client.Do(req)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
					})
				})

				Context("when there is an error", func() {
					var (
						expectedErr error
					)

					BeforeEach(func() {
						expectedErr = errors.New("some error")
						containerDB.FindContainerInfosByIdentifierReturns([]db.ContainerInfo{}, false, expectedErr)
					})

					It("returns 500", func() {
						response, err := client.Do(req)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
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
					Ω(err).ShouldNot(HaveOccurred())

					expectedArgs := db.ContainerIdentifier{
						PipelineName: pipelineName1,
					}
					Ω(containerDB.FindContainerInfosByIdentifierCallCount()).Should(Equal(1))
					Ω(containerDB.FindContainerInfosByIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
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
					Ω(err).ShouldNot(HaveOccurred())

					expectedArgs := db.ContainerIdentifier{
						Type: type1,
					}
					Ω(containerDB.FindContainerInfosByIdentifierCallCount()).Should(Equal(1))
					Ω(containerDB.FindContainerInfosByIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
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
					Ω(err).ShouldNot(HaveOccurred())

					expectedArgs := db.ContainerIdentifier{
						Name: name1,
					}
					Ω(containerDB.FindContainerInfosByIdentifierCallCount()).Should(Equal(1))
					Ω(containerDB.FindContainerInfosByIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
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
						Ω(err).ShouldNot(HaveOccurred())

						expectedArgs := db.ContainerIdentifier{
							BuildID: buildID1,
						}
						Ω(containerDB.FindContainerInfosByIdentifierCallCount()).Should(Equal(1))
						Ω(containerDB.FindContainerInfosByIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
					})

					Context("when the buildID fails to be parsed as an int", func() {
						BeforeEach(func() {
							req.URL.RawQuery = url.Values{
								"build-id": []string{"not-an-int"},
							}.Encode()
						})

						It("returns 400 Bad Request", func() {
							response, _ := client.Do(req)
							Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
						})

						It("does not lookup containers", func() {
							client.Do(req)

							Ω(containerDB.FindContainerInfosByIdentifierCallCount()).Should(Equal(0))
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
			containerDB.GetContainerInfoReturns(fakeContainer1, true, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers/"+containerID, nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when the container is not found", func() {
				BeforeEach(func() {
					containerDB.GetContainerInfoReturns(db.ContainerInfo{}, false, nil)
				})

				It("returns 404 Not Found", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
				})
			})

			Context("when the container is found", func() {
				BeforeEach(func() {
					containerDB.GetContainerInfoReturns(fakeContainer1, true, nil)
				})

				It("returns 200 OK", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("returns Content-Type application/json", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Header.Get("Content-Type")).Should(Equal("application/json"))
				})

				It("performs lookup by id", func() {
					_, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(containerDB.GetContainerInfoCallCount()).Should(Equal(1))
					Ω(containerDB.GetContainerInfoArgsForCall(0)).Should(Equal(containerID))
				})

				It("returns the container", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					var actual atc.Container
					err = json.Unmarshal(b, &actual)
					Ω(err).ShouldNot(HaveOccurred())

					expected := expectedPresentedContainer1

					Ω(actual.PipelineName).To(Equal(expected.PipelineName))
					Ω(actual.Type).To(Equal(expected.Type))
					Ω(actual.Name).To(Equal(expected.Name))
					Ω(actual.BuildID).To(Equal(expected.BuildID))
					Ω(actual.ID).To(Equal(expected.ID))
				})

			})
			Context("when there is an error", func() {
				var (
					expectedErr error
				)

				BeforeEach(func() {
					expectedErr = errors.New("some error")
					containerDB.GetContainerInfoReturns(db.ContainerInfo{}, false, expectedErr)
				})

				It("returns 500", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
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
			Ω(err).ShouldNot(HaveOccurred())

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

			Context("and the worker client returns a container", func() {
				var (
					fakeContainerInfo db.ContainerInfo
					fakeContainer     *workerfakes.FakeContainer
				)

				BeforeEach(func() {
					fakeContainerInfo = db.ContainerInfo{}
					containerDB.GetContainerInfoReturns(fakeContainerInfo, true, nil)

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
						Ω(lookedUpID).Should(Equal(containerID1))

						spec, io := fakeContainer.RunArgsForCall(0)
						Ω(spec).Should(Equal(garden.ProcessSpec{
							Path: "ls",
							User: "root",
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
							_, io := fakeContainer.RunArgsForCall(0)
							Ω(bufio.NewReader(io.Stdin).ReadBytes('\n')).Should(Equal([]byte("some stdin\n")))
						})
					})

					Context("when the process prints to stdout", func() {
						JustBeforeEach(func() {
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)

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
							Eventually(fakeContainer.RunCallCount).Should(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)

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
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("forwards it to the process", func() {
							Eventually(fakeProcess.SetTTYCallCount).Should(Equal(1))

							Ω(fakeProcess.SetTTYArgsForCall(0)).Should(Equal(garden.TTYSpec{
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

			Context("when the container cannot be found", func() {
				BeforeEach(func() {
					containerDB.GetContainerInfoReturns(db.ContainerInfo{}, false, nil)
				})

				It("returns 404 Not Found", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
					Ω(fakeWorkerClient.LookupContainerCallCount()).Should(Equal(0))
				})
			})

			Context("when the db request fails", func() {
				BeforeEach(func() {
					fakeErr := errors.New("error")
					containerDB.GetContainerInfoReturns(db.ContainerInfo{}, false, fakeErr)
				})
				It("returns 500 internal error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})

			})

			Context("when the request payload is invalid", func() {
				BeforeEach(func() {
					requestPayload = "ß"
				})

				It("returns 400 Bad Request", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not hijack the build", func() {
				Ω(fakeEngine.LookupBuildCallCount()).Should(BeZero())
			})
		})
	})
})
