package exec_test

import (
	"errors"
	"io"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func successResult(result Success) func(dest interface{}) bool {
	return func(dest interface{}) bool {
		switch x := dest.(type) {
		case *Success:
			*x = result
			return true

		default:
			return false
		}
	}
}

var _ = Describe("Conditional", func() {
	var (
		inSource *fakes.FakeArtifactSource

		fakeStep    *fakes.FakeStep
		conditional Conditional

		outSource *fakes.FakeArtifactSource

		process ifrit.Process
		source  ArtifactSource
	)

	BeforeEach(func() {
		inSource = new(fakes.FakeArtifactSource)
		fakeStep = new(fakes.FakeStep)

		outSource = new(fakes.FakeArtifactSource)
		outSource.ResultStub = successResult(true)

		fakeStep.UsingReturns(outSource)

		conditional = Conditional{
			Step: fakeStep,
		}
	})

	JustBeforeEach(func() {
		source = conditional.Using(inSource)
		process = ifrit.Invoke(source)
	})

	itDoesNothing := func() {
		It("succeeds", func() {
			Eventually(process.Wait()).Should(Receive(BeNil()))
		})

		It("does not use the step's artifact source", func() {
			Ω(fakeStep.UsingCallCount()).Should(BeZero())
		})

		Describe("streaming to a destination", func() {
			var fakeDestination *fakes.FakeArtifactDestination

			BeforeEach(func() {
				fakeDestination = new(fakes.FakeArtifactDestination)
			})

			It("does not stream from the input source", func() {
				err := source.StreamTo(fakeDestination)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(inSource.StreamToCallCount()).Should(Equal(0))

				Ω(fakeDestination.StreamInCallCount()).Should(Equal(0))
			})
		})

		Describe("streaming a file out", func() {
			It("returns ErrFileNotFound", func() {
				_, err := source.StreamFile("some-file")
				Ω(err).Should(Equal(ErrFileNotFound))
			})
		})

		Describe("releasing", func() {
			It("does not release the input source", func() {
				Ω(inSource.ReleaseCallCount()).Should(Equal(0))
			})
		})

		Describe("getting the result", func() {
			It("fails", func() {
				var success Success
				Ω(source.Result(&success)).Should(BeFalse())
			})
		})
	}

	itDoesAThing := func() {
		It("succeeds", func() {
			Eventually(process.Wait()).Should(Receive(BeNil()))
		})

		It("uses the step's artifact source", func() {
			Ω(fakeStep.UsingCallCount()).Should(Equal(1))
			Ω(fakeStep.UsingArgsForCall(0)).Should(Equal(inSource))
		})

		Describe("streaming to a destination", func() {
			var fakeDestination *fakes.FakeArtifactDestination

			BeforeEach(func() {
				fakeDestination = new(fakes.FakeArtifactDestination)
			})

			It("delegates to the step's artifact source", func() {
				err := source.StreamTo(fakeDestination)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outSource.StreamToCallCount()).Should(Equal(1))
				Ω(outSource.StreamToArgsForCall(0)).Should(Equal(fakeDestination))
			})

			Context("when the output source fails to stream out", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outSource.StreamToReturns(disaster)
				})

				It("returns the error", func() {
					err := source.StreamTo(fakeDestination)
					Ω(err).Should(Equal(disaster))
				})
			})

			Describe("getting the result", func() {
				It("succeeds", func() {
					var success Success
					Ω(source.Result(&success)).Should(BeTrue())

					Ω(bool(success)).Should(BeTrue())
				})
			})
		})

		Describe("streaming a file out", func() {
			var outStream io.ReadCloser

			BeforeEach(func() {
				outStream = gbytes.NewBuffer()
				outSource.StreamFileReturns(outStream, nil)
			})

			It("delegates to the step's artifact source", func() {
				reader, err := source.StreamFile("some-file")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outSource.StreamFileCallCount()).Should(Equal(1))
				Ω(outSource.StreamFileArgsForCall(0)).Should(Equal("some-file"))

				Ω(reader).Should(Equal(outStream))
			})

			Context("when the output source fails to stream out", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outSource.StreamFileReturns(nil, disaster)
				})

				It("returns the error", func() {
					_, err := source.StreamFile("some-file")
					Ω(err).Should(Equal(disaster))
				})
			})
		})

		Describe("releasing", func() {
			It("releases the output source", func() {
				err := source.Release()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outSource.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when releasing the output source fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outSource.ReleaseReturns(disaster)
				})

				It("returns the error", func() {
					Ω(source.Release()).Should(Equal(disaster))
				})
			})
		})
	}

	Context("with no conditions", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.OutputConditions{}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(true)
			})

			itDoesNothing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(false)
			})

			itDoesNothing()
		})

		Context("when the input source cannot indicate success", func() {
			itDoesNothing()
		})
	})

	Context("with a success condition", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.OutputConditions{atc.OutputConditionSuccess}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(true)
			})

			itDoesAThing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(false)
			})

			itDoesNothing()
		})

		Context("when the input source cannot indicate success", func() {
			itDoesNothing()
		})
	})

	Context("with a failure condition", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.OutputConditions{atc.OutputConditionFailure}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(true)
			})

			itDoesNothing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(false)
			})

			itDoesAThing()
		})

		Context("when the input source cannot indicate success", func() {
			itDoesNothing()
		})
	})

	Context("with a success or failure condition", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.OutputConditions{
				atc.OutputConditionFailure,
				atc.OutputConditionSuccess,
			}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(true)
			})

			itDoesAThing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inSource.ResultStub = successResult(false)
			})

			itDoesAThing()
		})

		Context("when the input source cannot indicate success", func() {
			itDoesNothing()
		})
	})
})
