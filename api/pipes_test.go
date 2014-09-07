package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/api/pipes"
)

var _ = Describe("Pipes API", func() {
	createPipe := func() pipes.Pipe {
		req, err := http.NewRequest("POST", server.URL+"/api/v1/pipes", nil)
		Ω(err).ShouldNot(HaveOccurred())

		response, err := client.Do(req)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(response.StatusCode).Should(Equal(http.StatusCreated))

		var pipe pipes.Pipe
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
		var pipe pipes.Pipe

		JustBeforeEach(func() {
			pipe = createPipe()
		})

		It("returns the server's configured peer addr", func() {
			Ω(pipe.PeerAddr).Should(Equal("127.0.0.1:1234"))
		})

		It("returns unique pipe IDs", func() {
			anotherPipe := createPipe()
			Ω(anotherPipe.ID).ShouldNot(Equal(pipe.ID))
		})

		Describe("PUT & GET /api/v1/pipes/:pipe", func() {
			It("pipes the PUTed data to the GET request, and reaps the pipe", func() {
				writeResCh := make(chan *http.Response)

				go func() {
					defer GinkgoRecover()
					writeResCh <- writePipe(pipe.ID, bytes.NewBufferString("some data"))
				}()

				readRes := readPipe(pipe.ID)
				Ω(readRes.StatusCode).Should(Equal(http.StatusOK))

				data, err := ioutil.ReadAll(readRes.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(string(data)).Should(Equal("some data"))

				var writeRes *http.Response
				Eventually(writeResCh).Should(Receive(&writeRes))
				Ω(writeRes.StatusCode).Should(Equal(http.StatusOK))

				secondReadRes := readPipe(pipe.ID)
				Ω(secondReadRes.StatusCode).Should(Equal(http.StatusNotFound))
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
