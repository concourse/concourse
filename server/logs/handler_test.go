package logs_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"code.google.com/p/go.net/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/redisrunner"
	"github.com/concourse/atc/server"
)

var _ = Describe("API", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var testServer *httptest.Server
	var client *http.Client

	var tracker *logfanout.Tracker

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		tracker = logfanout.NewTracker(redis)

		handler, err := server.New(
			lagertest.NewTestLogger("api"),
			config.Config{},
			redis,
			"../templates",
			"../public",
			"",
			nil,
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
		redisRunner.Stop()
	})

	Describe("GET /jobs/:job/builds/:build/logs", func() {
		var build builds.Build

		var endpoint string

		BeforeEach(func() {
			var err error

			build, err = redis.CreateBuild("some-job")
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
