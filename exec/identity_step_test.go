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

		identity Identity

		step Step
	)

	BeforeEach(func() {
		identity = Identity{}

		repo = worker.NewArtifactRepository()
	})

	JustBeforeEach(func() {
		step = identity.Using(repo)
	})

	Describe("Run", func() {
		It("is a no-op", func() {
			err := step.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Succeeded", func() {
		It("returns true", func() {
			Expect(step.Succeeded()).To(BeTrue())
		})
	})
})
