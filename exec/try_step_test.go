package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Try Step", func() {
	var (
		ctx    context.Context
		cancel func()

		runStep *execfakes.FakeStep

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		step Step
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		runStep = new(execfakes.FakeStep)

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		step = Try(runStep)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Succeeded", func() {
		It("returns true", func() {
			Expect(step.Succeeded()).Should(BeTrue())
		})
	})

	Describe("Run", func() {
		Context("when interrupted", func() {
			BeforeEach(func() {
				runStep.RunReturns(context.Canceled)
			})

			It("propagates the error", func() {
				err := step.Run(ctx, state)
				Expect(err).To(Equal(context.Canceled))
			})
		})

		Context("when the inner step returns any other error", func() {
			BeforeEach(func() {
				runStep.RunReturns(errors.New("some error"))
			})

			It("swallows the error", func() {
				err := step.Run(ctx, state)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
