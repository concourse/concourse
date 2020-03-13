package api_test

import (
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/concourse/concourse/atc/testhelpers"
	"github.com/concourse/go-archive/archivetest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI Downloads API", func() {
	var (
		response *http.Response
	)

	BeforeEach(func() {
		tgz := filepath.Join(cliDownloadsDir, "fly-darwin-amd64.tgz")
		zip := filepath.Join(cliDownloadsDir, "fly-windows-amd64.zip")

		unixArchive := archivetest.Archive{
			{
				Name: "some-file",
				Body: "skipped!",
			},
			{
				Name:    "fly",
				Body:    "soi soi soi",
				ModTime: time.Date(1991, time.June, 3, 5, 30, 45, 10, time.UTC),
			},
		}

		windowsArchive := archivetest.Archive{
			{
				Name: "some-file",
				Body: "skipped!",
			},
			{
				Name:    "fly.exe",
				Body:    "soi soi soi.notavirus.bat",
				ModTime: time.Date(1989, time.June, 29, 5, 30, 44, 10, time.UTC),
			},
		}

		tgzFile, err := os.Create(tgz)
		Expect(err).NotTo(HaveOccurred())

		gzWriter := gzip.NewWriter(tgzFile)

		err = unixArchive.WriteTar(gzWriter)
		Expect(err).NotTo(HaveOccurred())

		Expect(gzWriter.Close()).To(Succeed())
		Expect(tgzFile.Close()).To(Succeed())

		zipFile, err := os.Create(zip)
		Expect(err).NotTo(HaveOccurred())

		err = windowsArchive.WriteZip(zipFile)
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

		It("returns the a response with expected headers", func() {
			expectedHeaderEntries := map[string]string{
				"Content-Type":        "application/octet-stream",
				"Content-Length":      "11",
				"Content-Disposition": "attachment; filename=fly",
				"Last-Modified":       "Mon, 03 Jun 1991 05:30:45 GMT",
			}

			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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

		It("returns the a response with expected headers", func() {
			expectedHeaderEntries := map[string]string{
				"Content-Type":        "application/octet-stream",
				"Content-Length":      "25",
				"Content-Disposition": "attachment; filename=fly.exe",
				"Last-Modified":       "Thu, 29 Jun 1989 05:30:44 GMT",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
