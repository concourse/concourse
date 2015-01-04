package exec_test

import (
	"errors"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SuccessReporter", func() {
	var (
		successReporter SuccessReporter
	)

	BeforeEach(func() {
		successReporter = NewSuccessReporter()
	})

	Context("when no success indicators are subject", func() {
		It("is successful", func() {
			Ω(successReporter.Successful()).Should(BeTrue())
		})
	})

	Context("when a step is subject", func() {
		var (
			inSource *fakes.FakeArtifactSource

			firstStep       *fakes.FakeStep
			firstStepSource *fakes.FakeArtifactSource

			firstProcess ifrit.Process
		)

		BeforeEach(func() {
			inSource = new(fakes.FakeArtifactSource)

			firstStep = new(fakes.FakeStep)

			firstStepSource = new(fakes.FakeArtifactSource)
			firstStep.UsingReturns(firstStepSource)
		})

		JustBeforeEach(func() {
			firstProcess = ifrit.Invoke(successReporter.Subject(firstStep).Using(inSource))
		})

		Context("when the source is not successful", func() {
			BeforeEach(func() {
				firstStepSource.ResultStub = successResult(false)
			})

			Context("when the wrapped step's source exits successfully", func() {
				BeforeEach(func() {
					firstStepSource.RunReturns(nil)
				})

				It("forwards the input source to the wrapped step", func() {
					Ω(firstStep.UsingCallCount()).Should(Equal(1))
					Ω(firstStep.UsingArgsForCall(0)).Should(Equal(inSource))
				})

				It("exits successfully", func() {
					Eventually(firstProcess.Wait()).Should(Receive(BeNil()))
				})

				It("is no longer successful", func() {
					Eventually(firstProcess.Wait()).Should(Receive(BeNil()))

					Ω(successReporter.Successful()).Should(BeFalse())
				})

				Context("when a second step is subject", func() {
					var (
						inSource *fakes.FakeArtifactSource

						secondStep       *fakes.FakeStep
						secondStepSource *fakes.FakeArtifactSource

						secondProcess ifrit.Process
					)

					BeforeEach(func() {
						inSource = new(fakes.FakeArtifactSource)

						secondStep = new(fakes.FakeStep)

						secondStepSource = new(fakes.FakeArtifactSource)
						secondStep.UsingReturns(secondStepSource)
					})

					JustBeforeEach(func() {
						Eventually(firstProcess.Wait()).Should(Receive())
						secondProcess = ifrit.Invoke(successReporter.Subject(secondStep).Using(inSource))
					})

					Context("when the source is successful", func() {
						BeforeEach(func() {
							secondStepSource.ResultStub = successResult(true)
						})

						Context("when the wrapped step's source exits successfully", func() {
							BeforeEach(func() {
								secondStepSource.RunReturns(nil)
							})

							It("forwards the input source to the wrapped step", func() {
								Ω(secondStep.UsingCallCount()).Should(Equal(1))
								Ω(secondStep.UsingArgsForCall(0)).Should(Equal(inSource))
							})

							It("exits successfully", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))
							})

							It("is still unsuccessful", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})

						Context("when the wrapped step exits with failure", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								secondStepSource.RunReturns(disaster)
							})

							It("propagates the error", func() {
								Eventually(secondProcess.Wait()).Should(Receive(Equal(disaster)))
							})

							It("is still unsuccessful", func() {
								Eventually(secondProcess.Wait()).Should(Receive())

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})
					})

					Context("when the source is not successful", func() {
						BeforeEach(func() {
							secondStepSource.ResultStub = successResult(false)
						})

						Context("when the wrapped step's source exits successfully", func() {
							BeforeEach(func() {
								secondStepSource.RunReturns(nil)
							})

							It("forwards the input source to the wrapped step", func() {
								Ω(secondStep.UsingCallCount()).Should(Equal(1))
								Ω(secondStep.UsingArgsForCall(0)).Should(Equal(inSource))
							})

							It("exits successfully", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))
							})

							It("is no longer successful", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})

						Context("when the wrapped step exits with failure", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								secondStepSource.RunReturns(disaster)
							})

							It("propagates the error", func() {
								Eventually(secondProcess.Wait()).Should(Receive(Equal(disaster)))
							})

							It("is no longer successful", func() {
								Eventually(secondProcess.Wait()).Should(Receive())

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})
					})
				})
			})

			Context("when the wrapped step exits with failure", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					firstStepSource.RunReturns(disaster)
				})

				It("propagates the error", func() {
					Eventually(firstProcess.Wait()).Should(Receive(Equal(disaster)))
				})

				It("is no longer successful", func() {
					Eventually(firstProcess.Wait()).Should(Receive())

					Ω(successReporter.Successful()).Should(BeFalse())
				})
			})
		})

		Context("when the source is successful", func() {
			BeforeEach(func() {
				firstStepSource.ResultStub = successResult(true)
			})

			Context("when the wrapped step's source exits successfully", func() {
				BeforeEach(func() {
					firstStepSource.RunReturns(nil)
				})

				It("forwards the input source to the wrapped step", func() {
					Ω(firstStep.UsingCallCount()).Should(Equal(1))
					Ω(firstStep.UsingArgsForCall(0)).Should(Equal(inSource))
				})

				It("exits successfully", func() {
					Eventually(firstProcess.Wait()).Should(Receive(BeNil()))
				})

				It("is still successful", func() {
					Eventually(firstProcess.Wait()).Should(Receive(BeNil()))

					Ω(successReporter.Successful()).Should(BeTrue())
				})

				Context("when a second step is subject", func() {
					var (
						inSource *fakes.FakeArtifactSource

						secondStep       *fakes.FakeStep
						secondStepSource *fakes.FakeArtifactSource

						secondProcess ifrit.Process
					)

					BeforeEach(func() {
						inSource = new(fakes.FakeArtifactSource)

						secondStep = new(fakes.FakeStep)

						secondStepSource = new(fakes.FakeArtifactSource)
						secondStep.UsingReturns(secondStepSource)
					})

					JustBeforeEach(func() {
						Eventually(firstProcess.Wait()).Should(Receive())
						secondProcess = ifrit.Invoke(successReporter.Subject(secondStep).Using(inSource))
					})

					Context("when the source is successful", func() {
						BeforeEach(func() {
							secondStepSource.ResultStub = successResult(true)
						})

						Context("when the wrapped step's source exits successfully", func() {
							BeforeEach(func() {
								secondStepSource.RunReturns(nil)
							})

							It("forwards the input source to the wrapped step", func() {
								Ω(secondStep.UsingCallCount()).Should(Equal(1))
								Ω(secondStep.UsingArgsForCall(0)).Should(Equal(inSource))
							})

							It("exits successfully", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))
							})

							It("is still successful", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))

								Ω(successReporter.Successful()).Should(BeTrue())
							})
						})

						Context("when the wrapped step exits with failure", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								secondStepSource.RunReturns(disaster)
							})

							It("propagates the error", func() {
								Eventually(secondProcess.Wait()).Should(Receive(Equal(disaster)))
							})

							It("is no longer successful", func() {
								Eventually(secondProcess.Wait()).Should(Receive())

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})
					})

					Context("when the source is not successful", func() {
						BeforeEach(func() {
							secondStepSource.ResultStub = successResult(false)
						})

						Context("when the wrapped step's source exits successfully", func() {
							BeforeEach(func() {
								secondStepSource.RunReturns(nil)
							})

							It("forwards the input source to the wrapped step", func() {
								Ω(secondStep.UsingCallCount()).Should(Equal(1))
								Ω(secondStep.UsingArgsForCall(0)).Should(Equal(inSource))
							})

							It("exits successfully", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))
							})

							It("is no longer successful", func() {
								Eventually(secondProcess.Wait()).Should(Receive(BeNil()))

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})

						Context("when the wrapped step exits with failure", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								secondStepSource.RunReturns(disaster)
							})

							It("propagates the error", func() {
								Eventually(secondProcess.Wait()).Should(Receive(Equal(disaster)))
							})

							It("is no longer successful", func() {
								Eventually(secondProcess.Wait()).Should(Receive())

								Ω(successReporter.Successful()).Should(BeFalse())
							})
						})
					})
				})
			})

			Context("when the wrapped step exits with failure", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					firstStepSource.RunReturns(disaster)
				})

				It("propagates the error", func() {
					Eventually(firstProcess.Wait()).Should(Receive(Equal(disaster)))
				})

				It("is no longer successful", func() {
					Eventually(firstProcess.Wait()).Should(Receive())

					Ω(successReporter.Successful()).Should(BeFalse())
				})
			})
		})
	})
})
