package exec_test

import (
	"errors"
	"os"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

// There are a few places in this test where we assert that the release should
// be called twice. This is because the hooked compose will try and run the
// next step regardless. If the step is nil, we will use an identity step,
// which defaults to returning whatever the previous step was from using. For
// this reason, the input step gets returned as the next step of type identity
// step, which returns nil when ran.

var _ = Describe("Hooked Compose", func() {
	var (
		fakeStepFactoryStep     *fakes.FakeStepFactory
		fakeStepFactoryNextStep *fakes.FakeStepFactory

		hookedCompose StepFactory

		inStep *fakes.FakeStep
		repo   *SourceRepository

		outStep  *fakes.FakeStep
		nextStep *fakes.FakeStep

		startStep  chan error
		finishStep chan error

		startNextStep  chan error
		finishNextStep chan error

		step    Step
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepFactoryStep = new(fakes.FakeStepFactory)
		fakeStepFactoryNextStep = new(fakes.FakeStepFactory)

		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()

		outStep = new(fakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(outStep)

		nextStep = new(fakes.FakeStep)
		fakeStepFactoryNextStep.UsingReturns(nextStep)

		startStep = make(chan error, 1)
		finishStep = make(chan error, 1)

		startNextStep = make(chan error, 1)
		finishNextStep = make(chan error, 1)

		outStep.ResultStub = successResult(true)
		outStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			select {
			case err := <-startStep:
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
			case err := <-finishStep:
				return err
			}
		}

		nextStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			select {
			case err := <-startNextStep:
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
			case err := <-finishNextStep:
				return err
			}
		}
	})

	AfterEach(func() {
		close(startStep)
		close(finishStep)

		close(startNextStep)
		close(finishNextStep)

		Eventually(process.Wait()).Should(Receive())
	})

	Context("and there are hooks", func() {
		Context("with a success hook", func() {
			var (
				ensureStepFactory StepFactory

				fakeStepFactorySuccessStep *fakes.FakeStepFactory

				successStep *fakes.FakeStep
				ensureStep  *fakes.FakeStep

				startSuccess  chan error
				finishSuccess chan error
			)

			BeforeEach(func() {
				fakeStepFactorySuccessStep = new(fakes.FakeStepFactory)

				successStep = new(fakes.FakeStep)
				fakeStepFactorySuccessStep.UsingReturns(successStep)
				successStep.ResultStub = successResult(true)

				ensureStepFactory = Identity{}

				startSuccess = make(chan error, 1)
				finishSuccess = make(chan error, 1)

				successStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					select {
					case err := <-startSuccess:
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
					case err := <-finishSuccess:
						return err
					}
				}
			})

			AfterEach(func() {
				close(startSuccess)
				close(finishSuccess)
			})

			JustBeforeEach(func() {
				hookedCompose = HookedCompose(fakeStepFactoryStep, fakeStepFactoryNextStep, Identity{}, fakeStepFactorySuccessStep, ensureStepFactory)
				step = hookedCompose.Using(inStep, repo)
				process = ifrit.Background(step)
			})

			Context("and an ensure hook", func() {
				var (
					fakeStepFactoryEnsureStep *fakes.FakeStepFactory

					startEnsure  chan error
					finishEnsure chan error
				)

				BeforeEach(func() {
					startEnsure = make(chan error, 1)
					finishEnsure = make(chan error, 1)

					fakeStepFactoryEnsureStep = new(fakes.FakeStepFactory)
					ensureStep = new(fakes.FakeStep)
					fakeStepFactoryEnsureStep.UsingReturns(ensureStep)

					ensureStepFactory = fakeStepFactoryEnsureStep

					ensureStep.ResultStub = successResult(true)
					ensureStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						select {
						case err := <-startEnsure:
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
						case err := <-finishEnsure:
							return err
						}
					}

				})

				AfterEach(func() {
					close(startEnsure)
					close(finishEnsure)
				})

				Context("and the first step finishes successfully", func() {
					BeforeEach(func() {
						startStep <- nil
						finishStep <- nil
					})

					It("executes the ensure step and success step in parallel", func() {
						Eventually(fakeStepFactoryEnsureStep.UsingCallCount).Should(Equal(1))
						Eventually(ensureStep.RunCallCount).Should(Equal(1))
						step, repo := fakeStepFactoryEnsureStep.UsingArgsForCall(0)
						Ω(step).Should(Equal(outStep))
						Ω(repo).Should(Equal(repo))

						Eventually(fakeStepFactorySuccessStep.UsingCallCount).Should(Equal(1))
						Eventually(successStep.RunCallCount).Should(Equal(1))
						step, repo = fakeStepFactorySuccessStep.UsingArgsForCall(0)
						Ω(step).Should(Equal(outStep))
						Ω(repo).Should(Equal(repo))
					})

					Context("and the ensure step cannot respond to success", func() {

						BeforeEach(func() {
							ensureStep.ResultReturns(false)

							startEnsure <- nil
							finishEnsure <- nil

							startSuccess <- nil
							finishSuccess <- nil
						})

						It("exits", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))
						})

						It("does not proceed to the next step", func() {
							Consistently(fakeStepFactoryNextStep.UsingCallCount).Should(BeZero())
						})
					})

					Context("and the ensure step exits with an error", func() {
						var err error

						BeforeEach(func() {
							err = errors.New("disaster")
							startEnsure <- nil
							finishEnsure <- err

							startSuccess <- nil
							finishSuccess <- nil
						})

						It("exits with its error result", func() {
							var receivedError error
							Eventually(process.Wait()).Should(Receive(&receivedError))
							Ω(receivedError.Error()).Should(ContainSubstring(err.Error()))
						})

						It("does not proceed to the next step", func() {
							Consistently(fakeStepFactoryNextStep.UsingCallCount).Should(BeZero())
						})
					})

					Context("when the ensure step and the success step exits with an error", func() {
						var errOne error
						var errTwo error

						BeforeEach(func() {
							errOne = errors.New("one disaster")
							errTwo = errors.New("two disaster")

							startEnsure <- nil
							finishEnsure <- errOne

							startSuccess <- nil
							finishSuccess <- errTwo
						})

						It("exits with a combined error result", func() {
							var receivedError error
							Eventually(process.Wait()).Should(Receive(&receivedError))
							Ω(receivedError.Error()).Should(ContainSubstring(errOne.Error()))
							Ω(receivedError.Error()).Should(ContainSubstring(errTwo.Error()))
						})

						It("does not proceed to the next step", func() {
							Consistently(fakeStepFactoryNextStep.UsingCallCount).Should(BeZero())
						})
					})

					Context("and the ensure step is not successful", func() {
						BeforeEach(func() {
							ensureStep.ResultStub = successResult(false)
							successStep.ResultStub = successResult(true)
							startEnsure <- nil
							finishEnsure <- nil

							startSuccess <- nil
							finishSuccess <- nil
						})

						It("does not proceed to the next step", func() {
							Consistently(fakeStepFactoryNextStep.UsingCallCount).Should(BeZero())
						})
					})

					Context("and the success step is not successful", func() {
						BeforeEach(func() {
							ensureStep.ResultStub = successResult(true)
							successStep.ResultStub = successResult(false)
							startEnsure <- nil
							finishEnsure <- nil

							startSuccess <- nil
							finishSuccess <- nil
						})

						It("does not proceed to the next step", func() {
							Consistently(fakeStepFactoryNextStep.UsingCallCount).Should(BeZero())
						})
					})
				})

				Context("and the first step finishes with an error", func() {
					var err error

					BeforeEach(func() {
						err = errors.New("disaster")
						startStep <- err
					})

					It("runs the ensure step", func() {
						Eventually(ensureStep.RunCallCount).Should(Equal(1))
					})

					It("does not run the success step", func() {
						Consistently(successStep.RunCallCount).Should(BeZero())
					})

					It("does not run the next step", func() {
						Consistently(nextStep.RunCallCount).Should(BeZero())
					})
				})
			})

			Context("while the success step is starting", func() {
				BeforeEach(func() {
					startStep <- nil
					finishStep <- nil
				})

				It("forwards the signal to the success step", func() {
					Consistently(process.Ready()).ShouldNot(BeClosed())

					Eventually(successStep.RunCallCount).Should(Equal(1))

					process.Signal(os.Interrupt)

					var receivedError error
					Eventually(process.Wait()).Should(Receive(&receivedError))
					Ω(receivedError.Error()).Should(ContainSubstring(ErrInterrupted.Error()))
				})
			})

			Context("while the success step is starting", func() {
				BeforeEach(func() {
					startStep <- nil
					finishStep <- nil

					startSuccess <- nil
				})

				It("forwards the signal to the success step", func() {
					Eventually(process.Ready()).ShouldNot(BeClosed())

					Eventually(successStep.RunCallCount).Should(Equal(1))

					Consistently(process.Wait()).ShouldNot(Receive())

					process.Signal(os.Interrupt)

					var receivedError error
					Eventually(process.Wait()).Should(Receive(&receivedError))
					Ω(receivedError.Error()).Should(ContainSubstring(ErrInterrupted.Error()))
				})
			})

			Context("when the success step is finished", func() {
				BeforeEach(func() {
					startStep <- nil
					finishStep <- nil

					startSuccess <- nil
					finishSuccess <- nil
				})

				It("forwards the signal to the next step", func() {
					Eventually(process.Ready()).ShouldNot(BeClosed())

					Eventually(successStep.RunCallCount).Should(Equal(1))

					Eventually(nextStep.RunCallCount).Should(Equal(1))

					Consistently(process.Wait()).ShouldNot(Receive())

					process.Signal(os.Interrupt)

					Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
				})
			})

			Context("while the next step is starting", func() {
				BeforeEach(func() {
					startStep <- nil
					finishStep <- nil

					startSuccess <- nil
					finishSuccess <- nil

					startNextStep <- nil
				})

				It("forwards the signal to the success step", func() {
					Eventually(process.Ready()).ShouldNot(BeClosed())

					Eventually(successStep.RunCallCount).Should(Equal(1))

					Eventually(nextStep.RunCallCount).Should(Equal(1))

					Consistently(process.Wait()).ShouldNot(Receive())

					process.Signal(os.Interrupt)

					Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
				})
			})

			Context("and the first step finishes successfully", func() {
				BeforeEach(func() {
					startStep <- nil
					finishStep <- nil
				})

				It("uses the first step's source as the input for the success step", func() {
					Eventually(fakeStepFactorySuccessStep.UsingCallCount).Should(Equal(1))
					step, repo := fakeStepFactorySuccessStep.UsingArgsForCall(0)
					Ω(step).Should(Equal(outStep))
					Ω(repo).Should(Equal(repo))
				})

				Context("and the success hook exits successfully", func() {
					BeforeEach(func() {
						startSuccess <- nil
						finishSuccess <- nil
					})

					It("uses the first step's source as the input for the next step", func() {
						Eventually(fakeStepFactoryNextStep.UsingCallCount).Should(Equal(1))
						step, repo := fakeStepFactoryNextStep.UsingArgsForCall(0)
						Ω(step).Should(Equal(outStep))
						Ω(repo).Should(Equal(repo))
					})

					Context("and the next source exits successfully", func() {
						BeforeEach(func() {
							startNextStep <- nil
							finishNextStep <- nil
						})

						It("exits successfully", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))
						})

						Describe("releasing", func() {
							It("releases all sources", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								err := step.Release()
								Ω(err).ShouldNot(HaveOccurred())

								Ω(outStep.ReleaseCallCount()).Should(Equal(2))
								Ω(successStep.ReleaseCallCount()).Should(Equal(1))
								Ω(nextStep.ReleaseCallCount()).Should(Equal(1))
							})

							Context("when releasing the sources fails", func() {
								disasterA := errors.New("nope A")
								disasterB := errors.New("nope B")
								disasterC := errors.New("nope C")

								BeforeEach(func() {
									outStep.ReleaseReturns(disasterA)
									successStep.ReleaseReturns(disasterB)
									nextStep.ReleaseReturns(disasterC)
								})

								It("returns an aggregate error", func() {
									Eventually(process.Wait()).Should(Receive(BeNil()))

									err := step.Release()
									Ω(err).Should(HaveOccurred())

									Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
									Ω(err.Error()).Should(ContainSubstring("success step: nope B"))
									Ω(err.Error()).Should(ContainSubstring("next step: nope C"))
								})
							})
						})

						Describe("getting the result", func() {
							BeforeEach(func() {
								nextStep.ResultStub = successResult(true)
							})

							It("delegates to the success source", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								var success Success
								Ω(step.Result(&success)).Should(BeTrue())
								Ω(bool(success)).Should(BeTrue())
							})
						})
					})
				})

				Context("and the success step exits with an error", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						startSuccess <- nil
						finishSuccess <- disaster
					})

					It("exits with its error result", func() {
						var receivedError error
						Eventually(process.Wait()).Should(Receive(&receivedError))
						Ω(receivedError.Error()).Should(ContainSubstring(disaster.Error()))
					})

					Describe("releasing", func() {
						It("releases the first source and success source", func() {
							Eventually(process.Wait()).Should(Receive())

							err := step.Release()
							Ω(err).ShouldNot(HaveOccurred())

							Ω(outStep.ReleaseCallCount()).Should(Equal(2))
							Ω(successStep.ReleaseCallCount()).Should(Equal(1))
							Ω(nextStep.ReleaseCallCount()).Should(Equal(0))
						})

						Context("when releasing the sources fails", func() {
							disasterA := errors.New("nope A")
							disasterB := errors.New("nope B")
							disasterC := errors.New("nope C")

							BeforeEach(func() {
								outStep.ReleaseReturns(disasterA)
								successStep.ReleaseReturns(disasterB)
								nextStep.ReleaseReturns(disasterC)
							})

							It("returns an aggregate error", func() {
								Eventually(process.Wait()).Should(Receive())

								err := step.Release()
								Ω(err).Should(HaveOccurred())

								Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
								Ω(err.Error()).Should(ContainSubstring("success step: nope B"))
								Ω(err.Error()).ShouldNot(ContainSubstring("next step: nope C"))
							})
						})
					})
				})

				Context("and the success source fails to start", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						startSuccess <- disaster
					})

					It("exits with its error result", func() {
						var receivedError error
						Eventually(process.Wait()).Should(Receive(&receivedError))
						Ω(receivedError.Error()).Should(ContainSubstring(disaster.Error()))
					})

					Describe("releasing", func() {
						It("releases the first source and success source", func() {
							Eventually(process.Wait()).Should(Receive())

							err := step.Release()
							Ω(err).ShouldNot(HaveOccurred())

							Ω(outStep.ReleaseCallCount()).Should(Equal(2))
							Ω(successStep.ReleaseCallCount()).Should(Equal(1))
							Ω(nextStep.ReleaseCallCount()).Should(Equal(0))
						})

						Context("when releasing the sources fails", func() {
							disasterA := errors.New("nope A")
							disasterB := errors.New("nope B")
							disasterC := errors.New("nope C")

							BeforeEach(func() {
								outStep.ReleaseReturns(disasterA)
								successStep.ReleaseReturns(disasterB)
								nextStep.ReleaseReturns(disasterC)
							})

							It("returns an aggregate error", func() {
								Eventually(process.Wait()).Should(Receive())

								err := step.Release()
								Ω(err).Should(HaveOccurred())

								Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
								Ω(err.Error()).Should(ContainSubstring("success step: nope B"))
								Ω(err.Error()).ShouldNot(ContainSubstring("next step: nope C"))
							})
						})
					})
				})
			})

			Context("when the first step is not successful", func() {
				BeforeEach(func() {
					outStep.ResultStub = successResult(false)

					startStep <- nil
					finishStep <- nil

					startSuccess <- nil
					finishSuccess <- nil

					startNextStep <- nil
					finishNextStep <- nil
				})

				It("does not proceed to the next step", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					Ω(fakeStepFactorySuccessStep.UsingCallCount()).Should(BeZero())
					Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
				})

				Describe("releasing", func() {
					It("releases the first source", func() {
						Eventually(process.Wait()).Should(Receive())

						err := step.Release()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(outStep.ReleaseCallCount()).Should(Equal(3))
						Ω(successStep.ReleaseCallCount()).Should(BeZero())
						Ω(nextStep.ReleaseCallCount()).Should(BeZero())
					})

					Context("when releasing the source fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							outStep.ReleaseReturns(disaster)
						})

						It("returns an aggregate error", func() {
							Eventually(process.Wait()).Should(Receive())

							err := step.Release()
							Ω(err).Should(HaveOccurred())

							Ω(err.Error()).Should(ContainSubstring("first step: nope"))
						})
					})
				})
			})
		})

		Context("with a failure hook", func() {
			var (
				ensureStepFactory          StepFactory
				fakeStepFactoryFailureStep *fakes.FakeStepFactory

				failureStep *fakes.FakeStep

				startFailure  chan error
				finishFailure chan error
			)

			BeforeEach(func() {
				ensureStepFactory = Identity{}
				fakeStepFactoryFailureStep = new(fakes.FakeStepFactory)

				failureStep = new(fakes.FakeStep)
				fakeStepFactoryFailureStep.UsingReturns(failureStep)
				failureStep.ResultStub = successResult(true)

				startFailure = make(chan error, 1)
				finishFailure = make(chan error, 1)

				failureStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					select {
					case err := <-startFailure:
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
					case err := <-finishFailure:
						return err
					}
				}
			})

			JustBeforeEach(func() {
				hookedCompose = HookedCompose(
					fakeStepFactoryStep,
					fakeStepFactoryNextStep,
					fakeStepFactoryFailureStep,
					Identity{},
					ensureStepFactory,
				)
				step = hookedCompose.Using(inStep, repo)
				process = ifrit.Background(step)
			})

			AfterEach(func() {
				close(startFailure)
				close(finishFailure)
			})

			Context("and an ensure hook", func() {
				var (
					fakeStepFactoryEnsureStep *fakes.FakeStepFactory
					ensureStep                *fakes.FakeStep
					startEnsure               chan error
					finishEnsure              chan error
				)

				BeforeEach(func() {
					fakeStepFactoryEnsureStep = new(fakes.FakeStepFactory)

					ensureStep = new(fakes.FakeStep)
					fakeStepFactoryEnsureStep.UsingReturns(ensureStep)

					ensureStepFactory = fakeStepFactoryEnsureStep

					startEnsure = make(chan error, 1)
					finishEnsure = make(chan error, 1)

					ensureStep.ResultStub = successResult(true)
					ensureStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						select {
						case err := <-startEnsure:
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
						case err := <-finishEnsure:
							return err
						}
					}
				})

				AfterEach(func() {
					close(startEnsure)
					close(finishEnsure)
				})

				Context("and the first step finishes with a failure", func() {
					BeforeEach(func() {
						outStep.ResultStub = successResult(false)

						startStep <- nil
						finishStep <- nil
					})

					It("executes the ensure step and failure step in parallel", func() {
						Eventually(fakeStepFactoryEnsureStep.UsingCallCount).Should(Equal(1))
						Eventually(ensureStep.RunCallCount).Should(Equal(1))
						step, repo := fakeStepFactoryEnsureStep.UsingArgsForCall(0)
						Ω(step).Should(Equal(outStep))
						Ω(repo).Should(Equal(repo))

						Eventually(fakeStepFactoryFailureStep.UsingCallCount).Should(Equal(1))
						Eventually(failureStep.RunCallCount).Should(Equal(1))
						step, repo = fakeStepFactoryFailureStep.UsingArgsForCall(0)
						Ω(step).Should(Equal(outStep))
						Ω(repo).Should(Equal(repo))
					})

					Context("and the failure hook exits successfully", func() {
						BeforeEach(func() {
							startFailure <- nil
							finishFailure <- nil

							startEnsure <- nil
							finishEnsure <- nil
						})

						It("does not proceed to the next step", func() {
							Consistently(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
						})
					})

					Context("and the failure step exits with an error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							startFailure <- nil
							finishFailure <- disaster

							startEnsure <- nil
							finishEnsure <- nil
						})

						It("exits with its error result", func() {
							var receivedError error
							Eventually(process.Wait()).Should(Receive(&receivedError))
							Ω(receivedError.Error()).Should(ContainSubstring(disaster.Error()))
						})

						Describe("releasing", func() {
							It("releases the first source, failure source, and ensure source", func() {
								Eventually(process.Wait()).Should(Receive())

								err := step.Release()
								Ω(err).ShouldNot(HaveOccurred())

								Ω(outStep.ReleaseCallCount()).Should(Equal(1))
								Ω(failureStep.ReleaseCallCount()).Should(Equal(1))
								Ω(ensureStep.ReleaseCallCount()).Should(Equal(1))
								Ω(nextStep.ReleaseCallCount()).Should(Equal(0))
							})

							Context("when releasing the sources fails", func() {
								disasterA := errors.New("nope A")
								disasterB := errors.New("nope B")
								disasterC := errors.New("nope C")
								disasterD := errors.New("nope D")

								BeforeEach(func() {
									outStep.ReleaseReturns(disasterA)
									failureStep.ReleaseReturns(disasterB)
									ensureStep.ReleaseReturns(disasterC)
									nextStep.ReleaseReturns(disasterD)
								})

								It("returns an aggregate error", func() {
									Eventually(process.Wait()).Should(Receive())

									err := step.Release()
									Ω(err).Should(HaveOccurred())

									Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
									Ω(err.Error()).Should(ContainSubstring("failure step: nope B"))
									Ω(err.Error()).Should(ContainSubstring("ensure step: nope C"))
									Ω(err.Error()).ShouldNot(ContainSubstring("next step: nope D"))
								})
							})
						})
					})

					Context("and the failure step fails to start", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							startFailure <- disaster

							startEnsure <- nil
							finishEnsure <- nil
						})

						It("exits with its error result", func() {
							var receivedError error
							Eventually(process.Wait()).Should(Receive(&receivedError))
							Ω(receivedError.Error()).Should(ContainSubstring(disaster.Error()))
						})

						Describe("releasing", func() {
							It("releases the first source and success source", func() {
								Eventually(process.Wait()).Should(Receive())

								err := step.Release()
								Ω(err).ShouldNot(HaveOccurred())

								Ω(outStep.ReleaseCallCount()).Should(Equal(1))
								Ω(failureStep.ReleaseCallCount()).Should(Equal(1))
								Ω(nextStep.ReleaseCallCount()).Should(Equal(0))
							})

							Context("when releasing the sources fails", func() {
								disasterA := errors.New("nope A")
								disasterB := errors.New("nope B")
								disasterC := errors.New("nope C")

								BeforeEach(func() {
									outStep.ReleaseReturns(disasterA)
									failureStep.ReleaseReturns(disasterB)
									nextStep.ReleaseReturns(disasterC)
								})

								It("returns an aggregate error", func() {
									Eventually(process.Wait()).Should(Receive())

									err := step.Release()
									Ω(err).Should(HaveOccurred())

									Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
									Ω(err.Error()).Should(ContainSubstring("failure step: nope B"))
									Ω(err.Error()).ShouldNot(ContainSubstring("next step: nope C"))
								})
							})
						})
					})
				})
			})

			Context("when the first source exits successfully", func() {
				BeforeEach(func() {
					outStep.ResultStub = successResult(true)
					failureStep.ResultStub = successResult(true)

					startStep <- nil
					finishStep <- nil

					startNextStep <- nil
					finishNextStep <- nil
				})

				It("does not run the failure step", func() {
					Eventually(process.Wait()).Should(Receive())
					Ω(fakeStepFactoryFailureStep.UsingCallCount()).Should(BeZero())
				})

				It("still runs the next step", func() {
					Eventually(process.Wait()).Should(Receive())
					Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(Equal(1))
				})
			})
		})

	})

	Context("with a step and next step", func() {
		BeforeEach(func() {
			hookedCompose = HookedCompose(fakeStepFactoryStep, fakeStepFactoryNextStep, Identity{}, Identity{}, Identity{})
			step = hookedCompose.Using(inStep, repo)
			process = ifrit.Background(step)
		})

		Context("when the first step is starting", func() {
			It("forwards the signal to the first step and does not continue", func() {
				Consistently(process.Ready()).ShouldNot(Receive())

				process.Signal(os.Interrupt)

				var receivedError error
				Eventually(process.Wait()).Should(Receive(&receivedError))
				Ω(receivedError.Error()).Should(ContainSubstring(ErrInterrupted.Error()))

				Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
			})
		})

		Context("while the first step is running", func() {
			BeforeEach(func() {
				startStep <- nil
			})

			It("forwards the signal to the first step and does not continue", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())

				process.Signal(os.Interrupt)

				var receivedError error
				Eventually(process.Wait()).Should(Receive(&receivedError))
				Ω(receivedError.Error()).Should(ContainSubstring(ErrInterrupted.Error()))
				Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
			})
		})

		Context("while the next step is starting", func() {
			BeforeEach(func() {
				startStep <- nil
				finishStep <- nil
			})

			It("forwards the signal to the next step", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())

				Eventually(nextStep.RunCallCount).Should(Equal(1))

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
			})
		})

		Context("while the next step is running", func() {
			BeforeEach(func() {
				startStep <- nil
				finishStep <- nil

				startNextStep <- nil
			})

			It("forwards the signal to the next step", func() {
				Eventually(process.Ready()).Should(BeClosed())

				Eventually(nextStep.RunCallCount).Should(Equal(1))

				Consistently(process.Wait()).ShouldNot(Receive())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
			})
		})

		Context("when the first source exits successfully", func() {
			BeforeEach(func() {
				startStep <- nil
				finishStep <- nil
			})

			It("uses the input source for the first step", func() {
				Eventually(fakeStepFactoryStep.UsingCallCount).Should(Equal(1))
				step, repo := fakeStepFactoryStep.UsingArgsForCall(0)
				Ω(step).Should(Equal(inStep))
				Ω(repo).Should(Equal(repo))
			})

			It("uses the first step's source as the input for the next step", func() {
				Eventually(fakeStepFactoryNextStep.UsingCallCount).Should(Equal(1))
				step, repo := fakeStepFactoryNextStep.UsingArgsForCall(0)
				Ω(step).Should(Equal(outStep))
				Ω(repo).Should(Equal(repo))
			})

			Context("and the next source exits successfully", func() {
				BeforeEach(func() {
					startNextStep <- nil
					finishNextStep <- nil
				})

				It("exits successfully", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})

				Describe("releasing", func() {
					It("releases both sources", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						err := step.Release()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(outStep.ReleaseCallCount()).Should(Equal(3))
						Ω(nextStep.ReleaseCallCount()).Should(Equal(1))
					})

					Context("when releasing the sources fails", func() {
						disasterA := errors.New("nope A")
						disasterB := errors.New("nope B")

						BeforeEach(func() {
							outStep.ReleaseReturns(disasterA)
							nextStep.ReleaseReturns(disasterB)
						})

						It("returns an aggregate error", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							err := step.Release()
							Ω(err).Should(HaveOccurred())

							Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
							Ω(err.Error()).Should(ContainSubstring("next step: nope B"))
						})
					})
				})

				Describe("getting the result", func() {
					BeforeEach(func() {
						nextStep.ResultStub = successResult(true)
					})

					It("delegates to the next source", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						var success Success
						Ω(step.Result(&success)).Should(BeTrue())
						Ω(bool(success)).Should(BeTrue())
					})
				})
			})

			Context("and the next source exits with an error", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					startNextStep <- nil
					finishNextStep <- disaster
				})

				It("exits with its error result", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				Describe("releasing", func() {
					It("releases both sources", func() {
						Eventually(process.Wait()).Should(Receive())

						err := step.Release()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(outStep.ReleaseCallCount()).Should(Equal(3))
						Ω(nextStep.ReleaseCallCount()).Should(Equal(1))
					})

					Context("when releasing the sources fails", func() {
						disasterA := errors.New("nope A")
						disasterB := errors.New("nope B")

						BeforeEach(func() {
							outStep.ReleaseReturns(disasterA)
							nextStep.ReleaseReturns(disasterB)
						})

						It("returns an aggregate error", func() {
							Eventually(process.Wait()).Should(Receive())

							err := step.Release()
							Ω(err).Should(HaveOccurred())

							Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
							Ω(err.Error()).Should(ContainSubstring("next step: nope B"))
						})
					})
				})
			})

			Context("and the next source fails to start", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					startNextStep <- disaster
				})

				It("exits with its error result", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				Describe("releasing", func() {
					It("releases both sources", func() {
						Eventually(process.Wait()).Should(Receive())

						err := step.Release()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(outStep.ReleaseCallCount()).Should(Equal(3))
						Ω(nextStep.ReleaseCallCount()).Should(Equal(1))
					})

					Context("when releasing the sources fails", func() {
						disasterA := errors.New("nope A")
						disasterB := errors.New("nope B")

						BeforeEach(func() {
							outStep.ReleaseReturns(disasterA)
							nextStep.ReleaseReturns(disasterB)
						})

						It("returns an aggregate error", func() {
							Eventually(process.Wait()).Should(Receive())

							err := step.Release()
							Ω(err).Should(HaveOccurred())

							Ω(err.Error()).Should(ContainSubstring("first step: nope A"))
							Ω(err.Error()).Should(ContainSubstring("next step: nope B"))
						})
					})
				})
			})
		})

		Context("when the first source fails to start", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				startStep <- disaster
			})

			It("exits with its error result", func() {
				var receivedError error
				Eventually(process.Wait()).Should(Receive(&receivedError))
				Ω(receivedError.Error()).Should(ContainSubstring(disaster.Error()))
			})

			It("does not proceed to the next step", func() {
				Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
			})

			Describe("releasing", func() {
				It("releases the first source", func() {
					Eventually(process.Wait()).Should(Receive())

					err := step.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outStep.ReleaseCallCount()).Should(Equal(2))
					Ω(nextStep.ReleaseCallCount()).Should(BeZero())
				})

				Context("when releasing the source fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						outStep.ReleaseReturns(disaster)
					})

					It("returns an aggregate error", func() {
						Eventually(process.Wait()).Should(Receive())

						err := step.Release()
						Ω(err).Should(HaveOccurred())

						Ω(err.Error()).Should(ContainSubstring("first step: nope"))
					})
				})
			})
		})

		Context("when the first source exits with an error", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				startStep <- nil
				finishStep <- disaster
			})

			It("exits with its error result", func() {
				var receivedError error
				Eventually(process.Wait()).Should(Receive(&receivedError))
				Ω(receivedError.Error()).Should(ContainSubstring(disaster.Error()))
			})

			It("does not proceed to the next step", func() {
				Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
			})

			Describe("releasing", func() {
				It("releases the first source", func() {
					Eventually(process.Wait()).Should(Receive())

					err := step.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outStep.ReleaseCallCount()).Should(Equal(2))
					Ω(nextStep.ReleaseCallCount()).Should(BeZero())
				})

				Context("when releasing the source fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						outStep.ReleaseReturns(disaster)
					})

					It("returns an aggregate error", func() {
						Eventually(process.Wait()).Should(Receive())

						err := step.Release()
						Ω(err).Should(HaveOccurred())

						Ω(err.Error()).Should(ContainSubstring("first step: nope"))
					})
				})
			})
		})

		Context("when the first source is not successful", func() {
			BeforeEach(func() {
				outStep.ResultStub = successResult(false)

				startStep <- nil
				finishStep <- nil

				startNextStep <- nil
			})

			It("does not proceed to the next step", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))
				Ω(fakeStepFactoryNextStep.UsingCallCount()).Should(BeZero())
			})

			Describe("releasing", func() {
				It("releases the first source", func() {
					Eventually(process.Wait()).Should(Receive())

					err := step.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(outStep.ReleaseCallCount()).Should(Equal(3))
					Ω(nextStep.ReleaseCallCount()).Should(BeZero())
				})

				Context("when releasing the source fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						outStep.ReleaseReturns(disaster)
					})

					It("returns an aggregate error", func() {
						Eventually(process.Wait()).Should(Receive())

						err := step.Release()
						Ω(err).Should(HaveOccurred())

						Ω(err.Error()).Should(ContainSubstring("first step: nope"))
					})
				})
			})
		})
	})
})
