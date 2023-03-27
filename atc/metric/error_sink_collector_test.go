package metric_test

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/metricfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ErrorSinkCollector", func() {
	var (
		errorSinkCollector metric.ErrorSinkCollector
		emitter            *metricfakes.FakeEmitter
		monitor            *metric.Monitor
	)

	BeforeEach(func() {
		emitter = &metricfakes.FakeEmitter{}
		monitor = metric.NewMonitor()
		errorSinkCollector = metric.NewErrorSinkCollector(testLogger, monitor)

		emitterFactory := &metricfakes.FakeEmitterFactory{}
		emitterFactory.IsConfiguredReturns(true)
		emitterFactory.NewEmitterReturns(emitter, nil)
		monitor.RegisterEmitter(emitterFactory)
		monitor.Initialize(testLogger, "test", map[string]string{}, 1000)
	})

	Context("Log", func() {
		var log lager.LogFormat

		JustBeforeEach(func() {
			errorSinkCollector.Log(log)
		})

		Context("with message of level ERROR", func() {
			BeforeEach(func() {
				log = lager.LogFormat{
					Message:  "err-msg",
					LogLevel: lager.ERROR,
				}
			})

			It("emits with the message in the tags", func() {
				Eventually(emitter.EmitCallCount).Should(BeNumerically("==", 1))
				_, event := emitter.EmitArgsForCall(0)
				Expect(event.Attributes).To(HaveKeyWithValue("message", "err-msg"))
			})

			Context("with error being from failed emission", func() {
				BeforeEach(func() {
					log = lager.LogFormat{
						Message:  "message",
						LogLevel: lager.ERROR,
						Error:    metric.ErrFailedToEmit,
					}
				})

				It("doesn't emit", func() {
					Consistently(emitter.EmitCallCount).Should(BeNumerically("==", 0))
				})
			})
		})

		Context("with message of non-ERROR level", func() {
			BeforeEach(func() {
				log = lager.LogFormat{
					Message:  "message",
					LogLevel: lager.INFO,
				}
			})

			It("doesn't emit", func() {
				Consistently(emitter.EmitCallCount).Should(BeNumerically("==", 0))
			})
		})
	})
})
