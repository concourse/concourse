package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("On Error Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		onErrorStep exec.Step

		stepOk  bool
		stepErr error

		disaster error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		onErrorStep = exec.OnError(step, hook)

		stepErr = nil

		disaster = multierror.Append(disaster, errors.New("disaster"))
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		stepOk, stepErr = onErrorStep.Run(ctx, state)
	})

	Context("when the step errors", func() {
		BeforeEach(func() {
			step.RunReturns(false, disaster)
		})

		It("runs the error hook", func() {
			Expect(stepErr).To(Equal(disaster))
			Expect(hook.RunCallCount()).To(Equal(1))
			Expect(step.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the step error is retriable", func() {
		BeforeEach(func() {
			disaster = multierror.Append(disaster, exec.Retriable{})
			step.RunReturns(false, disaster)
		})

		It("does not run the error hook", func() {
			Expect(stepErr).To(Equal(disaster))
			Expect(hook.RunCallCount()).To(Equal(0))
			Expect(step.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
		})

		It("is successful", func() {
			Expect(stepOk).To(BeTrue())
		})

		It("does not run the error hook", func() {
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
		})

		It("is not successful", func() {
			Expect(stepOk).ToNot(BeTrue())
		})

		It("does not run the error hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})
})
