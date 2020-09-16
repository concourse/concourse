package emitter_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/emitter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("NewRelicEmitter", func() {

	var (
		testEmitter emitter.NewRelicEmitter
		server      *Server
		client      *http.Client
		testEvent   metric.Event
		testLogger  lager.Logger
	)

	BeforeEach(func() {
		testEvent = metric.Event{
			Name:  "build started",
			Value: 1,
		}

		testLogger = lager.NewLogger("newrelic")

		server = NewServer()

		client = &http.Client{
			Transport: &http.Transport{},
			Timeout:   time.Minute,
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Context("Emits metrics", func() {
		Context("when batch size is 2", func() {
			BeforeEach(func() {
				testEmitter = emitter.NewRelicEmitter{
					NewRelicBatch: make([]emitter.NewRelicEvent, 0),
					BatchDuration: 100 * time.Second,
					BatchSize:     2,
					LastEmitTime:  time.Now(),
					Url:           server.URL(),
					Client:        client,
				}
			})
			It("should write one batch to NewRelic", func() {
				server.RouteToHandler(http.MethodPost, "/", verifyEvents(2))
				for i := 0; i < 3; i++ {
					testEmitter.Emit(testLogger, testEvent)
				}
				Eventually(server.ReceivedRequests).Should(HaveLen(1))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(1))
			})
			It("should write two batches to NewRelic", func() {
				server.RouteToHandler(http.MethodPost, "/", verifyEvents(2))
				server.RouteToHandler(http.MethodPost, "/", verifyEvents(2))
				for i := 0; i < 4; i++ {
					testEmitter.Emit(testLogger, testEvent)
				}
				Eventually(server.ReceivedRequests).Should(HaveLen(2))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(0))
			})
			It("should write no batches to NewRelic", func() {
				testEmitter.Emit(testLogger, testEvent)

				time.Sleep(500 * time.Millisecond)
				Eventually(server.ReceivedRequests).Should(HaveLen(0))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(1))
			})
		})
		Context("when batch duration is 1 millisecond", func() {
			BeforeEach(func() {
				testEmitter = emitter.NewRelicEmitter{
					NewRelicBatch: make([]emitter.NewRelicEvent, 0),
					BatchDuration: 1 * time.Millisecond,
					BatchSize:     100,
					LastEmitTime:  time.Now(),
					Url:           server.URL(),
					Client:        client,
				}
			})
			It("should write one batch to NewRelic", func() {
				server.RouteToHandler(http.MethodPost, "/", verifyEvents(1))
				time.Sleep(1 * time.Millisecond)
				testEmitter.Emit(testLogger, testEvent)
				Eventually(server.ReceivedRequests).Should(HaveLen(1))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(0))
			})
			It("should write two batches to NewRelic", func() {
				server.RouteToHandler(http.MethodPost, "/", verifyEvents(1))
				server.RouteToHandler(http.MethodPost, "/", verifyEvents(1))
				for i := 0; i < 2; i++ {
					time.Sleep(1 * time.Millisecond)
					testEmitter.Emit(testLogger, testEvent)
				}
				Eventually(server.ReceivedRequests).Should(HaveLen(2))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(0))
			})
			It("should write no batches to NewRelic", func() {
				testEmitter.Emit(testLogger, testEvent)
				Eventually(server.ReceivedRequests).Should(HaveLen(0))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(1))

			})
		})

		DescribeTable("Compression", func(compressionState bool, expectedEncoding string) {
			testEmitter = emitter.NewRelicEmitter{
				NewRelicBatch:      make([]emitter.NewRelicEvent, 0),
				BatchDuration:      100 * time.Second,
				BatchSize:          1,
				LastEmitTime:       time.Now(),
				Url:                server.URL(),
				Client:             client,
				DisableCompression: compressionState,
			}

			server.RouteToHandler(http.MethodPost, "/", verifyEvents(1))

			testEmitter.Emit(testLogger, testEvent)
			Eventually(server.ReceivedRequests).Should(HaveLen(1))
			request := (server.ReceivedRequests())[0]
			Expect(request.Header.Get("Content-Encoding")).To(Equal(expectedEncoding))
		},
			Entry("is enabled", false, "gzip"),
			Entry("is disabled", true, ""),
		)
	})

	Context("NewRelicConfiguration", func() {
		It("sends events to configured endpoint", func() {
			config := &emitter.NewRelicConfig{
				AccountID: "123456",
				APIKey:    "eu019347923874648573934074",
				Url:       server.URL(),
			}

			server.RouteToHandler(http.MethodPost, "/v1/accounts/123456/events", verifyEvents(1))

			e, _ := config.NewEmitter()
			e.Emit(testLogger, testEvent)

			newRelicEmitter := e.(*emitter.NewRelicEmitter)
			Expect(newRelicEmitter.Url).To(Equal(fmt.Sprintf("%s/v1/accounts/123456/events", server.URL())))
			Eventually(server.ReceivedRequests).Should(HaveLen(1))
		})
	})
})

func verifyEvents(expectedEvents int) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var (
			givenBody []byte
			err       error
		)

		if request.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(request.Body)
			Expect(err).To(Not(HaveOccurred()))
			givenBody, err = ioutil.ReadAll(reader)
			Expect(err).To(Not(HaveOccurred()))
		} else {
			givenBody, err = ioutil.ReadAll(request.Body)
			Expect(err).To(Not(HaveOccurred()))
		}

		var events []emitter.NewRelicEvent
		err = json.Unmarshal(givenBody, &events)
		Expect(err).To(Not(HaveOccurred()))

		Expect(len(events)).To(BeNumerically("==", expectedEvents))

		for _, event := range events {
			Expect(event["eventType"]).To(Equal("build_started"))
			Expect(event["value"]).To(Equal(float64(1)))
		}

		writer.WriteHeader(http.StatusOK)
	}
}
