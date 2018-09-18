package exec_test

import (
	"context"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Identity", func() {
	var (
		state *execfakes.FakeRunState

		step IdentityStep

		stepErr error
	)

	BeforeEach(func() {
		step = IdentityStep{}
		state = new(execfakes.FakeRunState)
	})

	JustBeforeEach(func() {
		stepErr = step.Run(context.Background(), state)
	})

	Describe("Run", func() {
		It("is a no-op", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})
	})

	Describe("Succeeded", func() {
		It("returns true", func() {
			Expect(step.Succeeded()).To(BeTrue())
		})
	})
})
