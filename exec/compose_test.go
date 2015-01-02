package exec_test

import (
	"errors"
	"io"
	"os"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Compose", func() {
	var (
		fakeStepA *fakes.FakeStep
		fakeStepB *fakes.FakeStep

		compose Step

		inSource *fakes.FakeArtifactSource

		outSourceA *fakes.FakeArtifactSource
		outSourceB *fakes.FakeArtifactSource

		startA  chan error
		finishA chan error

		startB  chan error
		finishB chan error

		source  ArtifactSource
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepA = new(fakes.FakeStep)
		fakeStepB = new(fakes.FakeStep)

		compose = Compose(fakeStepA, fakeStepB)

		inSource = new(fakes.FakeArtifactSource)

		outSourceA = new(fakes.FakeArtifactSource)
		fakeStepA.UsingReturns(outSourceA)

		outSourceB = new(fakes.FakeArtifactSource)
		fakeStepB.UsingReturns(outSourceB)

		startA = make(chan error, 1)
		finishA = make(chan error, 1)

		startB = make(chan error, 1)
		finishB = make(chan error, 1)

		outSourceA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			select {
			case err := <-startA:
				if err != nil {
					return err
				}
			case <-signals:
				return ErrInterrupted
			}

			close(ready)

			select {
			case <-signals:
				return ErrInterrupted
			case err := <-finishA:
				return err
			}
		}

		outSourceB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			select {
			case err := <-startB:
				if err != nil {
					return err
				}
			case <-signals:
				return ErrInterrupted
			}

			close(ready)

			select {
			case <-signals:
				return ErrInterrupted
			case err := <-finishB:
				return err
			}
		}
	})

	JustBeforeEach(func() {
		source = compose.Using(inSource)
		process = ifrit.Background(source)
	})

	AfterEach(func() {
		close(startA)
		close(finishA)

		close(startB)
		close(finishB)

		Eventually(process.Wait()).Should(Receive())
	})

	Describe("signalling", func() {
		Context("when the first step is starting", func() {
			It("forwards the signal to the first step and does not continue", func() {
				Consistently(process.Ready()).ShouldNot(Receive())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

				Ω(fakeStepB.UsingCallCount()).Should(BeZero())
			})
		})

		Context("while the first step is running", func() {
			BeforeEach(func() {
				startA <- nil
			})

			It("forwards the signal to the first step and does not continue", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

				Ω(fakeStepB.UsingCallCount()).Should(BeZero())
			})
		})

		Context("while the second step is starting", func() {
			BeforeEach(func() {
				startA <- nil
				finishA <- nil
			})

			It("forwards the signal to the second step", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())

				Eventually(outSourceB.RunCallCount).Should(Equal(1))

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
			})
		})

		Context("while the second step is running", func() {
			BeforeEach(func() {
				startA <- nil
				finishA <- nil

				startB <- nil
			})

			It("forwards the signal to the second step", func() {
				Eventually(process.Ready()).Should(BeClosed())

				Eventually(outSourceB.RunCallCount).Should(Equal(1))

				Consistently(process.Wait()).ShouldNot(Receive())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
			})
		})
	})

	Context("when the first source exits successfully", func() {
		BeforeEach(func() {
			startA <- nil
			finishA <- nil
		})

		It("uses the input source for the first step", func() {
			Eventually(fakeStepA.UsingCallCount).Should(Equal(1))
			Ω(fakeStepA.UsingArgsForCall(0)).Should(Equal(inSource))
		})

		It("uses the first step's source as the input for the second step", func() {
			Eventually(fakeStepB.UsingCallCount).Should(Equal(1))
			Ω(fakeStepB.UsingArgsForCall(0)).Should(Equal(outSourceA))
		})

		Context("and the second source exits successfully", func() {
			BeforeEach(func() {
				startB <- nil
				finishB <- nil
			})

			It("exits successfully", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))
			})

			Describe("releasing", func() {
				It("releases both sources", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					err := source.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outSourceA.ReleaseCallCount()).Should(Equal(1))
					Ω(outSourceB.ReleaseCallCount()).Should(Equal(1))
				})

				Context("when releasing the sources fails", func() {
					disasterA := errors.New("nope A")
					disasterB := errors.New("nope B")

					BeforeEach(func() {
						outSourceA.ReleaseReturns(disasterA)
						outSourceB.ReleaseReturns(disasterB)
					})

					It("returns an aggregate error", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						err := source.Release()
						Ω(err).Should(HaveOccurred())

						Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
						Ω(err.Error()).Should(ContainSubstring("second step: nope B"))
					})
				})
			})

			Describe("streaming to a destination", func() {
				var fakeDestination *fakes.FakeArtifactDestination

				BeforeEach(func() {
					fakeDestination = new(fakes.FakeArtifactDestination)
				})

				It("delegates to the second step's artifact source", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					err := source.StreamTo(fakeDestination)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outSourceA.StreamToCallCount()).Should(Equal(0))

					Ω(outSourceB.StreamToCallCount()).Should(Equal(1))
					Ω(outSourceB.StreamToArgsForCall(0)).Should(Equal(fakeDestination))
				})

				Context("when the second step's source fails to stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						outSourceB.StreamToReturns(disaster)
					})

					It("returns the error", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						err := source.StreamTo(fakeDestination)
						Ω(err).Should(Equal(disaster))
					})
				})
			})

			Describe("streaming a file out", func() {
				var outStream io.ReadCloser

				BeforeEach(func() {
					outStream = gbytes.NewBuffer()
					outSourceB.StreamFileReturns(outStream, nil)
				})

				It("delegates to the second step's artifact source", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					reader, err := source.StreamFile("some-file")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outSourceB.StreamFileCallCount()).Should(Equal(1))
					Ω(outSourceB.StreamFileArgsForCall(0)).Should(Equal("some-file"))

					Ω(reader).Should(Equal(outStream))
				})

				Context("when the output source fails to stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						outSourceB.StreamFileReturns(nil, disaster)
					})

					It("returns the error", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						_, err := source.StreamFile("some-file")
						Ω(err).Should(Equal(disaster))
					})
				})
			})
		})

		Context("and the second source exits with an error", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				startB <- nil
				finishB <- disaster
			})

			It("exits with its error result", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})

			Describe("releasing", func() {
				It("releases both sources", func() {
					Eventually(process.Wait()).Should(Receive())

					err := source.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outSourceA.ReleaseCallCount()).Should(Equal(1))
					Ω(outSourceB.ReleaseCallCount()).Should(Equal(1))
				})

				Context("when releasing the sources fails", func() {
					disasterA := errors.New("nope A")
					disasterB := errors.New("nope B")

					BeforeEach(func() {
						outSourceA.ReleaseReturns(disasterA)
						outSourceB.ReleaseReturns(disasterB)
					})

					It("returns an aggregate error", func() {
						Eventually(process.Wait()).Should(Receive())

						err := source.Release()
						Ω(err).Should(HaveOccurred())

						Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
						Ω(err.Error()).Should(ContainSubstring("second step: nope B"))
					})
				})
			})
		})

		Context("and the second source fails to start", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				startB <- disaster
			})

			It("exits with its error result", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})

			Describe("releasing", func() {
				It("releases both sources", func() {
					Eventually(process.Wait()).Should(Receive())

					err := source.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outSourceA.ReleaseCallCount()).Should(Equal(1))
					Ω(outSourceB.ReleaseCallCount()).Should(Equal(1))
				})

				Context("when releasing the sources fails", func() {
					disasterA := errors.New("nope A")
					disasterB := errors.New("nope B")

					BeforeEach(func() {
						outSourceA.ReleaseReturns(disasterA)
						outSourceB.ReleaseReturns(disasterB)
					})

					It("returns an aggregate error", func() {
						Eventually(process.Wait()).Should(Receive())

						err := source.Release()
						Ω(err).Should(HaveOccurred())

						Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
						Ω(err.Error()).Should(ContainSubstring("second step: nope B"))
					})
				})
			})
		})
	})

	Context("when the first source fails to start", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			startA <- disaster
		})

		It("exits with its error result", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))
		})

		It("does not proceed to the second step", func() {
			Ω(fakeStepB.UsingCallCount()).Should(BeZero())
		})

		Describe("releasing", func() {
			It("releases the first source", func() {
				Eventually(process.Wait()).Should(Receive())

				err := source.Release()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outSourceA.ReleaseCallCount()).Should(Equal(1))
				Ω(outSourceB.ReleaseCallCount()).Should(BeZero())
			})

			Context("when releasing the source fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outSourceA.ReleaseReturns(disaster)
				})

				It("returns an aggregate error", func() {
					Eventually(process.Wait()).Should(Receive())

					err := source.Release()
					Ω(err).Should(HaveOccurred())

					Ω(err.Error()).Should(ContainSubstring("first step: nope"))
				})
			})
		})
	})

	Context("when the first source exits with an error", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			startA <- nil
			finishA <- disaster
		})

		It("exits with its error result", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))
		})

		It("does not proceed to the second step", func() {
			Ω(fakeStepB.UsingCallCount()).Should(BeZero())
		})

		Describe("releasing", func() {
			It("releases the first source", func() {
				Eventually(process.Wait()).Should(Receive())

				err := source.Release()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outSourceA.ReleaseCallCount()).Should(Equal(1))
				Ω(outSourceB.ReleaseCallCount()).Should(BeZero())
			})

			Context("when releasing the source fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outSourceA.ReleaseReturns(disaster)
				})

				It("returns an aggregate error", func() {
					Eventually(process.Wait()).Should(Receive())

					err := source.Release()
					Ω(err).Should(HaveOccurred())

					Ω(err.Error()).Should(ContainSubstring("first step: nope"))
				})
			})
		})
	})
})
