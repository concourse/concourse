package metric_test

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	Describe("worker state metric", func() {
		It("emits a value for every state", func() {
			events := givenNoWorkers().Events()

			for _, state := range db.AllWorkerStates() {
				event := eventWithState(state, events)
				Expect(event.Value).To(Equal(0))
			}
		})

		It("correctly emits the number of running workers", func() {
			events := givenOneWorkerWithState(db.WorkerStateRunning).
				Events()

			event := eventWithState(db.WorkerStateRunning, events)
			Expect(event.Value).To(Equal(1))
		})

		It("emits warning event for stalled workers", func() {
			events := givenOneWorkerWithState(db.WorkerStateStalled).
				Events()

			event := eventWithState(db.WorkerStateStalled, events)
			Expect(event.State).To(Equal(metric.EventStateWarning))
		})

		It("emits OK event when no workers are stalled", func() {
			events := givenNoWorkers().Events()

			event := eventWithState(db.WorkerStateStalled, events)
			Expect(event.State).To(Equal(metric.EventStateOK))
		})
	})
})

func eventWithState(state db.WorkerState, events []metric.Event) *metric.Event {
	for _, event := range events {
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
