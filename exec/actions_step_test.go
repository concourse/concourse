package exec_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("ActionsStep", func() {
	var (
		fakeBuildEventsDelegate *execfakes.FakeActionsBuildEventsDelegate
		fakeAction1             *execfakes.FakeAction
		fakeAction2             *execfakes.FakeAction
		artifactRepository      *worker.ArtifactRepository
		actionsStep             exec.Step
	)

	BeforeEach(func() {
		fakeBuildEventsDelegate = new(execfakes.FakeActionsBuildEventsDelegate)
		fakeAction1 = new(execfakes.FakeAction)
		fakeAction2 = new(execfakes.FakeAction)
		artifactRepository = worker.NewArtifactRepository()

		actionsStep = exec.NewActionsStep(
			lagertest.NewTestLogger("actions-step-test"),
			[]exec.Action{fakeAction1, fakeAction2},
			fakeBuildEventsDelegate,
		).Using(artifactRepository)
	})

	JustBeforeEach(func() {
		ifrit.Invoke(actionsStep)
	})

	Context("when actions return no error", func() {
		It("executes every action", func() {
			Expect(fakeAction1.RunCallCount()).To(Equal(1))
			Expect(fakeAction2.RunCallCount()).To(Equal(1))
		})

		It("invoked the delegate's ActionCompleted callback", func() {
			Expect(fakeBuildEventsDelegate.ActionCompletedCallCount()).To(Equal(2))
			_, action := fakeBuildEventsDelegate.ActionCompletedArgsForCall(0)
			Expect(action).To(Equal(fakeAction1))
			_, action = fakeBuildEventsDelegate.ActionCompletedArgsForCall(1)
			Expect(action).To(Equal(fakeAction2))
		})

		Context("when all actions exited with 0 status", func() {
			BeforeEach(func() {
				fakeAction1.ExitStatusReturns(0)
				fakeAction2.ExitStatusReturns(0)
			})

			It("returns succeeded true", func() {
				Expect(actionsStep.Succeeded()).To(BeTrue())
			})
		})

		Context("when at least one of the actions exited with non-0 status", func() {
			BeforeEach(func() {
				fakeAction1.ExitStatusReturns(1)
				fakeAction2.ExitStatusReturns(0)
			})

			It("returns succeeded false", func() {
				Expect(actionsStep.Succeeded()).To(BeFalse())
			})
		})
	})

	Context("when action returns an error", func() {
		var disaster = errors.New("disaster")

		BeforeEach(func() {
			fakeAction1.RunReturns(disaster)
		})

		It("invoked the delegate's Failed callback", func() {
			Expect(fakeBuildEventsDelegate.FailedCallCount()).To(Equal(1))
			_, failedErr := fakeBuildEventsDelegate.FailedArgsForCall(0)
			Expect(failedErr).To(Equal(disaster))
		})

		It("does not execute subsequent actions", func() {
			Expect(fakeAction2.RunCallCount()).To(Equal(0))
		})
	})

	Context("when action returns ErrAborted", func() {
		BeforeEach(func() {
			fakeAction1.RunReturns(resource.ErrAborted)
		})

		It("invoked the delegate's Failed callback with ErrInterrupted", func() {
			Expect(fakeBuildEventsDelegate.FailedCallCount()).To(Equal(1))
			_, failedErr := fakeBuildEventsDelegate.FailedArgsForCall(0)
			Expect(failedErr).To(Equal(exec.ErrInterrupted))
		})

		It("does not execute subsequent actions", func() {
			Expect(fakeAction2.RunCallCount()).To(Equal(0))
		})
	})
})
