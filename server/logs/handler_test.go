package logs_test

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"code.google.com/p/go.net/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	Db "github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/postgresrunner"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/server"
)

var _ = Describe("API", func() {
	var postgresRunner postgresrunner.Runner

	var dbConn *sql.DB
	var dbProcess ifrit.Process

	var db Db.DB

	var testServer *httptest.Server
	var client *http.Client

	var tracker *logfanout.Tracker

	BeforeSuite(func() {
		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Envoke(postgresRunner)
	})

	AfterSuite(func() {
		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
	})

	BeforeEach(func() {
		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()
		db = Db.NewSQL(dbConn)

		tracker = logfanout.NewTracker(db)

		handler, err := server.New(
			lagertest.NewTestLogger("api"),
			config.Config{},
			&scheduler.Scheduler{},
			&radar.Radar{},
			db,
			"../templates",
			"../public",
			"",
			tracker,
		)
		Ω(err).ShouldNot(HaveOccurred())

		testServer = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		testServer.Close()

		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	Describe("GET /jobs/:job/builds/:build/logs", func() {
		var build builds.Build

		var endpoint string

		BeforeEach(func() {
			var err error

			err = db.RegisterJob("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			build, err = db.CreateBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			endpoint = fmt.Sprintf(
				"ws://%s/jobs/%s/builds/%d/log",
				testServer.Listener.Addr().String(),
				"some-job",
				build.ID,
			)
		})

		outputSink := func() *gbytes.Buffer {
			outConn, err := websocket.Dial(endpoint, "", "http://0.0.0.0")
			Ω(err).ShouldNot(HaveOccurred())

			buf := gbytes.NewBuffer()

			go func() {
				defer GinkgoRecover()

				_, err := io.Copy(buf, outConn)
				Ω(err).ShouldNot(HaveOccurred())

				err = buf.Close()
				Ω(err).ShouldNot(HaveOccurred())
			}()

			return buf
		}

		It("transmits ansi escape characters as html", func() {
			logFanout := tracker.Register("some-job", build.ID, gbytes.NewBuffer())
			defer logFanout.Close()

			sink := outputSink()

			_, err := logFanout.Write([]byte("some \x1b[1mmessage"))
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sink).Should(gbytes.Say(`some <span class="ansi-bold">message`))
		})
	})
})
