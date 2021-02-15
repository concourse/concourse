package metric_test

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/metricfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	Describe("worker state metric", func() {
		var (
			emitter *smartFakeEmitter
			monitor *metric.Monitor
		)

		BeforeEach(func() {
			emitter = new(smartFakeEmitter)
			monitor = metric.NewMonitor()

			emitterFactory := new(metricfakes.FakeEmitterFactory)
			emitterFactory.IsConfiguredReturns(true)
			emitterFactory.NewEmitterReturns(emitter, nil)

			monitor.RegisterEmitter(emitterFactory)
			monitor.Initialize(testLogger, "test", map[string]string{}, 1000)
		})

		It("emits a value for every state", func() {
			givenNoWorkers().Emit(testLogger, monitor)

			waitForEvents(emitter)

			for _, state := range db.AllWorkerStates() {
				event := emitter.eventWithState(state)
				Expect(event.Value).To(Equal(float64(0)))
			}
		})

		It("correctly emits the number of running workers", func() {
			givenOneWorkerWithState(db.WorkerStateRunning).
				Emit(testLogger, monitor)

			waitForEvents(emitter)

			event := emitter.eventWithState(db.WorkerStateRunning)
			Expect(event.Value).To(Equal(float64(1)))
		})
	})
})

type smartFakeEmitter struct {
	metricfakes.FakeEmitter
}

func (fakeEmitter *smartFakeEmitter) eventWithState(state db.WorkerState) *metric.Event {
	for i := 0; i < fakeEmitter.EmitCallCount(); i++ {
		_, event := fakeEmitter.EmitArgsForCall(i)
		if event.Attributes["state"] == string(state) {
			return &event
		}
	}
	return nil
}

func givenNoWorkers() metric.WorkersState {
	return metric.WorkersState{
		WorkerStateByName: make(map[string]db.WorkerState),
	}
}

func givenOneWorkerWithState(state db.WorkerState) metric.WorkersState {
	workersState := givenNoWorkers()
	workersState.WorkerStateByName["my-worker"] = state
	return workersState
}

func waitForEvents(fakeEmitter *smartFakeEmitter) {
	numberOfWorkerStates := len(db.AllWorkerStates())
	Eventually(fakeEmitter.EmitCallCount).Should(Equal(numberOfWorkerStates))
}
