package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipes API", func() {
	createPipe := func() atc.Pipe {
		req, err := http.NewRequest("POST", server.URL+"/api/v1/pipes", nil)
		Ω(err).ShouldNot(HaveOccurred())

		response, err := client.Do(req)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(response.StatusCode).Should(Equal(http.StatusCreated))

		var pipe atc.Pipe
		err = json.NewDecoder(response.Body).Decode(&pipe)
		Ω(err).ShouldNot(HaveOccurred())

		return pipe
	}

	readPipe := func(id string) *http.Response {
		response, err := http.Get(server.URL + "/api/v1/pipes/" + id)
		Ω(err).ShouldNot(HaveOccurred())

		return response
	}

	writePipe := func(id string, body io.Reader) *http.Response {
		req, err := http.NewRequest("PUT", server.URL+"/api/v1/pipes/"+id, body)
		Ω(err).ShouldNot(HaveOccurred())

		response, err := client.Do(req)
		Ω(err).ShouldNot(HaveOccurred())

		return response
	}

	Context("when authenticated", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(true)
		})

		Describe("POST /api/v1/pipes", func() {
			var pipe atc.Pipe

			BeforeEach(func() {
				pipe = createPipe()
				pipeDB.GetPipeReturns(db.Pipe{
					ID:  pipe.ID,
					URL: "127.0.0.1:1234",
				}, nil)
			})

			It("returns unique pipe IDs", func() {
				anotherPipe := createPipe()
				Ω(anotherPipe.ID).ShouldNot(Equal(pipe.ID))
			})

			It("saves it", func() {
				Ω(pipeDB.CreatePipeCallCount()).Should(Equal(1))
			})

			Describe("GET /api/v1/pipes/:pipe", func() {
				var readRes *http.Response

				BeforeEach(func() {
					readRes = readPipe(pipe.ID)
				})

				AfterEach(func() {
					readRes.Body.Close()
				})

				It("responds with 200", func() {
					Ω(readRes.StatusCode).Should(Equal(http.StatusOK))
				})

				Describe("PUT /api/v1/pipes/:pipe", func() {
					var writeRes *http.Response

					BeforeEach(func() {
						writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"))
					})

					AfterEach(func() {
						writeRes.Body.Close()
					})

					It("responds with 200", func() {
						Ω(writeRes.StatusCode).Should(Equal(http.StatusOK))
					})

					It("streams the data to the reader", func() {
						Ω(ioutil.ReadAll(readRes.Body)).Should(Equal([]byte("some data")))
					})

					It("reaps the pipe", func() {
						Eventually(func() int {
							secondReadRes := readPipe(pipe.ID)
							defer secondReadRes.Body.Close()

							return secondReadRes.StatusCode
						}).Should(Equal(http.StatusNotFound))
					})
				})

				Context("when the reader disconnects", func() {
					BeforeEach(func() {
						readRes.Body.Close()
					})

					It("reaps the pipe", func() {
						Eventually(func() int {
							secondReadRes := readPipe(pipe.ID)
							defer secondReadRes.Body.Close()

							return secondReadRes.StatusCode
						}).Should(Equal(http.StatusNotFound))
					})
				})
			})

			Describe("with an invalid id", func() {
				It("returns 404", func() {
					readRes := readPipe("bogus-id")
					defer readRes.Body.Close()

					Ω(readRes.StatusCode).Should(Equal(http.StatusNotFound))

					writeRes := writePipe("bogus-id", nil)
					defer writeRes.Body.Close()

					Ω(writeRes.StatusCode).Should(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Context("when not authenticated", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(false)
		})

		Describe("POST /api/v1/pipes", func() {
			var response *http.Response

			BeforeEach(func() {
				req, err := http.NewRequest("POST", server.URL+"/api/v1/pipes", nil)
				Ω(err).ShouldNot(HaveOccurred())

				response, err = client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				response.Body.Close()
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})

		Describe("GET /api/v1/pipes/:pipe", func() {
			var response *http.Response

			BeforeEach(func() {
				req, err := http.NewRequest("GET", server.URL+"/api/v1/pipes/some-guid", nil)
				Ω(err).ShouldNot(HaveOccurred())

				response, err = client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				response.Body.Close()
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})
})
