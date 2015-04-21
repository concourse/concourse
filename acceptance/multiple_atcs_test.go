package acceptance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Multiple ATCs", func() {
	var atcOneProcess ifrit.Process
	var atcOnePort uint16

	var atcTwoProcess ifrit.Process
	var atcTwoPort uint16

	var dbListener *pq.Listener

	BeforeEach(func() {
		atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
		Ω(err).ShouldNot(HaveOccurred())

		dbLogger := lagertest.NewTestLogger("test")

		// postgresRunner.DropTestDB()
		postgresRunner.CreateTestDB()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		sqlDB = db.NewSQL(dbLogger, dbConn, dbListener)

		atcOneProcess, atcOnePort = startATC(atcBin, 1)
		atcTwoProcess, atcTwoPort = startATC(atcBin, 2)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcOneProcess)
		ginkgomon.Interrupt(atcTwoProcess)

		Ω(dbConn.Close()).Should(Succeed())
		Ω(dbListener.Close()).Should(Succeed())

		postgresRunner.DropTestDB()
	})

	Describe("Pipes", func() {

		var client *http.Client
		BeforeEach(func() {
			client = &http.Client{
				Transport: &http.Transport{},
			}
		})

		createPipe := func(atcPort uint16) atc.Pipe {
			req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/api/v1/pipes", atcPort), nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.SetBasicAuth("admin", "password")

			response, err := client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response.StatusCode).Should(Equal(http.StatusCreated))

			var pipe atc.Pipe
			err = json.NewDecoder(response.Body).Decode(&pipe)
			Ω(err).ShouldNot(HaveOccurred())

			return pipe
		}

		readPipe := func(id string, atcPort uint16) *http.Response {
			req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/pipes/%s", atcPort, id), nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.SetBasicAuth("admin", "password")
			response, err := client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())

			return response
		}

		writePipe := func(id string, body io.Reader, atcPort uint16) *http.Response {
			req, err := http.NewRequest("PUT", fmt.Sprintf("http://127.0.0.1:%d/api/v1/pipes/%s", atcPort, id), body)
			Ω(err).ShouldNot(HaveOccurred())
			req.SetBasicAuth("admin", "password")

			response, err := client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())

			return response
		}

		It("data can be written or read from the pipe regardless of where it was created", func() {
			pipe := createPipe(atcOnePort)

			readRes := readPipe(pipe.ID, atcOnePort)
			Ω(readRes.StatusCode).Should(Equal(http.StatusOK))

			writeRes := writePipe(pipe.ID, bytes.NewBufferString("some data"), atcOnePort)
			Ω(writeRes.StatusCode).Should(Equal(http.StatusOK))

			Ω(ioutil.ReadAll(readRes.Body)).Should(Equal([]byte("some data")))
			Eventually(func() int {
				secondReadRes := readPipe(pipe.ID, atcOnePort)
				defer secondReadRes.Body.Close()

				return secondReadRes.StatusCode
			}).Should(Equal(http.StatusNotFound))

			readRes.Body.Close()
			writeRes.Body.Close()

			pipe = createPipe(atcOnePort)

			readRes = readPipe(pipe.ID, atcOnePort)
			Ω(readRes.StatusCode).Should(Equal(http.StatusOK))

			writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"), atcTwoPort)
			Ω(writeRes.StatusCode).Should(Equal(http.StatusOK))

			Ω(ioutil.ReadAll(readRes.Body)).Should(Equal([]byte("some data")))
			Eventually(func() int {
				secondReadRes := readPipe(pipe.ID, atcOnePort)
				defer secondReadRes.Body.Close()

				return secondReadRes.StatusCode
			}).Should(Equal(http.StatusNotFound))

			readRes.Body.Close()
			writeRes.Body.Close()

			pipe = createPipe(atcTwoPort)
			readRes = readPipe(pipe.ID, atcOnePort)
			Ω(readRes.StatusCode).Should(Equal(http.StatusOK))

			writeRes = writePipe(pipe.ID, bytes.NewBufferString("some kind of data"), atcTwoPort)
			Ω(writeRes.StatusCode).Should(Equal(http.StatusOK))
			Ω(ioutil.ReadAll(readRes.Body)).Should(Equal([]byte("some kind of data")))

			readRes.Body.Close()
			writeRes.Body.Close()

			pipe = createPipe(atcOnePort)

			readRes = readPipe(pipe.ID, atcTwoPort)
			Ω(readRes.StatusCode).Should(Equal(http.StatusOK))

			writeRes = writePipe(pipe.ID, bytes.NewBufferString("some other data"), atcTwoPort)
			Ω(writeRes.StatusCode).Should(Equal(http.StatusOK))

			Ω(ioutil.ReadAll(readRes.Body)).Should(Equal([]byte("some other data")))

			readRes.Body.Close()
			writeRes.Body.Close()
		})
	})

})
