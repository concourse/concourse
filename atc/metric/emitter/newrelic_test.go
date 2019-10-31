package emitter_test

import (
	"compress/gzip"
	"encoding/json"
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
			State: metric.EventStateOK,
		}

		testLogger = lager.NewLogger("newrelic")

		server = NewServer()

		client = &http.Client{
			Transport: &http.Transport{},
			Timeout:   time.Minute,
		}

		server.RouteToHandler(http.MethodPost, "/", verifyFakeEvent)
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
				for i := 0; i < 3; i++ {
					testEmitter.Emit(testLogger, testEvent)
				}
				Eventually(server.ReceivedRequests).Should(HaveLen(1))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(1))
			})
			It("should write two batches to NewRelic", func() {
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
				time.Sleep(1 * time.Millisecond)
				testEmitter.Emit(testLogger, testEvent)
				Eventually(server.ReceivedRequests).Should(HaveLen(1))
				Expect(testEmitter.NewRelicBatch).To(HaveLen(0))
			})
			It("should write two batches to NewRelic", func() {
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
				NewRelicBatch:       make([]emitter.NewRelicEvent, 0),
				BatchDuration:       100 * time.Second,
				BatchSize:           1,
				LastEmitTime:        time.Now(),
				Url:                 server.URL(),
				Client:              client,
				CompressionDisabled: compressionState,
			}

			testEmitter.Emit(testLogger, testEvent)
			Eventually(server.ReceivedRequests).Should(HaveLen(1))
			request := (server.ReceivedRequests())[0]
			Expect(request.Header.Get("Content-Encoding")).To(Equal(expectedEncoding))
		},
			Entry("is enabled", false, "gzip"),
			Entry("is disabled", true, ""),
		)
	})
})

func verifyFakeEvent(writer http.ResponseWriter, request *http.Request) {
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

	Expect(len(events)).To(BeNumerically(">=", 1))

	for _, event := range events {
		Expect(event["eventType"]).To(Equal("build_started"))
		Expect(event["value"]).To(Equal(float64(1)))
		Expect(event["state"]).To(Equal("ok"))
	}

	writer.WriteHeader(http.StatusOK)
}
