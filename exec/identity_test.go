package exec_test

import (
	"os"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Identity", func() {
	var (
		inSource *fakes.FakeArtifactSource

		identity Identity

		source ArtifactSource
	)

	BeforeEach(func() {
		identity = Identity{}

		inSource = new(fakes.FakeArtifactSource)
	})

	JustBeforeEach(func() {
		source = identity.Using(inSource)
	})

	Describe("Run", func() {
		It("is a no-op", func() {
			ready := make(chan struct{})
			signals := make(chan os.Signal)

			err := source.Run(signals, ready)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(inSource.RunCallCount()).Should(BeZero())
		})
	})

	Describe("StreamTo", func() {
		It("calls through to the input source", func() {
			destination := new(fakes.FakeArtifactDestination)

			err := source.StreamTo(destination)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(inSource.StreamToCallCount()).Should(Equal(1))
			Ω(inSource.StreamToArgsForCall(0)).Should(Equal(destination))
		})
	})

	Describe("Result", func() {
		It("calls through to the input source", func() {
			var result int
			source.Result(&result)

			Ω(inSource.ResultCallCount()).Should(Equal(1))
			Ω(inSource.ResultArgsForCall(0)).Should(Equal(&result))
		})
	})

	Describe("StreamFile", func() {
		It("calls through to the input source", func() {
			_, err := source.StreamFile("some-path")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(inSource.StreamFileCallCount()).Should(Equal(1))
			Ω(inSource.StreamFileArgsForCall(0)).Should(Equal("some-path"))
		})
	})
})
