package api_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI Downloads API", func() {
	var (
		response *http.Response
	)

	BeforeEach(func() {
		err := ioutil.WriteFile(filepath.Join(cliDownloadsDir, "fly-darwin-amd64.tgz"), []byte("soi soi soi"), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(cliDownloadsDir, "fly-windows-amd64.zip"), []byte("soi soi soi.notavirus.bat"), 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(cliDownloadsDir)
	})

	Describe("GET /api/v1/cli?platform=darwin&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=darwin&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns the file binary", func() {
			Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("soi soi soi")))
		})
	})

	Describe("GET /api/v1/cli?platform=windows&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=windows&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns the file binary", func() {
			Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("soi soi soi.notavirus.bat")))
		})
	})

	Describe("GET /api/v1/cli?platform=Darwin&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=Darwin&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns the file binary", func() {
			Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("soi soi soi")))
		})
	})

	Describe("GET /api/v1/cli?platform=Windows&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=Windows&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns the file binary", func() {
			Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("soi soi soi.notavirus.bat")))
		})
	})

	Describe("GET /api/v1/cli?platform=darwin&arch=../darwin/amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=darwin&arch=../darwin/amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns Bad Request", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/v1/cli?platform=../etc/passwd&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=../etc/passwd&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns Bad Request", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})
})
