package exec_test

import (
	"os"

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
			ready := make(chan struct{})
			signals := make(chan os.Signal)

			err := step.Run(signals, ready)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Succeeded", func() {
		It("calls through to the input source", func() {
			Expect(step.Succeeded()).To(BeTrue())
		})
	})
})
