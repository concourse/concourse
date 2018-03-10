package exec_test

import (
	"context"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Identity", func() {
	var (
		repo *worker.ArtifactRepository

		step IdentityStep

		stepErr error
	)

	BeforeEach(func() {
		step = IdentityStep{}
		repo = worker.NewArtifactRepository()
	})

	JustBeforeEach(func() {
		stepErr = step.Run(context.Background(), repo)
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
