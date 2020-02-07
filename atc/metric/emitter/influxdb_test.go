package emitter_test

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/emitter"
	"github.com/concourse/concourse/atc/metric/emitter/emitterfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func batchPointAt(influxDBClient *emitterfakes.FakeInfluxDBClient, index int) string {
	if influxDBClient.WriteCallCount() == 1 {
		points := influxDBClient.WriteArgsForCall(0).Points()
		return (*(points[index])).String()
	}
	return ""
}

var _ = Describe("InfluxDBEmitter", func() {
	var (
		influxDBEmitter *emitter.InfluxDBEmitter
		influxDBClient  *emitterfakes.FakeInfluxDBClient
		testLogger      lager.Logger
	)

	BeforeEach(func() {
		testLogger = lager.NewLogger("test")

		influxDBClient = &emitterfakes.FakeInfluxDBClient{}
	})

	Context("Emit", func() {
		Context("with batch size 2", func() {
			BeforeEach(func() {
				influxDBEmitter = &emitter.InfluxDBEmitter{
					Client:        influxDBClient,
					BatchSize:     2,
					BatchDuration: 300 * time.Second,
				}
			})

			AfterEach(func() {
				influxDBEmitter.SubmitBatch(testLogger)
			})

			It("should write no batches to InfluxDB", func() {
				influxDBEmitter.Emit(testLogger, metric.Event{})
				Eventually(influxDBClient.WriteCallCount).Should(BeNumerically("==", 0))
			})

			It("should write 1 batch to InfluxDB", func() {
				for i := 0; i < 3; i++ {
					influxDBEmitter.Emit(testLogger, metric.Event{})
				}
				Eventually(influxDBClient.WriteCallCount).Should(BeNumerically("==", 1))
			})

			It("should write 2 batches to InfluxDB", func() {
				for i := 0; i < 4; i++ {
					influxDBEmitter.Emit(testLogger, metric.Event{})
				}
				Eventually(influxDBClient.WriteCallCount).Should(BeNumerically("==", 2))
			})

			It("should populate the batch points", func() {
				influxDBEmitter.Emit(testLogger, metric.Event{
					Name:  "build started",
					Value: 123,
					Attributes: map[string]string{
						"pipeline":   "test1",
						"job":        "job1",
						"build_name": "build1",
						"build_id":   "123",
						"team_name":  "team1",
					},
					Host: "localhost",
				})
				influxDBEmitter.Emit(testLogger, metric.Event{
					Name:  "build finished",
					Value: 100,
					Attributes: map[string]string{
						"pipeline":     "test2",
						"job":          "job2",
						"build_name":   "build2",
						"build_id":     "456",
						"build_status": "succeeded",
						"team_name":    "team2",
					},
					Host: "localhost",
				})

				Eventually(func() string {
					return batchPointAt(influxDBClient, 0)
				}).Should(Equal(`build\ started,build_id=123,build_name=build1,host=localhost,job=job1,pipeline=test1,team_name=team1 value=123`))

				Eventually(func() string {
					return batchPointAt(influxDBClient, 1)
				}).Should(Equal(`build\ finished,build_id=456,build_name=build2,build_status=succeeded,host=localhost,job=job2,pipeline=test2,team_name=team2 value=100`))
			})
		})

		Context("with batch duration 1 nanosecond", func() {
			BeforeEach(func() {
				influxDBEmitter = &emitter.InfluxDBEmitter{
					Client:        influxDBClient,
					BatchSize:     5000,
					BatchDuration: 1 * time.Nanosecond,
				}
			})

			AfterEach(func() {
				influxDBEmitter.SubmitBatch(testLogger)
			})

			It("should write no batches to InfluxDB", func() {
				Eventually(influxDBClient.WriteCallCount).Should(BeNumerically("==", 0))
			})

			It("should write 1 batch to InfluxDB", func() {
				influxDBEmitter.Emit(testLogger, metric.Event{})
				time.Sleep(2 * time.Nanosecond)
				Eventually(influxDBClient.WriteCallCount).Should(BeNumerically("==", 1))
			})

			It("should write 2 batches to InfluxDB", func() {
				for i := 0; i < 2; i++ {
					influxDBEmitter.Emit(testLogger, metric.Event{})
					time.Sleep(2 * time.Nanosecond)
				}
				Eventually(influxDBClient.WriteCallCount).Should(BeNumerically("==", 2))
			})
		})
	})
})
