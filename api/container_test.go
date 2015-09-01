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
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	workerfakes "github.com/concourse/atc/worker/fakes"
)

const (
	pipelineName1 = "pipeline-1"
	type1         = worker.ContainerTypeCheck
	name1         = "name-1"
	buildID1      = 1234
	containerID1  = "dh93mvi"
)

type listContainersReturn struct {
	Containers []present.PresentedContainer
	Errors     []string
}

var _ = Describe("Pipelines API", func() {
	var (
		req *http.Request

		fakeContainer1              *workerfakes.FakeContainer
		expectedPresentedContainer1 present.PresentedContainer
	)

	BeforeEach(func() {
		fakeContainer1 = &workerfakes.FakeContainer{}
		fakeContainer1.IdentifierFromPropertiesReturns(
			worker.Identifier{
				PipelineName: pipelineName1,
				Type:         type1,
				Name:         name1,
				BuildID:      buildID1,
			})

		fakeContainer1.HandleReturns(containerID1)

		expectedPresentedContainer1 = present.PresentedContainer{
			ID:           containerID1,
			PipelineName: pipelineName1,
			Type:         type1,
			Name:         name1,
			BuildID:      buildID1,
		}
	})

	Describe("GET /api/v1/containers", func() {
		var (
			fakeContainer2 *workerfakes.FakeContainer

			expectedPresentedContainer2 present.PresentedContainer
		)

		BeforeEach(func() {
			fakeContainer1 = &workerfakes.FakeContainer{}
			fakeContainer1.IdentifierFromPropertiesReturns(
				worker.Identifier{
					PipelineName: pipelineName1,
					Type:         type1,
					Name:         name1,
					BuildID:      buildID1,
				})
			fakeContainer1.HandleReturns(containerID1)

			fakeContainer2 = &workerfakes.FakeContainer{}
			fakeContainer2.IdentifierFromPropertiesReturns(
				worker.Identifier{
					PipelineName: "pipeline-2",
					Type:         worker.ContainerTypePut,
					Name:         "name-2",
					BuildID:      4321,
				})
			fakeContainer2.HandleReturns("cfvwser")

			expectedPresentedContainer2 = present.PresentedContainer{
				ID:           "cfvwser",
				PipelineName: "pipeline-2",
				Type:         worker.ContainerTypePut,
				Name:         "name-2",
				BuildID:      4321,
			}

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers", nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("with no params", func() {

			Context("when no errors are returned", func() {
				var (
					fakeContainers              []worker.Container
					expectedPresentedContainers []present.PresentedContainer
				)
				BeforeEach(func() {
					fakeContainers = []worker.Container{
						fakeContainer1,
						fakeContainer2,
					}
					expectedPresentedContainers = []present.PresentedContainer{
						expectedPresentedContainer1,
						expectedPresentedContainer2,
					}
					fakeWorkerClient.FindContainersForIdentifierReturns(fakeContainers, nil)
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

					var returned listContainersReturn
					err = json.Unmarshal(b, &returned)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(len(returned.Containers)).To(Equal(len(expectedPresentedContainers)))
					for i, _ := range returned.Containers {
						expected := expectedPresentedContainers[i]
						actual := returned.Containers[i]

						Ω(actual.PipelineName).To(Equal(expected.PipelineName))
						Ω(actual.Type).To(Equal(expected.Type))
						Ω(actual.Name).To(Equal(expected.Name))
						Ω(actual.BuildID).To(Equal(expected.BuildID))
						Ω(actual.ID).To(Equal(expected.ID))
					}
				})

				It("releases all containers", func() {
					_, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					for _, c := range fakeContainers {
						fc := c.(*workerfakes.FakeContainer)
						Ω(fc.ReleaseCallCount()).Should(Equal(1))
					}
				})
			})

			Context("when there is a MultiWorkerError and no containers are found", func() {
				var (
					fakeContainers []worker.Container
					expectedErr    worker.MultiWorkerError
				)

				BeforeEach(func() {
					expectedErr = worker.MultiWorkerError{}
					expectedErr.AddError("worker1", errors.New("worker-1-error"))
					expectedErr.AddError("worker2", errors.New("worker-2-error"))
					fakeContainers = []worker.Container{}
					fakeWorkerClient.FindContainersForIdentifierReturns(fakeContainers, expectedErr)
				})

				It("returns all the errors in the MultiWorkerError", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					var returned listContainersReturn
					err = json.Unmarshal(b, &returned)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returned.Errors).Should(ConsistOf(
						"workerName: worker1, error: worker-1-error",
						"workerName: worker2, error: worker-2-error"))
				})

				It("returns status code 500", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})

				It("returns content-type application/json", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Header.Get("Content-Type")).Should(Equal("application/json"))

				})
			})

			Context("When there is a MultiWorkerError and some containers are found", func() {
				var (
					fakeContainers              []worker.Container
					expectedPresentedContainers []present.PresentedContainer
					expectedErr                 worker.MultiWorkerError
				)

				BeforeEach(func() {
					expectedErr = worker.MultiWorkerError{}
					expectedErr.AddError("worker1", errors.New("worker-1-error"))
					fakeContainers = []worker.Container{
						fakeContainer2,
					}
					expectedPresentedContainers = []present.PresentedContainer{
						expectedPresentedContainer2,
					}
					fakeWorkerClient.FindContainersForIdentifierReturns(fakeContainers, expectedErr)
				})
				It("Returns both containers and errors", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					var returned listContainersReturn
					err = json.Unmarshal(b, &returned)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returned.Errors).Should(ConsistOf(
						"workerName: worker1, error: worker-1-error"))

					Ω(len(returned.Containers)).To(Equal(len(expectedPresentedContainers)))
					for i, _ := range returned.Containers {
						expected := expectedPresentedContainers[i]
						actual := returned.Containers[i]

						Ω(actual.PipelineName).To(Equal(expected.PipelineName))
						Ω(actual.Type).To(Equal(expected.Type))
						Ω(actual.Name).To(Equal(expected.Name))
						Ω(actual.BuildID).To(Equal(expected.BuildID))
						Ω(actual.ID).To(Equal(expected.ID))
					}
				})

				It("returns status code 500", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})

			Context("when there is an error other than ContainersNotFound", func() {
				var (
					fakeContainers []worker.Container
					expectedErr    error
				)
				BeforeEach(func() {
					expectedErr = errors.New("some error")
					fakeContainers = nil
					fakeWorkerClient.FindContainersForIdentifierReturns(fakeContainers, expectedErr)
				})

				It("returns the error in the json body", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					var returned listContainersReturn
					err = json.Unmarshal(b, &returned)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returned.Errors).Should(ConsistOf(expectedErr.Error()))
				})

				It("returns all containers", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					var returned listContainersReturn
					err = json.Unmarshal(b, &returned)
					Ω(err).ShouldNot(HaveOccurred())

					actualPresentedContainers := returned.Containers
					Ω(len(actualPresentedContainers)).To(Equal(0))
				})

			})
		})

		Describe("querying with pipeline name", func() {
			BeforeEach(func() {
				req.URL.RawQuery = url.Values{
					"pipeline": []string{pipelineName1},
				}.Encode()
			})

			It("calls FindContainersForIdentifier with the queried pipeline name", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				expectedArgs := worker.Identifier{
					PipelineName: pipelineName1,
				}
				Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
			})
		})

		Describe("querying with type", func() {
			BeforeEach(func() {
				req.URL.RawQuery = url.Values{
					"type": []string{string(type1)},
				}.Encode()
			})

			It("calls FindContainersForIdentifier with the queried type", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				expectedArgs := worker.Identifier{
					Type: type1,
				}
				Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
			})
		})

		Describe("querying with name", func() {
			BeforeEach(func() {
				req.URL.RawQuery = url.Values{
					"name": []string{string(name1)},
				}.Encode()
			})

			It("calls FindContainersForIdentifier with the queried name", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				expectedArgs := worker.Identifier{
					Name: name1,
				}
				Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
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

				It("calls FindContainersForIdentifier with the queried build id", func() {
					_, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					expectedArgs := worker.Identifier{
						BuildID: buildID1,
					}
					Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
					Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
				})
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

					Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("GET /api/v1/containers/:id", func() {
		const (
			containerID = "23sxrfu"
		)

		BeforeEach(func() {
			fakeWorkerClient.LookupContainerReturns(fakeContainer1, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers/"+containerID, nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				fakeWorkerClient.LookupContainerReturns(fakeContainer1, garden.ContainerNotFoundError{})
			})

			It("returns 404 Not Found", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})

		Context("when the container is found", func() {
			BeforeEach(func() {
				fakeWorkerClient.LookupContainerReturns(fakeContainer1, nil)
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

				Ω(fakeWorkerClient.LookupContainerCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.LookupContainerArgsForCall(0)).Should(Equal(containerID))
			})

			It("returns the container", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				b, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				var actual present.PresentedContainer
				err = json.Unmarshal(b, &actual)
				Ω(err).ShouldNot(HaveOccurred())

				expected := expectedPresentedContainer1

				Ω(actual.PipelineName).To(Equal(expected.PipelineName))
				Ω(actual.Type).To(Equal(expected.Type))
				Ω(actual.Name).To(Equal(expected.Name))
				Ω(actual.BuildID).To(Equal(expected.BuildID))
				Ω(actual.ID).To(Equal(expected.ID))
			})

			It("releases the container", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer1.ReleaseCallCount()).Should(Equal(1))
			})
		})

		Context("when a worker fails", func() {
			Context("when a container is not found", func() {
				var (
					workerName      string
					workerErrorText string
				)
				BeforeEach(func() {
					fakeErr := new(worker.MultiWorkerError)
					workerName = "worker1"
					workerErrorText = "bad things afoot"
					fakeErr.AddError(workerName, errors.New(workerErrorText))
					fakeWorkerClient.LookupContainerReturns(nil, *fakeErr)
				})
				It("returns 500 internal error", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
				It("Returns an error containing the worker name and the error message", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					body := string(b)
					Ω(body).To(ContainSubstring(workerName))
					Ω(body).To(ContainSubstring(workerErrorText))
				})
			})
			Context("when a container is found", func() {
				BeforeEach(func() {
					fakeWorkerClient.LookupContainerReturns(fakeContainer1, nil)
				})

				It("returns 200 OK", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("Returns the container", func() {
					response, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					b, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					var actual present.PresentedContainer
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
				var fakeContainer *workerfakes.FakeContainer

				BeforeEach(func() {
					fakeContainer = new(workerfakes.FakeContainer)
					fakeWorkerClient.LookupContainerReturns(fakeContainer, nil)
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

						Ω(fakeWorkerClient.LookupContainerArgsForCall(0)).Should(Equal(containerID1))

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
					fakeWorkerClient.LookupContainerReturns(nil, worker.ErrContainerNotFound)
				})

				It("returns 404 Not Found", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
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
