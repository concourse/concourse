package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
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

	Describe("POST /api/v1/pipes", func() {
		var pipe atc.Pipe

		BeforeEach(func() {
			pipe = createPipe()
		})

		It("returns the server's configured peer addr", func() {
			Ω(pipe.PeerAddr).Should(Equal("127.0.0.1:1234"))
		})

		It("returns unique pipe IDs", func() {
			anotherPipe := createPipe()
			Ω(anotherPipe.ID).ShouldNot(Equal(pipe.ID))
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
					secondReadRes := readPipe(pipe.ID)
					Ω(secondReadRes.StatusCode).Should(Equal(http.StatusNotFound))
					secondReadRes.Body.Close()
				})
			})

			Context("when the reader disconnects", func() {
				BeforeEach(func() {
					readRes.Body.Close()
				})

				It("reaps the pipe", func() {
					secondReadRes := readPipe(pipe.ID)
					Ω(secondReadRes.StatusCode).Should(Equal(http.StatusNotFound))
					secondReadRes.Body.Close()
				})
			})
		})

		Describe("with an invalid id", func() {
			It("returns 404", func() {
				readRes := readPipe("bogus-id")
				Ω(readRes.StatusCode).Should(Equal(http.StatusNotFound))

				writeRes := writePipe("bogus-id", nil)
				Ω(writeRes.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})
})
