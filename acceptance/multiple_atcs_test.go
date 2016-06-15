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
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbConn, bus)

		atcOneProcess, atcOnePort, _ = startATC(atcBin, 1, []string{}, BASIC_AUTH)
		atcTwoProcess, atcTwoPort, _ = startATC(atcBin, 2, []string{}, BASIC_AUTH)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcOneProcess)
		ginkgomon.Interrupt(atcTwoProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
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
			Expect(err).NotTo(HaveOccurred())
			req.SetBasicAuth("admin", "password")

			response, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			var pipe atc.Pipe
			err = json.NewDecoder(response.Body).Decode(&pipe)
			Expect(err).NotTo(HaveOccurred())

			return pipe
		}

		readPipe := func(id string, atcPort uint16) *http.Response {
			req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/pipes/%s", atcPort, id), nil)
			Expect(err).NotTo(HaveOccurred())
			req.SetBasicAuth("admin", "password")
			response, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			return response
		}

		writePipe := func(id string, body io.Reader, atcPort uint16) *http.Response {
			req, err := http.NewRequest("PUT", fmt.Sprintf("http://127.0.0.1:%d/api/v1/pipes/%s", atcPort, id), body)
			Expect(err).NotTo(HaveOccurred())
			req.SetBasicAuth("admin", "password")

			response, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			return response
		}

		It("data can be written or read from the pipe regardless of where it was created", func() {
			pipe := createPipe(atcOnePort)

			readRes := readPipe(pipe.ID, atcOnePort)
			Expect(readRes.StatusCode).To(Equal(http.StatusOK))

			writeRes := writePipe(pipe.ID, bytes.NewBufferString("some data"), atcOnePort)
			Expect(writeRes.StatusCode).To(Equal(http.StatusOK))

			Expect(ioutil.ReadAll(readRes.Body)).To(Equal([]byte("some data")))
			Eventually(func() int {
				secondReadRes := readPipe(pipe.ID, atcOnePort)
				defer secondReadRes.Body.Close()

				return secondReadRes.StatusCode
			}).Should(Equal(http.StatusNotFound))

			readRes.Body.Close()
			writeRes.Body.Close()

			pipe = createPipe(atcOnePort)

			readRes = readPipe(pipe.ID, atcOnePort)
			Expect(readRes.StatusCode).To(Equal(http.StatusOK))

			writeRes = writePipe(pipe.ID, bytes.NewBufferString("some data"), atcTwoPort)
			Expect(writeRes.StatusCode).To(Equal(http.StatusOK))

			Expect(ioutil.ReadAll(readRes.Body)).To(Equal([]byte("some data")))
			Eventually(func() int {
				secondReadRes := readPipe(pipe.ID, atcOnePort)
				defer secondReadRes.Body.Close()

				return secondReadRes.StatusCode
			}).Should(Equal(http.StatusNotFound))

			readRes.Body.Close()
			writeRes.Body.Close()

			pipe = createPipe(atcTwoPort)
			readRes = readPipe(pipe.ID, atcOnePort)
			Expect(readRes.StatusCode).To(Equal(http.StatusOK))

			writeRes = writePipe(pipe.ID, bytes.NewBufferString("some kind of data"), atcTwoPort)
			Expect(writeRes.StatusCode).To(Equal(http.StatusOK))
			Expect(ioutil.ReadAll(readRes.Body)).To(Equal([]byte("some kind of data")))

			readRes.Body.Close()
			writeRes.Body.Close()

			pipe = createPipe(atcOnePort)

			readRes = readPipe(pipe.ID, atcTwoPort)
			Expect(readRes.StatusCode).To(Equal(http.StatusOK))

			writeRes = writePipe(pipe.ID, bytes.NewBufferString("some other data"), atcTwoPort)
			Expect(writeRes.StatusCode).To(Equal(http.StatusOK))

			Expect(ioutil.ReadAll(readRes.Body)).To(Equal([]byte("some other data")))

			readRes.Body.Close()
			writeRes.Body.Close()
		})
	})

})
