package gardenruntime

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc/worker/gardenruntime/gclient/gclientfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Process.Wait", func() {
	var (
		fakeProcess   *gardenfakes.FakeProcess
		fakeContainer *gclientfakes.FakeContainer
		streamCtx     context.Context
		cancelStream  context.CancelFunc
		process       Process
		origGrace     time.Duration
	)

	BeforeEach(func() {
		origGrace = gardenProcessStopGracePeriod
		gardenProcessStopGracePeriod = 50 * time.Millisecond

		fakeProcess = new(gardenfakes.FakeProcess)
		fakeContainer = new(gclientfakes.FakeContainer)
		streamCtx, cancelStream = context.WithCancel(context.Background())

		process = Process{
			GardenContainer: fakeContainer,
			GardenProcess:   fakeProcess,
			cancelStream:    cancelStream,
		}
	})

	AfterEach(func() {
		gardenProcessStopGracePeriod = origGrace
		cancelStream()
	})

	Context("when the worker has disconnected and the process never exits on its own", func() {
		BeforeEach(func() {
			// Simulate a disconnected worker: the wait only unblocks once the
			// stream's context is cancelled, which in production closes the hung
			// connection.
			fakeProcess.WaitStub = func() (int, error) {
				<-streamCtx.Done()
				return 0, errors.New("stream closed")
			}
		})

		It("forcibly stops the container and cancels the stream so Wait returns", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // abort

			done := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				_, err := process.Wait(ctx)
				done <- err
			}()

			var err error
			Eventually(done, 5*time.Second).Should(Receive(&err))
			Expect(err).To(MatchError(context.Canceled))

			Expect(streamCtx.Err()).To(HaveOccurred(), "expected the stream context to be cancelled")
			Expect(fakeContainer.StopCallCount()).To(Equal(2), "expected SIGTERM then SIGKILL")
			Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse(), "first Stop should be graceful (kill=false)")
			Expect(fakeContainer.StopArgsForCall(1)).To(BeTrue(), "second Stop should be forceful (kill=true)")
		})
	})

	Context("when the process exits within the grace period after SIGTERM", func() {
		var release chan struct{}

		BeforeEach(func() {
			gardenProcessStopGracePeriod = time.Second

			release = make(chan struct{})
			fakeProcess.WaitStub = func() (int, error) {
				<-release
				return 0, nil
			}
			// Let the process "handle" the SIGTERM and exit promptly.
			fakeContainer.StopStub = func(kill bool) error {
				close(release)
				return nil
			}
		})

		It("does not force-kill the container", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // abort

			done := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				_, err := process.Wait(ctx)
				done <- err
			}()

			var err error
			Eventually(done, 5*time.Second).Should(Receive(&err))
			Expect(err).To(MatchError(context.Canceled))

			Expect(fakeContainer.StopCallCount()).To(Equal(1), "expected a single graceful Stop")
			Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse(), "Stop should be graceful (kill=false)")
		})
	})

	Context("when the process completes normally (no abort)", func() {
		BeforeEach(func() {
			fakeProcess.WaitReturns(42, nil)
		})

		It("records the exit status and releases the stream", func() {
			result, err := process.Wait(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(42))

			Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))
			name, value := fakeContainer.SetPropertyArgsForCall(0)
			Expect(name).To(Equal(exitStatusPropertyName))
			Expect(value).To(Equal("42"))

			Expect(streamCtx.Err()).To(HaveOccurred(), "expected the stream context to be released on completion")
			Expect(fakeContainer.StopCallCount()).To(Equal(0), "expected no Stop calls on normal completion")
		})
	})
})
