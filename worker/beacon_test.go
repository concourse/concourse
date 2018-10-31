package worker_test

import (
	"context"
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/tsa"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Beacon", func() {
	var (
		beacon     *worker.Beacon
		fakeClient *workerfakes.FakeTSAClient

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeClient = new(workerfakes.FakeTSAClient)

		logger := lager.NewLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		beacon = &worker.Beacon{
			Logger: logger,

			Client: fakeClient,

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
		var finishRegister chan error

		BeforeEach(func() {
			finishRegister = make(chan error, 1)

			fakeClient.RegisterStub = func(ctx context.Context, opts tsa.RegisterOptions) error {
				select {
				case err := <-finishRegister:
					return err
				case <-ctx.Done():
					return nil
				}
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
	})

	Context("when rebalancing is configured", func() {
		BeforeEach(func() {
			beacon.RebalanceInterval = 500 * time.Millisecond

			fakeClient.RegisterStub = func(ctx context.Context, opts tsa.RegisterOptions) error {
				<-ctx.Done()
				return nil
			}
		})

		It("configures the interval as the drain timeout", func() {
			Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
			_, opts := fakeClient.RegisterArgsForCall(0)
			Expect(opts.DrainTimeout).To(Equal(beacon.RebalanceInterval))
		})

		It("continuously registers on the configured interval", func() {
			fuzz := beacon.RebalanceInterval / 2

			before := time.Now()
			Eventually(fakeClient.RegisterCallCount).Should(Equal(1))
			Expect(time.Now().Sub(before)).To(BeNumerically("~", 0, fuzz))

			before = time.Now()
			Eventually(fakeClient.RegisterCallCount).Should(Equal(2))
			Expect(time.Now().Sub(before)).To(BeNumerically("~", beacon.RebalanceInterval, fuzz))

			before = time.Now()
			Eventually(fakeClient.RegisterCallCount).Should(Equal(3))
			Expect(time.Now().Sub(before)).To(BeNumerically("~", beacon.RebalanceInterval, fuzz))
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
