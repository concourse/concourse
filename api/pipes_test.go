package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipes API", func() {
	createPipe := func() atc.Pipe {
		req, err := http.NewRequest("POST", server.URL+"/api/v1/pipes", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())

		Expect(response.StatusCode).To(Equal(http.StatusCreated))

		var pipe atc.Pipe
		err = json.NewDecoder(response.Body).Decode(&pipe)
		Expect(err).NotTo(HaveOccurred())

		return pipe
	}

	createPipeWithError := func(statusCode int) {
		req, err := http.NewRequest("POST", server.URL+"/api/v1/pipes", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())

		Expect(response.StatusCode).To(Equal(statusCode))
	}

	readPipe := func(id string) *http.Response {
		response, err := http.Get(server.URL + "/api/v1/pipes/" + id)
		Expect(err).NotTo(HaveOccurred())

		return response
	}

	writePipe := func(id string, body io.Reader) *http.Response {
		req, err := http.NewRequest("PUT", server.URL+"/api/v1/pipes/"+id, body)
		Expect(err).NotTo(HaveOccurred())

		response, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())

		return response
	}

	Context("when authenticated", func() {
		BeforeEach(func() {
			jwtValidator.IsAuthenticatedReturns(true)
		})

		Describe("POST /api/v1/pipes", func() {
			Context("when team not found", func() {
				BeforeEach(func() {
					userContextReader.GetTeamReturns("", false, false)
				})
				It("returns 500", func() {
					createPipeWithError(http.StatusInternalServerError)
				})
			})

			Context("when team is found", func() {
				BeforeEach(func() {
					jwtValidator.IsAuthenticatedReturns(true)
				})

				var pipe atc.Pipe

				BeforeEach(func() {
					userContextReader.GetTeamReturns("team1", false, true)
					pipe = createPipe()
					dbTeam.GetPipeReturns(db.Pipe{
						ID:       pipe.ID,
						URL:      peerAddr,
						TeamName: "team1",
					}, nil)
				})

				It("returns unique pipe IDs", func() {
					anotherPipe := createPipe()
					Expect(anotherPipe.ID).NotTo(Equal(pipe.ID))
				})

				It("returns the pipe's read/write URLs", func() {
					Expect(pipe.ReadURL).To(Equal(fmt.Sprintf("https://example.com/api/v1/pipes/%s", pipe.ID)))
					Expect(pipe.WriteURL).To(Equal(fmt.Sprintf("https://example.com/api/v1/pipes/%s", pipe.ID)))
				})

				It("saves it", func() {
					teamName := dbTeamFactory.FindTeamArgsForCall(0)
					Expect(teamName).To(Equal("team1"))
					Expect(dbTeam.CreatePipeCallCount()).To(Equal(1))
				})

				Describe("GET /api/v1/pipes/:pipe", func() {
					var readRes *http.Response
					Context("when not authorized", func() {
						BeforeEach(func() {
							userContextReader.GetTeamReturns("team", false, true)
							pipe := createPipe()
							userContextReader.GetTeamReturns("another-team", false, true)
							readRes = readPipe(pipe.ID)
						})
						It("returns 403 Forbidden", func() {
							Expect(readRes.StatusCode).To(Equal(http.StatusForbidden))
						})
					})

					Context("when team not found", func() {
						BeforeEach(func() {
							userContextReader.GetTeamReturns("team", false, true)
							pipe := createPipe()
							userContextReader.GetTeamReturns("", false, false)
							readRes = readPipe(pipe.ID)
						})
						It("returns 500", func() {
							Expect(readRes.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when authorized", func() {
						BeforeEach(func() {
							readRes = readPipe(pipe.ID)
						})

						AfterEach(func() {
							_ = readRes.Body.Close()
						})

						It("responds with 200", func() {
							Expect(readRes.StatusCode).To(Equal(http.StatusOK))
						})

						Describe("PUT /api/v1/pipes/:pipe", func() {
							var writeRes *http.Response
							Context("when not authorized", func() {
								BeforeEach(func() {
									userContextReader.GetTeamReturns("another-team", false, true)
									writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"))
								})
								It("returns 403 Forbidden", func() {
									Expect(writeRes.StatusCode).To(Equal(http.StatusForbidden))
								})
							})

							Context("when team not found", func() {
								BeforeEach(func() {
									userContextReader.GetTeamReturns("", false, false)
									writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"))
								})
								It("returns 500", func() {
									Expect(writeRes.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})

							Context("when authorized", func() {
								BeforeEach(func() {
									userContextReader.GetTeamReturns("team1", false, true)
									writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"))
								})

								AfterEach(func() {
									_ = writeRes.Body.Close()
								})

								It("responds with 200", func() {
									Expect(writeRes.StatusCode).To(Equal(http.StatusOK))
								})

								It("streams the data to the reader", func() {
									Expect(ioutil.ReadAll(readRes.Body)).To(Equal([]byte("some data")))
								})

								It("reaps the pipe", func() {
									Eventually(func() int {
										secondReadRes := readPipe(pipe.ID)
										defer db.Close(secondReadRes.Body)

										return secondReadRes.StatusCode
									}).Should(Equal(http.StatusNotFound))
								})
							})
						})

						Context("when the reader disconnects", func() {
							BeforeEach(func() {
								_ = readRes.Body.Close()
							})

							It("reaps the pipe", func() {
								Eventually(func() int {
									secondReadRes := readPipe(pipe.ID)
									defer db.Close(secondReadRes.Body)

									return secondReadRes.StatusCode
								}).Should(Equal(http.StatusNotFound))
							})
						})
					})
				})

				Describe("with an invalid id", func() {
					It("returns 404", func() {
						readRes := readPipe("bogus-id")
						defer db.Close(readRes.Body)

						Expect(readRes.StatusCode).To(Equal(http.StatusNotFound))

						writeRes := writePipe("bogus-id", nil)
						defer db.Close(writeRes.Body)

						Expect(writeRes.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when pipe was created on another ATC", func() {
					var otherATCServer *ghttp.Server

					BeforeEach(func() {
						otherATCServer = ghttp.NewServer()

						dbTeam.GetPipeReturns(db.Pipe{
							ID:       "some-guid",
							URL:      otherATCServer.URL(),
							TeamName: "team1",
						}, nil)
					})

					Context("when the other ATC returns 200", func() {
						BeforeEach(func() {
							otherATCServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/pipes/some-guid"),
									ghttp.VerifyHeaderKV("Connection", "close"),
									ghttp.RespondWith(200, "hello from the other side"),
								),
							)
						})

						It("forwards request to that ATC with disabled keep-alive", func() {
							req, err := http.NewRequest("GET", server.URL+"/api/v1/pipes/some-guid", nil)
							Expect(err).NotTo(HaveOccurred())

							response, err := client.Do(req)
							Expect(err).NotTo(HaveOccurred())

							Expect(otherATCServer.ReceivedRequests()).To(HaveLen(1))

							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("hello from the other side")))
						})
					})

					Context("when the other ATC returns a bad status code", func() {
						BeforeEach(func() {
							otherATCServer.AppendHandlers(ghttp.RespondWith(403, "nope"))
						})

						It("returns the same status code", func() {
							req, err := http.NewRequest("GET", server.URL+"/api/v1/pipes/some-guid", nil)
							Expect(err).NotTo(HaveOccurred())

							response, err := client.Do(req)
							Expect(err).NotTo(HaveOccurred())

							Expect(otherATCServer.ReceivedRequests()).To(HaveLen(1))

							Expect(response.StatusCode).To(Equal(http.StatusForbidden))
						})
					})
				})
			})
		})
	})

	Context("when not authenticated", func() {
		BeforeEach(func() {
			jwtValidator.IsAuthenticatedReturns(false)
		})

		Describe("POST /api/v1/pipes", func() {
			var response *http.Response

			BeforeEach(func() {
				req, err := http.NewRequest("POST", server.URL+"/api/v1/pipes", nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(req)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				_ = response.Body.Close()
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Describe("GET /api/v1/pipes/:pipe", func() {
			var response *http.Response

			BeforeEach(func() {
				req, err := http.NewRequest("GET", server.URL+"/api/v1/pipes/some-guid", nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(req)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				_ = response.Body.Close()
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
