package exec_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Aggregate", func() {
	var (
		fakeStepA *fakes.FakeStep
		fakeStepB *fakes.FakeStep

		aggregate Step

		inSource *fakes.FakeArtifactSource

		outSourceA *fakes.FakeArtifactSource
		outSourceB *fakes.FakeArtifactSource

		source  ArtifactSource
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepA = new(fakes.FakeStep)
		fakeStepB = new(fakes.FakeStep)

		aggregate = Aggregate{
			"A": fakeStepA,
			"B": fakeStepB,
		}

		inSource = new(fakes.FakeArtifactSource)

		outSourceA = new(fakes.FakeArtifactSource)
		fakeStepA.UsingReturns(outSourceA)

		outSourceB = new(fakes.FakeArtifactSource)
		fakeStepB.UsingReturns(outSourceB)
	})

	JustBeforeEach(func() {
		source = aggregate.Using(inSource)
		process = ifrit.Invoke(source)
	})

	It("uses the input source for all steps", func() {
		Ω(fakeStepA.UsingCallCount()).Should(Equal(1))
		Ω(fakeStepA.UsingArgsForCall(0)).Should(Equal(inSource))

		Ω(fakeStepB.UsingCallCount()).Should(Equal(1))
		Ω(fakeStepB.UsingArgsForCall(0)).Should(Equal(inSource))
	})

	It("exits successfully", func() {
		Eventually(process.Wait()).Should(Receive(BeNil()))
	})

	Describe("executing each source", func() {
		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			wg.Add(2)

			outSourceA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				wg.Done()
				wg.Wait()
				close(ready)
				return nil
			}

			outSourceB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				wg.Done()
				wg.Wait()
				close(ready)
				return nil
			}
		})

		It("happens concurrently", func() {
			Ω(outSourceA.RunCallCount()).Should(Equal(1))
			Ω(outSourceB.RunCallCount()).Should(Equal(1))
		})
	})

	Context("when sources fail", func() {
		disasterA := errors.New("nope A")
		disasterB := errors.New("nope B")

		BeforeEach(func() {
			outSourceA.RunReturns(disasterA)
			outSourceB.RunReturns(disasterB)
		})

		It("exits with an error including the original message", func() {
			var err error
			Eventually(process.Wait()).Should(Receive(&err))

			Ω(err.Error()).Should(ContainSubstring("A: nope A"))
			Ω(err.Error()).Should(ContainSubstring("B: nope B"))
		})
	})

	Describe("streaming to a destination", func() {
		var fakeDestination *fakes.FakeArtifactDestination

		BeforeEach(func() {
			fakeDestination = new(fakes.FakeArtifactDestination)
		})

		It("streams each source to a subdirectory in the destination", func() {
			err := source.StreamTo(fakeDestination)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(outSourceA.StreamToCallCount()).Should(Equal(1))
			Ω(outSourceB.StreamToCallCount()).Should(Equal(1))

			destA := outSourceA.StreamToArgsForCall(0)
			destB := outSourceB.StreamToArgsForCall(0)

			src := new(bytes.Buffer)

			err = destA.StreamIn("foo", src)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeDestination.StreamInCallCount()).Should(Equal(1))

			realDest, realSrc := fakeDestination.StreamInArgsForCall(0)
			Ω(realDest).Should(Equal("A/foo"))
			Ω(realSrc).Should(Equal(src))

			err = destB.StreamIn("foo", src)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeDestination.StreamInCallCount()).Should(Equal(2))

			realDest, realSrc = fakeDestination.StreamInArgsForCall(1)
			Ω(realDest).Should(Equal("B/foo"))
			Ω(realSrc).Should(Equal(src))
		})

		Context("when the any of the sources fails to stream", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				outSourceA.StreamToReturns(disaster)
			})

			It("returns the error", func() {
				err := source.StreamTo(fakeDestination)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("streaming a file out", func() {
		Context("from a path not referring to any source", func() {
			It("returns ErrFileNotFound", func() {
				_, err := source.StreamFile("X/foo")
				Ω(err).Should(Equal(ErrFileNotFound))
			})
		})

		Context("from a path referring to a source", func() {
			var outStream io.ReadCloser

			BeforeEach(func() {
				outStream = gbytes.NewBuffer()
				outSourceA.StreamFileReturns(outStream, nil)
			})

			It("streams out from the source", func() {
				out, err := source.StreamFile("A/foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(out).Should(Equal(outStream))

				Ω(outSourceA.StreamFileArgsForCall(0)).Should(Equal("foo"))
			})

			Context("when streaming out from the source fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outSourceA.StreamFileReturns(nil, disaster)
				})

				It("returns the error", func() {
					_, err := source.StreamFile("A/foo")
					Ω(err).Should(Equal(disaster))
				})
			})
		})
	})

	Describe("releasing", func() {
		It("releases all sources", func() {
			err := source.Release()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(outSourceA.ReleaseCallCount()).Should(Equal(1))
			Ω(outSourceB.ReleaseCallCount()).Should(Equal(1))
		})

		Context("when the sources fail to release", func() {
			disasterA := errors.New("nope A")
			disasterB := errors.New("nope B")

			BeforeEach(func() {
				outSourceA.ReleaseReturns(disasterA)
				outSourceB.ReleaseReturns(disasterB)
			})

			It("returns an error describing the failures", func() {
				err := source.Release()
				Ω(err).Should(HaveOccurred())

				Ω(err.Error()).Should(ContainSubstring("A: nope A"))
				Ω(err.Error()).Should(ContainSubstring("B: nope B"))
			})
		})
	})

	Describe("getting a result", func() {
		BeforeEach(func() {
			outSourceA.ResultStub = successResult(true)
			outSourceB.ResultStub = successResult(false)
		})

		It("collects aggregate results into a map", func() {
			result := map[string]Success{}
			Ω(source.Result(&result)).Should(BeTrue())

			Ω(result["A"]).Should(Equal(Success(true)))
			Ω(result["B"]).Should(Equal(Success(false)))
		})
	})
})
