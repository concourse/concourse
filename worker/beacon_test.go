package worker_test

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/tsa"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/workerfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Beacon", func() {
	var (
		beacon     *worker.Beacon
		fakeClient *workerfakes.FakeTSAClient

		drainSignals chan<- os.Signal

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeClient = new(workerfakes.FakeTSAClient)

		logger := lager.NewLogger("test")
		logger.RegisterSink(lager.NewPrettySink(GinkgoWriter, lager.DEBUG))

		ds := make(chan os.Signal, 1)
		drainSignals = ds

		beacon = &worker.Beacon{
			Logger: logger,

			Client: fakeClient,

			DrainSignals: ds,

			LocalGardenNetwork: "some-garden-network",
			LocalGardenAddr:    "some-garden-addr",

			LocalBaggageclaimNetwork: "some-baggageclaim-network",
			LocalBaggageclaimAddr:    "some-baggageclaim-addr",
		}
	})

	JustBeforeEach(func() {
		process = ifrit.Background(beacon)
	})

	It("registers with the configured garden/baggageclaim info", func() {
		Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
		_, opts := fakeClient.RegisterArgsForCall(0)
		Expect(opts.LocalGardenNetwork).To(Equal(beacon.LocalGardenNetwork))
		Expect(opts.LocalGardenAddr).To(Equal(beacon.LocalGardenAddr))
		Expect(opts.LocalBaggageclaimNetwork).To(Equal(beacon.LocalBaggageclaimNetwork))
		Expect(opts.LocalBaggageclaimAddr).To(Equal(beacon.LocalBaggageclaimAddr))
	})

	Context("during registration", func() {
		BeforeEach(func() {
			fakeClient.RegisterStub = func(ctx context.Context, opts tsa.RegisterOptions) error {
				opts.RegisteredFunc()

				<-ctx.Done()

				return nil
			}
		})

		Describe("sending a signal", func() {
			It("cancels the registration context", func() {
				Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
				ctx, _ := fakeClient.RegisterArgsForCall(0)

				Consistently(ctx.Done()).ShouldNot(BeClosed())

				process.Signal(os.Interrupt)

				Eventually(ctx.Done()).Should(BeClosed())
			})
		})

		Describe("Drained", func() {
			It("returns false", func() {
				Expect(beacon.Drained()).Should(BeFalse())
			})
		})

		Context("when syscall.SIGUSR1 is received", func() {
			JustBeforeEach(func() {
				drainSignals <- syscall.SIGUSR1
			})

			It("lands the worker without exiting", func() {
				Eventually(fakeClient.LandCallCount).Should(Equal(1))
				Consistently(process.Wait()).ShouldNot(Receive())
			})

			Describe("Drained", func() {
				It("returns true", func() {
					Eventually(beacon.Drained).Should(BeTrue())
				})
			})

			Context("when landing the worker fails", func() {
				BeforeEach(func() {
					fakeClient.LandReturns(errors.New("nope"))
				})

				It("exits with the error", func() {
					Expect(<-process.Wait()).To(MatchError("nope"))
				})
			})

			Context("when syscall.SIGTERM is received after landing", func() {
				JustBeforeEach(func() {
					Eventually(fakeClient.LandCallCount).Should(Equal(1))
					process.Signal(syscall.SIGTERM)
				})

				It("exits without deleting the worker", func() {
					Expect(<-process.Wait()).ToNot(HaveOccurred())
					Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
				})

				Describe("Drained", func() {
					It("still returns true", func() {
						Consistently(beacon.Drained).Should(BeTrue())
					})
				})
			})

			Context("when syscall.SIGINT is received after landing", func() {
				JustBeforeEach(func() {
					Eventually(fakeClient.LandCallCount).Should(Equal(1))
					process.Signal(syscall.SIGINT)
				})

				It("exits without deleting the worker", func() {
					Expect(<-process.Wait()).ToNot(HaveOccurred())
					Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
				})

				Describe("Drained", func() {
					It("still returns true", func() {
						Consistently(beacon.Drained).Should(BeTrue())
					})
				})
			})
		})

		Context("when syscall.SIGUSR2 is received", func() {
			JustBeforeEach(func() {
				drainSignals <- syscall.SIGUSR2
			})

			It("retires the worker without exiting", func() {
				Eventually(fakeClient.RetireCallCount).Should(Equal(1))
				Consistently(process.Wait()).ShouldNot(Receive())
			})

			Describe("Drained", func() {
				It("returns true", func() {
					Eventually(beacon.Drained).Should(BeTrue())
				})
			})

			Context("when retiring the worker fails", func() {
				BeforeEach(func() {
					fakeClient.RetireReturns(errors.New("nope"))
				})

				It("exits with the error", func() {
					Expect(<-process.Wait()).To(MatchError("nope"))
				})
			})

			Context("when syscall.SIGTERM is received after retiring", func() {
				JustBeforeEach(func() {
					Eventually(fakeClient.RetireCallCount).Should(Equal(1))
					process.Signal(syscall.SIGTERM)
				})

				It("deletes the worker and exits", func() {
					Eventually(fakeClient.DeleteCallCount).Should(Equal(1))
					Expect(<-process.Wait()).ToNot(HaveOccurred())
				})

				Describe("Drained", func() {
					It("still returns true", func() {
						Consistently(beacon.Drained).Should(BeTrue())
					})
				})

				Context("when deleting the worker fails", func() {
					BeforeEach(func() {
						fakeClient.DeleteReturns(errors.New("nope"))
					})

					It("exits with the error", func() {
						Expect(<-process.Wait()).To(MatchError("nope"))
					})
				})
			})

			Context("when syscall.SIGINT is received after retiring", func() {
				JustBeforeEach(func() {
					Eventually(fakeClient.RetireCallCount).Should(Equal(1))
					process.Signal(syscall.SIGINT)
				})

				It("deletes the worker and exits", func() {
					Eventually(fakeClient.DeleteCallCount).Should(Equal(1))
					Expect(<-process.Wait()).ToNot(HaveOccurred())
				})

				Describe("Drained", func() {
					It("still returns true", func() {
						Consistently(beacon.Drained).Should(BeTrue())
					})
				})

				Context("when deleting the worker fails", func() {
					BeforeEach(func() {
						fakeClient.DeleteReturns(errors.New("nope"))
					})

					It("exits with the error", func() {
						Expect(<-process.Wait()).To(MatchError("nope"))
					})
				})
			})
		})

		Context("when syscall.SIGTERM is received", func() {
			JustBeforeEach(func() {
				process.Signal(syscall.SIGTERM)
			})

			It("exits without landing, retiring, or deleting the worker", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())
				Expect(fakeClient.LandCallCount()).Should(Equal(0))
				Expect(fakeClient.RetireCallCount()).Should(Equal(0))
				Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
			})

			Describe("Drained", func() {
				It("returns false", func() {
					Consistently(beacon.Drained).Should(BeFalse())
				})
			})
		})

		Context("when syscall.SIGINT is received", func() {
			JustBeforeEach(func() {
				process.Signal(syscall.SIGINT)
			})

			It("exits without landing, retiring, or deleting the worker", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())
				Expect(fakeClient.LandCallCount()).Should(Equal(0))
				Expect(fakeClient.RetireCallCount()).Should(Equal(0))
				Expect(fakeClient.DeleteCallCount()).Should(Equal(0))
			})

			Describe("Drained", func() {
				It("returns false", func() {
					Consistently(beacon.Drained).Should(BeFalse())
				})
			})
		})
	})

	Context("when a connection drain timeout is configured", func() {
		BeforeEach(func() {
			beacon.ConnectionDrainTimeout = time.Hour
		})

		It("configures it in the register options", func() {
			Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
			_, opts := fakeClient.RegisterArgsForCall(0)
			Expect(opts.ConnectionDrainTimeout).To(Equal(time.Hour))
		})
	})

	Context("when rebalancing is configured", func() {
		BeforeEach(func() {
			beacon.RebalanceInterval = 500 * time.Millisecond

			fakeClient.RegisterStub = func(ctx context.Context, opts tsa.RegisterOptions) error {
				opts.RegisteredFunc()
				<-ctx.Done()
				return nil
			}
		})

		It("continuously registers on the configured interval", func() {
			fuzz := beacon.RebalanceInterval / 2

			before := time.Now()
			Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
			Expect(time.Since(before)).To(BeNumerically("~", 0, fuzz))

			before = time.Now()
			Eventually(fakeClient.RegisterCallCount).Should(Equal(2))
			Expect(time.Since(before)).To(BeNumerically("~", beacon.RebalanceInterval, fuzz))

			before = time.Now()
			Eventually(fakeClient.RegisterCallCount).Should(Equal(3))
			Expect(time.Since(before)).To(BeNumerically("~", beacon.RebalanceInterval, fuzz))
		})

		It("cancels the prior registration when the rebalanced registration registers", func() {
			Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
			Consistently(process.Ready()).ShouldNot(Receive())
			ctx1, opts1 := fakeClient.RegisterArgsForCall(0)
			opts1.RegisteredFunc()
			Eventually(process.Ready()).Should(BeClosed())

			Eventually(fakeClient.RegisterCallCount).Should(Equal(2))
			Consistently(ctx1.Done()).ShouldNot(Receive())
			ctx2, opts2 := fakeClient.RegisterArgsForCall(1)
			opts2.RegisteredFunc()
			Eventually(ctx1.Done()).Should(BeClosed())

			Eventually(fakeClient.RegisterCallCount).Should(Equal(3))
			Consistently(ctx2.Done()).ShouldNot(Receive())
			ctx3, opts3 := fakeClient.RegisterArgsForCall(2)
			opts3.RegisteredFunc()
			Eventually(ctx2.Done()).Should(BeClosed())

			Consistently(ctx3.Done()).ShouldNot(Receive())
		})

		Context("when the maximum number of registrations is reached", func() {
			var someoneExit chan struct{}

			BeforeEach(func() {
				someoneExit = make(chan struct{})

				fakeClient.RegisterStub = func(ctx context.Context, opts tsa.RegisterOptions) error {
					opts.RegisteredFunc()
					<-someoneExit
					return nil
				}
			})

			It("stops rebalancing until one of the registrations exits", func() {
				Eventually(fakeClient.RegisterCallCount).Should(Equal(5))
				Consistently(fakeClient.RegisterCallCount, 2*beacon.RebalanceInterval).Should(Equal(5))

				someoneExit <- struct{}{}
				someoneExit <- struct{}{}

				Eventually(fakeClient.RegisterCallCount).Should(Equal(7))
				Consistently(fakeClient.RegisterCallCount, 2*beacon.RebalanceInterval).Should(Equal(7))
			})
		})

		Context("when the rebalanced registration exits", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				callTracker := make(chan struct{}, 1)

				fakeClient.RegisterStub = func(ctx context.Context, opts tsa.RegisterOptions) error {
					select {
					case callTracker <- struct{}{}:
						// first call; wait for normal exit
						opts.RegisteredFunc()
						<-ctx.Done()
						return nil
					default:
						// second call; simulate error
						return disaster
					}
				}
			})

			It("returns its error", func() {
				Expect(<-process.Wait()).To(Equal(disaster))
			})
		})
	})

	Context("when registering exits", func() {
		BeforeEach(func() {
			fakeClient.RegisterReturns(nil)
		})

		It("exits successfully", func() {
			Expect(<-process.Wait()).ToNot(HaveOccurred())
		})

		Context("with an error", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeClient.RegisterReturns(disaster)
			})

			It("exits with the same error", func() {
				Expect(<-process.Wait()).To(Equal(disaster))
			})
		})
	})

	Context("when registration succeeds", func() {
		It("becomes ready", func() {
			Consistently(process.Ready()).ShouldNot(Receive())

			Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
			_, opts := fakeClient.RegisterArgsForCall(0)
			opts.RegisteredFunc()

			Eventually(process.Ready()).Should(BeClosed())
		})
	})
})
