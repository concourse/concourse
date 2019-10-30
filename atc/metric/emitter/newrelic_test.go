package emitter_test

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/emitter"
	"github.com/concourse/flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("newrelic metric", func() {
	OKResponse := `{"success":true,"uuid":"12345678-1234-5678-9012-123456789012"}`
	var (
		newrelicServer     *ghttp.Server
		newrelicConfig     *emitter.NewRelicConfig
		metricEmitter      metric.Emitter
		logger             lager.Logger
		compressionEnabled bool
	)
	emitSampleMetrics := func(times int, expectLenInBuffer int) {
		for i := 0; i < times; i++ {
			metricEmitter.Emit(logger, metric.Event{
				Name:  "build started",
				Value: "",
				State: metric.EventStateOK,
				Host:  "test-client-1",
				Time:  time.Now(),
			})
		}

		if newrelicEmitter, OK := metricEmitter.(*emitter.NewRelicEmitter); OK {
			Expect(newrelicEmitter.BufferPayloadSize()).To(Equal(expectLenInBuffer))
		} else {
			Fail("failed to convert the emitter to NewRelicEmitter")
		}
	}

	JustBeforeEach(func() {
		logger, _ = flag.Lager{LogLevel: "debug"}.Logger("newrelic-test")
		newrelicServer = ghttp.NewServer()
		newrelicServer.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/EVENTS"),
			ghttp.RespondWithJSONEncoded(200, OKResponse),
		))
		newrelicConfig = &emitter.NewRelicConfig{
			AccountID:         "ACCOUNT-1",
			APIKey:            "INSERT-API-KEY-1",
			ServicePrefix:     "",
			EnableCompression: compressionEnabled,
			FlushInterval:     30 * time.Second,
		}
		var err error
		metricEmitter, err = newrelicConfig.NewEmitter()
		if err != nil {
			Fail("failed to create emitter from new relic configuration")
		}

		if newrelicEmitter, OK := metricEmitter.(*emitter.NewRelicEmitter); OK {
			newrelicEmitter.SetUrl(newrelicServer.URL() + "/EVENTS")
		} else {
			Fail("failed to convert the emitter to NewRelicEmitter")
		}

	})

	JustAfterEach(func() {
		newrelicServer.Close()
	})

	Context("when compression is enabled", func() {
		BeforeEach(func() {
			compressionEnabled = true
		})
		AfterEach(func() {
			compressionEnabled = false
		})
		It("does not send out compressed data out when the data length is less than 1MB", func() {
			emitSampleMetrics(10486, 1048600-1)
		})

		It("does send out compressed data when the data over the limit", func() {
			emitSampleMetrics(10490, 1049000-1)
		})
	})

	Context("when batch buffer is less than 1MB", func() {
		It("enqueue to the batch buffer", func() {
			emitSampleMetrics(1, 99)
		})
	})

	Context("when batch buffer is great than 1MB", func() {
		It("emit the existed metrics, enqueue the current metric", func() {
			emitSampleMetrics(10486, 99)
		})
	})
})
