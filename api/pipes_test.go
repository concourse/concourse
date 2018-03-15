package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"

	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Pipes API", func() {
	var fakeaccess *accessorfakes.FakeAccess

	createPipe := func() atc.Pipe {
		fakeAccessor.CreateReturns(fakeaccess)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipes", nil)
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
		fakeAccessor.CreateReturns(fakeaccess)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipes", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())

		Expect(response.StatusCode).To(Equal(statusCode))
	}

	readPipe := func(id string) *http.Response {
		fakeAccessor.CreateReturns(fakeaccess)
		response, err := http.Get(server.URL + "/api/v1/teams/a-team/pipes/" + id)
		Expect(err).NotTo(HaveOccurred())

		return response
	}

	writePipe := func(id string, body io.Reader) *http.Response {
		fakeAccessor.CreateReturns(fakeaccess)
		req, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipes/"+id, body)
		Expect(err).NotTo(HaveOccurred())

		response, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())

		return response
	}

	BeforeEach(func() {
		fakeaccess = new(accessorfakes.FakeAccess)
	})

	Context("when authenticated", func() {
		BeforeEach(func() {
			fakeaccess.IsAuthenticatedReturns(true)
			fakeaccess.IsAuthorizedReturns(true)
		})

		Describe("POST /api/v1/teams/a-team/pipes", func() {
			Context("when team not found", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, nil)
				})
				It("returns 404", func() {
					createPipeWithError(http.StatusNotFound)
				})
			})

			Context("when team is found", func() {
				var pipe atc.Pipe

				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
					pipe = createPipe()
					dbTeam.GetPipeReturns(db.Pipe{
						ID:       pipe.ID,
						URL:      peerURL,
						TeamName: "a-team",
					}, nil)
				})

				It("returns unique pipe IDs", func() {
					anotherPipe := createPipe()
					Expect(anotherPipe.ID).NotTo(Equal(pipe.ID))
				})

				It("returns the pipe's read/write URLs", func() {
					Expect(pipe.ReadURL).To(Equal(fmt.Sprintf("https://example.com/api/v1/teams/a-team/pipes/%s", pipe.ID)))
					Expect(pipe.WriteURL).To(Equal(fmt.Sprintf("https://example.com/api/v1/teams/a-team/pipes/%s", pipe.ID)))
				})

				It("saves it", func() {
					teamName := dbTeamFactory.FindTeamArgsForCall(0)
					Expect(teamName).To(Equal("a-team"))
					Expect(dbTeam.CreatePipeCallCount()).To(Equal(1))
				})

				Describe("GET /api/v1/teams/a-team/pipes/:pipe", func() {
					var readRes *http.Response
					Context("when not authorized", func() {
						BeforeEach(func() {
							pipe := createPipe()
							fakeaccess.IsAuthorizedReturns(false)
							readRes = readPipe(pipe.ID)
						})
						It("returns 401 Unauthorized", func() {
							Expect(readRes.StatusCode).To(Equal(http.StatusUnauthorized))
						})
					})

					Context("when team not found", func() {
						BeforeEach(func() {
							pipe := createPipe()
							dbTeamFactory.FindTeamReturns(nil, false, nil)
							readRes = readPipe(pipe.ID)
						})
						It("returns 401", func() {
							Expect(readRes.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when authorized", func() {
						BeforeEach(func() {
							fakeaccess.IsAuthorizedReturns(true)
							readRes = readPipe(pipe.ID)
						})

						AfterEach(func() {
							_ = readRes.Body.Close()
						})

						It("responds with 200", func() {
							Expect(readRes.StatusCode).To(Equal(http.StatusOK))
						})

						Describe("PUT /api/v1/teams/a-team/pipes/:pipe", func() {
							var writeRes *http.Response
							Context("when not authorized", func() {
								BeforeEach(func() {
									fakeaccess.IsAuthorizedReturns(false)
									writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"))
								})
								It("returns 403 Forbidden", func() {
									Expect(writeRes.StatusCode).To(Equal(http.StatusForbidden))
								})
							})

							Context("when team not found", func() {
								BeforeEach(func() {
									fakeaccess.IsAuthorizedReturns(true)
									dbTeamFactory.FindTeamReturns(nil, false, nil)
									writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"))
								})
								It("returns 500", func() {
									Expect(writeRes.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})

							Context("when authorized", func() {
								BeforeEach(func() {
									fakeaccess.IsAuthorizedReturns(true)
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
							TeamName: "a-team",
						}, nil)
					})

					Context("when the other ATC returns 200", func() {
						BeforeEach(func() {
							otherATCServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/teams/a-team/pipes/some-guid"),
									ghttp.VerifyHeaderKV("Connection", "close"),
									ghttp.RespondWith(200, "hello from the other side"),
								),
							)
						})

						It("forwards request to that ATC with disabled keep-alive", func() {
							req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipes/some-guid", nil)
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
							req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipes/some-guid", nil)
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
			fakeaccess.IsAuthenticatedReturns(false)
		})
		JustBeforeEach(func() {
			fakeAccessor.CreateReturns(fakeaccess)
		})

		Describe("POST /api/v1/teams/a-team/pipes", func() {
			var response *http.Response

			JustBeforeEach(func() {
				req, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipes", nil)
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

		Describe("GET /api/v1/teams/a-team/pipes/:pipe", func() {
			var response *http.Response

			JustBeforeEach(func() {
				req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipes/some-guid", nil)
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
