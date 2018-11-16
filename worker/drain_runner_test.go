package worker_test

import (
	"errors"
	"os"
	"syscall"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("DrainRunner", func() {
	var fakeClient *workerfakes.FakeTSAClient
	var subRunner ifrit.Runner
	var drainSignals chan<- os.Signal
	var subSignals <-chan os.Signal
	var subRunning <-chan struct{}
	var subExit chan<- error

	var runner *worker.DrainRunner
	var process ifrit.Process

	BeforeEach(func() {
		fakeClient = new(workerfakes.FakeTSAClient)

		exit := make(chan error)
		running := make(chan struct{}, 1)

		subRunner = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			subSignals = signals
			close(running)
			return <-exit
		})

		subSignals = nil
		subRunning = running
		subExit = exit

		ds := make(chan os.Signal)
		drainSignals = ds

		runner = &worker.DrainRunner{
			Logger:       lagertest.NewTestLogger("test"),
			Client:       fakeClient,
			DrainSignals: ds,

			Runner: subRunner,
		}

		process = ifrit.Invoke(runner)

		<-subRunning
	})

	AfterEach(func() {
		close(subExit)
		<-process.Wait()
	})

	It("runs the sub-process", func() {
		<-subRunning
	})

	Describe("Drained", func() {
		It("returns false", func() {
			Expect(runner.Drained()).Should(BeFalse())
		})
	})

	Context("when syscall.SIGUSR1 is received", func() {
		JustBeforeEach(func() {
			drainSignals <- syscall.SIGUSR1
		})

		It("lands the worker", func() {
			Eventually(fakeClient.LandCallCount).Should(Equal(1))
		})

		It("does not forward the signal", func() {
			Consistently(subSignals).ShouldNot(Receive())
		})

		Describe("Drained", func() {
			It("returns true", func() {
				Eventually(runner.Drained).Should(BeTrue())
			})
		})

		Context("when landing the worker fails", func() {
			BeforeEach(func() {
				fakeClient.LandReturns(errors.New("nope"))
			})

			It("interrupts the sub-process", func() {
				Expect(<-subSignals).To(Equal(os.Interrupt))
			})

			It("waits for the sub-process to exit", func() {
				exit := errors.New("exiting")
				subExit <- exit
				Expect(<-process.Wait()).To(Equal(exit))
			})
		})

		Context("when syscall.SIGTERM is received after landing", func() {
			JustBeforeEach(func() {
				Eventually(fakeClient.LandCallCount).Should(Equal(1))
				process.Signal(syscall.SIGTERM)
			})

			It("does not delete the worker", func() {
				Consistently(fakeClient.DeleteCallCount).Should(Equal(0))
			})

			It("forwards the signal without deleting the worker", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGTERM))
				Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
			})

			Describe("Drained", func() {
				It("still returns true", func() {
					Consistently(runner.Drained).Should(BeTrue())
				})
			})
		})

		Context("when syscall.SIGINT is received after landing", func() {
			JustBeforeEach(func() {
				Eventually(fakeClient.LandCallCount).Should(Equal(1))
				process.Signal(syscall.SIGINT)
			})

			It("does not delete the worker", func() {
				Consistently(fakeClient.DeleteCallCount).Should(Equal(0))
			})

			It("forwards the signal without deleting the worker", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGINT))
				Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
			})

			Describe("Drained", func() {
				It("still returns true", func() {
					Consistently(runner.Drained).Should(BeTrue())
				})
			})
		})
	})

	Context("when syscall.SIGUSR2 is received", func() {
		JustBeforeEach(func() {
			drainSignals <- syscall.SIGUSR2
		})

		It("retires the worker", func() {
			Eventually(fakeClient.RetireCallCount).Should(Equal(1))
		})

		It("does not forward the signal", func() {
			Consistently(subSignals).ShouldNot(Receive())
		})

		Describe("Drained", func() {
			It("returns true", func() {
				Eventually(runner.Drained).Should(BeTrue())
			})
		})

		Context("when retiring the worker fails", func() {
			BeforeEach(func() {
				fakeClient.RetireReturns(errors.New("nope"))
			})

			It("interrupts the sub-process", func() {
				Expect(<-subSignals).To(Equal(os.Interrupt))
			})

			It("waits for the sub-process to exit", func() {
				exit := errors.New("exiting")
				subExit <- exit
				Expect(<-process.Wait()).To(Equal(exit))
			})
		})

		Context("when syscall.SIGTERM is received after retiring", func() {
			JustBeforeEach(func() {
				Eventually(fakeClient.RetireCallCount).Should(Equal(1))
				process.Signal(syscall.SIGTERM)
			})

			It("deletes the worker", func() {
				Eventually(fakeClient.DeleteCallCount).Should(Equal(1))
			})

			It("forwards the signal", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGTERM))
			})

			Describe("Drained", func() {
				It("still returns true", func() {
					Consistently(runner.Drained).Should(BeTrue())
				})
			})

			Context("when deleting the worker fails", func() {
				BeforeEach(func() {
					fakeClient.DeleteReturns(errors.New("nope"))
				})

				It("still forwards the signal", func() {
					Expect(<-subSignals).To(Equal(syscall.SIGTERM))
				})

				It("still waits for the sub-process to exit", func() {
					exit := errors.New("exiting")
					subExit <- exit
					Expect(<-process.Wait()).To(Equal(exit))
				})
			})
		})

		Context("when syscall.SIGINT is received after retiring", func() {
			JustBeforeEach(func() {
				Eventually(fakeClient.RetireCallCount).Should(Equal(1))
				process.Signal(syscall.SIGINT)
			})

			It("deletes the worker", func() {
				Eventually(fakeClient.DeleteCallCount).Should(Equal(1))
			})

			It("forwards the signal", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGINT))
			})

			Describe("Drained", func() {
				It("still returns true", func() {
					Consistently(runner.Drained).Should(BeTrue())
				})
			})

			Context("when deleting the worker fails", func() {
				BeforeEach(func() {
					fakeClient.DeleteReturns(errors.New("nope"))
				})

				It("still forwards the signal", func() {
					Expect(<-subSignals).To(Equal(syscall.SIGINT))
				})

				It("still waits for the sub-process to exit", func() {
					exit := errors.New("exiting")
					subExit <- exit
					Expect(<-process.Wait()).To(Equal(exit))
				})
			})
		})
	})

	Context("when syscall.SIGTERM is received", func() {
		JustBeforeEach(func() {
			process.Signal(syscall.SIGTERM)
		})

		It("forwards the signal without landing, retiring, or deleting the worker", func() {
			Expect(<-subSignals).To(Equal(syscall.SIGTERM))
			Expect(fakeClient.LandCallCount()).Should(Equal(0))
			Expect(fakeClient.RetireCallCount()).Should(Equal(0))
			Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
		})

		Describe("Drained", func() {
			It("returns false", func() {
				Consistently(runner.Drained).Should(BeFalse())
			})
		})
	})

	Context("when syscall.SIGINT is received", func() {
		JustBeforeEach(func() {
			process.Signal(syscall.SIGINT)
		})

		It("forwards the signal without landing, retiring, or deleting the worker", func() {
			Expect(<-subSignals).To(Equal(syscall.SIGINT))
			Expect(fakeClient.LandCallCount()).Should(Equal(0))
			Expect(fakeClient.RetireCallCount()).Should(Equal(0))
			Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
		})

		Describe("Drained", func() {
			It("returns false", func() {
				Consistently(runner.Drained).Should(BeFalse())
			})
		})
	})
})
