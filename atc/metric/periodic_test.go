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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Periodic emission of metrics", func() {
	var (
		emitter *metricfakes.FakeEmitter
		monitor *metric.Monitor

		process ifrit.Process
	)

	BeforeEach(func() {
		emitter = &metricfakes.FakeEmitter{}
		monitor = metric.NewMonitor()

		emitterFactory := &metricfakes.FakeEmitterFactory{}
		emitterFactory.IsConfiguredReturns(true)
		emitterFactory.NewEmitterReturns(emitter, nil)

		monitor.RegisterEmitter(emitterFactory)
		monitor.Initialize(testLogger, "test", map[string]string{}, 1000)

	})

	JustBeforeEach(func() {
		runner := metric.PeriodicallyEmit(
			lager.NewLogger("dont care"),
			monitor,
			250*time.Millisecond,
		)

		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
	})

	events := func() []metric.Event {
		var events []metric.Event
		for i := 0; i < emitter.EmitCallCount(); i++ {
			_, event := emitter.EmitArgsForCall(i)
			events = append(events, event)
		}
		return events
	}

	Context("database-related metrics", func() {
		BeforeEach(func() {
			a := &dbfakes.FakeConn{}
			a.NameReturns("A")
			b := &dbfakes.FakeConn{}
			b.NameReturns("B")
			monitor.Databases = []db.Conn{a, b}
		})

		It("emits database queries", func() {
			Eventually(events).Should(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("database queries"),
					}),
				),
			)

			By("emits database connections for each pool")
			Eventually(events).Should(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Name":       Equal("database connections"),
						"Attributes": Equal(map[string]string{"ConnectionName": "A"}),
					}),
				),
			)
			Eventually(events).Should(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Name":       Equal("database connections"),
						"Attributes": Equal(map[string]string{"ConnectionName": "B"}),
					}),
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

			monitor.ConcurrentRequests[action] = gauge
			monitor.ConcurrentRequestsLimitHit[action] = counter
		})

		It("emits", func() {
			Eventually(events).Should(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Name":  Equal("concurrent requests"),
						"Value": Equal(float64(123)),
						"Attributes": Equal(map[string]string{
							"action": action,
						}),
					}),
				),
			)

			Eventually(events).Should(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Name":  Equal("concurrent requests limit hit"),
						"Value": Equal(float64(10)),
						"Attributes": Equal(map[string]string{
							"action": action,
						}),
					}),
				),
			)
		})
	})

	Context("waiting steps metrics", func() {
		labels := metric.StepsWaitingLabels{
			Platform:   "darwin",
			TeamId:     "42",
			TeamName:   "teamdev",
			Type:       "task",
			WorkerTags: "tester",
		}

		BeforeEach(func() {
			gauge := &metric.Gauge{}
			gauge.Set(123)
			monitor.StepsWaiting[labels] = gauge
		})

		It("emits", func() {
			Eventually(events).Should(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Name":  Equal("steps waiting"),
						"Value": Equal(float64(123)),
						"Attributes": Equal(map[string]string{
							"platform":   labels.Platform,
							"teamId":     labels.TeamId,
							"teamName":   labels.TeamName,
							"type":       labels.Type,
							"workerTags": labels.WorkerTags,
						}),
					}),
				),
			)
		})
	})
})
