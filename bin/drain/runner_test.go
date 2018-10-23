package drain_test

import (
	"errors"
	"os"
	"syscall"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/bin/drain"
	"github.com/concourse/concourse/worker/beacon/beaconfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("DrainRunner", func() {
	var fakeBeaconClient *beaconfakes.FakeBeaconClient
	var subRunner ifrit.Runner
	var subSignals <-chan os.Signal
	var subRunning <-chan struct{}
	var subExit chan<- error
	var process ifrit.Process

	BeforeEach(func() {
		fakeBeaconClient = new(beaconfakes.FakeBeaconClient)

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

		process = ifrit.Invoke(drain.Runner{
			Logger: lagertest.NewTestLogger("test"),
			Beacon: fakeBeaconClient,
			Runner: subRunner,
		})

		<-subRunning
	})

	AfterEach(func() {
		close(subExit)
		<-process.Wait()
	})

	It("runs the sub-process", func() {
		<-subRunning
	})

	Context("when syscall.SIGUSR1 is received", func() {
		JustBeforeEach(func() {
			process.Signal(syscall.SIGUSR1)
		})

		It("lands the worker", func() {
			Eventually(fakeBeaconClient.LandWorkerCallCount).Should(Equal(1))
		})

		It("does not forward the signal", func() {
			Consistently(subSignals).ShouldNot(Receive())
		})

		Context("when landing the worker fails", func() {
			BeforeEach(func() {
				fakeBeaconClient.LandWorkerReturns(errors.New("nope"))
			})

			It("still waits for the sub-process to exit", func() {
				exit := errors.New("exiting")
				subExit <- exit
				Expect(<-process.Wait()).To(Equal(exit))
			})
		})

		Context("when syscall.SIGTERM is received", func() {
			BeforeEach(func() {
				process.Signal(syscall.SIGTERM)
			})

			It("forwards the signal without deleting the worker", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGTERM))
				Expect(fakeBeaconClient.DeleteWorkerCallCount()).Should(Equal(0))
			})
		})

		Context("when syscall.SIGINT is received", func() {
			BeforeEach(func() {
				process.Signal(syscall.SIGINT)
			})

			It("forwards the signal without deleting the worker", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGINT))
				Expect(fakeBeaconClient.DeleteWorkerCallCount()).Should(Equal(0))
			})
		})
	})

	Context("when syscall.SIGUSR2 is received", func() {
		JustBeforeEach(func() {
			process.Signal(syscall.SIGUSR2)
		})

		It("retires the worker", func() {
			Eventually(fakeBeaconClient.RetireWorkerCallCount).Should(Equal(1))
		})

		It("does not forward the signal", func() {
			Consistently(subSignals).ShouldNot(Receive())
		})

		Context("when retiring the worker fails", func() {
			BeforeEach(func() {
				fakeBeaconClient.RetireWorkerReturns(errors.New("nope"))
			})

			It("still waits for the sub-process to exit", func() {
				exit := errors.New("exiting")
				subExit <- exit
				Expect(<-process.Wait()).To(Equal(exit))
			})
		})

		Context("when syscall.SIGTERM is received", func() {
			JustBeforeEach(func() {
				process.Signal(syscall.SIGTERM)
			})

			It("deletes the worker", func() {
				Eventually(fakeBeaconClient.DeleteWorkerCallCount).Should(Equal(1))
			})

			It("forwards the signal", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGTERM))
			})

			Context("when deleting the worker fails", func() {
				BeforeEach(func() {
					fakeBeaconClient.DeleteWorkerReturns(errors.New("nope"))
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

		Context("when syscall.SIGINT is received", func() {
			JustBeforeEach(func() {
				process.Signal(syscall.SIGINT)
			})

			It("deletes the worker", func() {
				Eventually(fakeBeaconClient.DeleteWorkerCallCount).Should(Equal(1))
			})

			It("forwards the signal", func() {
				Expect(<-subSignals).To(Equal(syscall.SIGINT))
			})

			Context("when deleting the worker fails", func() {
				BeforeEach(func() {
					fakeBeaconClient.DeleteWorkerReturns(errors.New("nope"))
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

		It("forward the signal without landing, retiring, or deleting the worker", func() {
			Expect(<-subSignals).To(Equal(syscall.SIGTERM))
			Expect(fakeBeaconClient.LandWorkerCallCount()).Should(Equal(0))
			Expect(fakeBeaconClient.RetireWorkerCallCount()).Should(Equal(0))
			Expect(fakeBeaconClient.DeleteWorkerCallCount()).Should(Equal(0))
		})
	})

	Context("when syscall.SIGINT is received", func() {
		JustBeforeEach(func() {
			process.Signal(syscall.SIGINT)
		})

		It("forward the signal without landing, retiring, or deleting the worker", func() {
			Expect(<-subSignals).To(Equal(syscall.SIGINT))
			Expect(fakeBeaconClient.LandWorkerCallCount()).Should(Equal(0))
			Expect(fakeBeaconClient.RetireWorkerCallCount()).Should(Equal(0))
			Expect(fakeBeaconClient.DeleteWorkerCallCount()).Should(Equal(0))
		})
	})
})
