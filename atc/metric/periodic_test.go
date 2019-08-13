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
		testLogger := lager.NewLogger("test")

		emitterFactory := &metricfakes.FakeEmitterFactory{}
		emitter = &metricfakes.FakeEmitter{}

		metric.RegisterEmitter(emitterFactory)
		emitterFactory.IsConfiguredReturns(true)
		emitterFactory.NewEmitterReturns(emitter, nil)
		a := &dbfakes.FakeConn{}
		a.NameReturns("A")
		b := &dbfakes.FakeConn{}
		b.NameReturns("B")
		metric.Databases = []db.Conn{a, b}
		metric.Initialize(testLogger, "test", map[string]string{}, 1000)

		process = ifrit.Invoke(metric.PeriodicallyEmit(lager.NewLogger("dont care"), 250*time.Millisecond))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
		metric.Deinitialize(nil)
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
