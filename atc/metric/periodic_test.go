package metric_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/metricfakes"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Periodic emission of metrics", func() {
	var (
		emitter *metricfakes.FakeEmitter

		process ifrit.Process
	)

	BeforeEach(func() {
		emitterFactory := &metricfakes.FakeEmitterFactory{}
		emitter = &metricfakes.FakeEmitter{}

		metric.RegisterEmitter(emitterFactory)
		emitterFactory.IsConfiguredReturns(true)
		emitterFactory.NewEmitterReturns(emitter, nil)
		metric.Initialize(testLogger, "test", map[string]string{}, 1000)

	})

	JustBeforeEach(func() {
		runner := metric.PeriodicallyEmit(
			lager.NewLogger("dont care"),
			250*time.Millisecond,
		)

		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
		metric.Deinitialize(nil)
	})

	Context("database-related metrics", func() {
		BeforeEach(func() {
			a := &dbfakes.FakeConn{}
			a.NameReturns("A")
			b := &dbfakes.FakeConn{}
			b.NameReturns("B")
			metric.Databases = []db.Conn{a, b}
		})

		It("emits database queries", func() {
			Eventually(emitter.EmitCallCount).Should(BeNumerically(">=", 1))
			Expect(emitter.Invocations()["Emit"]).To(
				ContainElement(
					ContainElement(
						MatchFields(IgnoreExtras, Fields{
							"Name": Equal("database queries"),
						}),
					),
				),
			)

			By("emits database connections for each pool")
			Expect(emitter.Invocations()["Emit"]).To(
				ContainElement(
					ContainElement(
						MatchFields(IgnoreExtras, Fields{
							"Name":       Equal("database connections"),
							"Attributes": Equal(map[string]string{"ConnectionName": "A"}),
						}),
					),
				),
			)
			Expect(emitter.Invocations()["Emit"]).To(
				ContainElement(
					ContainElement(
						MatchFields(IgnoreExtras, Fields{
							"Name":       Equal("database connections"),
							"Attributes": Equal(map[string]string{"ConnectionName": "B"}),
						}),
					),
				),
			)
		})
	})

	Context("concurrent requests", func() {
		const action = "ListAllSomething"

		BeforeEach(func() {
			gauge := &metric.Gauge{}
			gauge.Set(123)

			counter := &metric.Counter{}
			counter.IncDelta(10)

			metric.ConcurrentRequests[action] = gauge
			metric.ConcurrentRequestsLimitHit[action] = counter
		})

		It("emits", func() {
			Eventually(emitter.EmitCallCount).Should(BeNumerically(">=", 1))

			Expect(emitter.Invocations()["Emit"]).To(
				ContainElement(
					ContainElement(
						MatchFields(IgnoreExtras, Fields{
							"Name":  Equal("concurrent requests"),
							"Value": Equal(float64(123)),
							"Attributes": Equal(map[string]string{
								"action": action,
							}),
						}),
					),
				),
			)

			Expect(emitter.Invocations()["Emit"]).To(
				ContainElement(
					ContainElement(
						MatchFields(IgnoreExtras, Fields{
							"Name":  Equal("concurrent requests limit hit"),
							"Value": Equal(float64(10)),
							"Attributes": Equal(map[string]string{
								"action": action,
							}),
						}),
					),
				),
			)
		})
	})

	Context("limit-active-tasks metrics", func() {
		BeforeEach(func() {
			gauge := &metric.Gauge{}
			gauge.Set(123)
			metric.TasksWaiting = gauge
		})
		It("emits", func() {
			Eventually(emitter.EmitCallCount).Should(BeNumerically(">=", 1))
			Expect(emitter.Invocations()["Emit"]).To(
				ContainElement(
					ContainElement(
						MatchFields(IgnoreExtras, Fields{
							"Name":  Equal("tasks waiting"),
							"Value": Equal(float64(123)),
						}),
					),
				),
			)
		})
	})
})
