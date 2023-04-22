package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Try Step", func() {
	var (
		ctx    context.Context
		cancel func()

		runStep *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		step Step

		stepOk  bool
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		runStep = new(execfakes.FakeStep)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		step = Try(runStep)
	})

	JustBeforeEach(func() {
		stepOk, stepErr = step.Run(ctx, state)
	})

	AfterEach(func() {
		cancel()
	})

	Context("when the inner step fails", func() {
		BeforeEach(func() {
			runStep.RunReturns(false, nil)
		})

		It("succeeds anyway", func() {
			Expect(stepErr).NotTo(HaveOccurred())
			Expect(stepOk).To(BeTrue())
		})
	})

	Context("when interrupted", func() {
		BeforeEach(func() {
			runStep.RunReturns(false, context.Canceled)
		})

		It("propagates the error and does not succeed", func() {
			Expect(stepErr).To(Equal(context.Canceled))
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when the inner step returns any other error", func() {
		BeforeEach(func() {
			runStep.RunReturns(false, errors.New("some error"))
		})

		It("swallows the error", func() {
			Expect(stepErr).NotTo(HaveOccurred())
			Expect(stepOk).To(BeTrue())
		})
	})
})
