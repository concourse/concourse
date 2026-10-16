package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/atc/worker"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("selectStepWorker", func() {
	var (
		ctx           context.Context
		fakePool      *execfakes.FakePool
		fakeState     *execfakes.FakeRunState
		owner         db.ContainerOwner
		containerSpec runtime.ContainerSpec
		spec          worker.Spec
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakePool = new(execfakes.FakePool)
		fakeState = new(execfakes.FakeRunState)
		owner = db.NewFixedHandleContainerOwner("any")
		containerSpec = runtime.ContainerSpec{Type: db.ContainerTypeTask}
		spec = worker.Spec{}
	})

	It("calls FindOrSelectWorker when pin_worker is false", func() {
		w := runtimetest.NewWorker("any")
		fakePool.FindOrSelectWorkerReturns(w, nil)

		got, err := exec.SelectStepWorkerForTest(ctx, fakePool, nil, false, fakeState, nil, owner, containerSpec, spec)
		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal(w))

		Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
		Expect(fakePool.FindOrSelectWorkerOnPinnedCallCount()).To(Equal(0))
		Expect(fakePool.ReleaseWorkerCallCount()).To(Equal(0))
	})

	It("calls FindOrSelectWorkerOnPinned when a worker is already pinned", func() {
		fakeState.PinnedWorkerReturns("pinned-worker")
		w := runtimetest.NewWorker("pinned-worker")
		fakePool.FindOrSelectWorkerOnPinnedReturns(w, nil)

		got, err := exec.SelectStepWorkerForTest(ctx, fakePool, nil, true, fakeState, nil, owner, containerSpec, spec)
		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal(w))

		Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(0))
		Expect(fakePool.FindOrSelectWorkerOnPinnedCallCount()).To(Equal(1))
		_, _, _, _, pinnedName, _, _ := fakePool.FindOrSelectWorkerOnPinnedArgsForCall(0)
		Expect(pinnedName).To(Equal("pinned-worker"))
	})

	It("pins the chosen worker and returns it when no worker was previously pinned", func() {
		fakeState.PinnedWorkerReturns("")
		fakeState.SetPinnedWorkerStub = func(name string) string {
			// First call: nothing pinned yet, so we win.
			Expect(name).To(Equal("candidate"))
			return name
		}

		candidate := runtimetest.NewWorker("candidate")
		fakePool.FindOrSelectWorkerReturns(candidate, nil)

		got, err := exec.SelectStepWorkerForTest(ctx, fakePool, nil, true, fakeState, nil, owner, containerSpec, spec)
		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal(candidate))

		Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
		Expect(fakePool.FindOrSelectWorkerOnPinnedCallCount()).To(Equal(0))
		Expect(fakePool.ReleaseWorkerCallCount()).To(Equal(0))
	})

	It("releases the candidate and re-selects on the winning worker when losing the pin race", func() {
		// PinnedWorker() returns "" the first time (no pinned worker yet),
		// but SetPinnedWorker returns "winner-worker" — simulating a
		// parallel step that won the race between the helper's check
		// and pin.
		fakeState.PinnedWorkerReturns("")
		fakeState.SetPinnedWorkerStub = func(_ string) string {
			return "winner-worker"
		}

		candidate := runtimetest.NewWorker("candidate")
		winner := runtimetest.NewWorker("winner-worker")
		fakePool.FindOrSelectWorkerReturns(candidate, nil)
		fakePool.FindOrSelectWorkerOnPinnedReturns(winner, nil)

		got, err := exec.SelectStepWorkerForTest(ctx, fakePool, nil, true, fakeState, nil, owner, containerSpec, spec)
		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal(winner))

		Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
		Expect(fakePool.FindOrSelectWorkerOnPinnedCallCount()).To(Equal(1))
		Expect(fakePool.ReleaseWorkerCallCount()).To(Equal(1))
		_, _, _, _, pinnedName, _, _ := fakePool.FindOrSelectWorkerOnPinnedArgsForCall(0)
		Expect(pinnedName).To(Equal("winner-worker"))
	})

	It("returns nil when FindOrSelectWorker returns no worker and nothing is pinned", func() {
		fakeState.PinnedWorkerReturns("")
		fakePool.FindOrSelectWorkerReturns(nil, nil)

		got, err := exec.SelectStepWorkerForTest(ctx, fakePool, nil, true, fakeState, nil, owner, containerSpec, spec)
		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(BeNil())

		Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
		Expect(fakePool.ReleaseWorkerCallCount()).To(Equal(0))
	})

	It("propagates errors from FindOrSelectWorker", func() {
		disaster := errors.New("boom")
		fakePool.FindOrSelectWorkerReturns(nil, disaster)

		got, err := exec.SelectStepWorkerForTest(ctx, fakePool, nil, false, fakeState, nil, owner, containerSpec, spec)
		Expect(err).To(MatchError(disaster))
		Expect(got).To(BeNil())
	})
})
