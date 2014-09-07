package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/buildserver/fakes"
	"github.com/concourse/atc/builder/fakebuilder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/logfanout"
	logfakes "github.com/concourse/atc/logfanout/fakes"
)

var _ = Describe("Builds API", func() {
	var buildsDB *fakes.FakeBuildsDB
	var logDB *logfakes.FakeLogDB
	var builder *fakebuilder.FakeBuilder
	var tracker *logfanout.Tracker
	var pingInterval time.Duration

	var server *httptest.Server
	var client *http.Client

	BeforeEach(func() {
		buildsDB = new(fakes.FakeBuildsDB)
		logDB = new(logfakes.FakeLogDB)
		builder = new(fakebuilder.FakeBuilder)
		tracker = logfanout.NewTracker(logDB)
		pingInterval = 100 * time.Millisecond

		handler, err := api.NewHandler(
			lagertest.NewTestLogger("callbacks"),
			buildsDB,
			builder,
			tracker,
			pingInterval,
		)
		Ω(err).ShouldNot(HaveOccurred())

		server = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("POST /api/v1/builds", func() {
		var turbineBuild tbuilds.Build

		var response *http.Response

		BeforeEach(func() {
			turbineBuild = tbuilds.Build{
				Config: tbuilds.Config{
					Run: tbuilds.RunConfig{
						Path: "ls",
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when creating a one-off build succeeds", func() {
			BeforeEach(func() {
				buildsDB.CreateOneOffBuildReturns(builds.Build{ID: 42}, nil)
			})

			Context("and building succeeds", func() {
				It("returns 201 Created", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusCreated))
				})

				It("returns the build ID", func() {
					var build builds.Build
					err := json.NewDecoder(response.Body).Decode(&build)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(build.ID).ShouldNot(BeZero())
				})

				It("executes a one-off build", func() {
					Ω(buildsDB.CreateOneOffBuildCallCount()).Should(Equal(1))

					Ω(builder.BuildCallCount()).Should(Equal(1))
					oneOff, tBuild := builder.BuildArgsForCall(0)
					Ω(oneOff).Should(Equal(builds.Build{ID: 42}))
					Ω(tBuild).Should(Equal(turbineBuild))
				})
			})

			Context("and building fails", func() {
				BeforeEach(func() {
					builder.BuildReturns(errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when creating a one-off build fails", func() {
			BeforeEach(func() {
				buildsDB.CreateOneOffBuildReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/events", func() {
		var conn *websocket.Conn

		JustBeforeEach(func() {
			var err error

			conn, _, err = websocket.DefaultDialer.Dial("ws://"+server.Listener.Addr().String()+"/api/v1/builds/128/events", nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := conn.Close()
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("emits events received for the build", func() {
			fanout := tracker.Register(128, gbytes.NewBuffer())

			sentMsg := json.RawMessage("123")
			err := fanout.WriteMessage(&sentMsg)
			Ω(err).ShouldNot(HaveOccurred())

			var receivedMsg json.RawMessage
			err = conn.ReadJSON(&receivedMsg)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(receivedMsg).Should(Equal(sentMsg))
		})

		It("continuously pings the connection", func() {
			gotPing := make(chan struct{}, 10)

			conn.SetPingHandler(func(string) error {
				gotPing <- struct{}{}
				return nil
			})

			// must be reading to see pings; try for 3 * ping interval and give up,
			// and check that we saw at least 2 pings

			conn.SetReadDeadline(time.Now().Add(3 * pingInterval))

			var receivedMsg json.RawMessage
			err := conn.ReadJSON(&receivedMsg)
			Ω(err).Should(HaveOccurred())

			Ω(gotPing).Should(Receive())
			Ω(gotPing).Should(Receive())
		})
	})
})
