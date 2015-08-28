package api_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	workerfakes "github.com/concourse/atc/worker/fakes"
)

var _ = Describe("Hijacking API", func() {
	Describe("POST /api/v1/hijack/:handle", func() {
		var (
			requestPayload string
			handle         string
			buildID        string
			stepType       string
			stepName       string
			pipelineName   string

			response *http.Response

			clientConn   net.Conn
			clientReader *bufio.Reader

			clientEnc *json.Encoder
			clientDec *json.Decoder
		)

		BeforeEach(func() {
			handle = "sdfwer8"
			requestPayload = `{"path":"ls", "user": "root"}`
			buildID = "128"
			stepType = "task"
			stepName = "build"
		})

		JustBeforeEach(func() {
			var err error

			hijackReq, err := http.NewRequest(
				"POST",
				server.URL+"/api/v1/hijack/"+handle,
				bytes.NewBufferString(requestPayload),
			)
			Ω(err).ShouldNot(HaveOccurred())

			hijackReq.URL.RawQuery = url.Values{
				"build-id": []string{buildID},
				"type":     []string{stepType},
				"name":     []string{stepName},
				"pipeline": []string{pipelineName},
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

						Ω(fakeWorkerClient.LookupContainerArgsForCall(0)).Should(Equal(handle))

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
