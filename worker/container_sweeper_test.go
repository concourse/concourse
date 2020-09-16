package worker_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Container Sweeper", func() {
	const (
		sweepInterval              = 1 * time.Second
		maxInFlight                = uint16(1)
		gardenClientTimeoutRequest = 5 * time.Millisecond
	)

	var (
		garden *ghttp.Server

		testLogger = lagertest.NewTestLogger("container-sweeper")

		fakeTSAClient workerfakes.FakeTSAClient
		gardenClient  gclient.Client

		sweeper *worker.ContainerSweeper

		osSignal chan os.Signal
		exited   chan struct{}
	)

	BeforeEach(func() {
		garden = ghttp.NewServer()

		osSignal = make(chan os.Signal)
		exited = make(chan struct{})

		gardenAddr := fmt.Sprintf("http://%s", garden.Addr())
		gardenClient = gclient.BasicGardenClientWithRequestTimeout(testLogger, gardenClientTimeoutRequest, gardenAddr)

		fakeTSAClient = workerfakes.FakeTSAClient{}
		fakeTSAClient.ReportContainersReturns(nil)

		sweeper = worker.NewContainerSweeper(testLogger, sweepInterval, &fakeTSAClient, gardenClient, maxInFlight)

	})

	JustBeforeEach(func() {
		go func() {
			_ = sweeper.Run(osSignal, make(chan struct{}))
			close(exited)
		}()
	})

	AfterEach(func() {
		close(osSignal)
		<-exited
		garden.Close()
	})

	Context("when garden doesn't respond on DELETE", func() {
		var (
			gardenContext context.Context
			gardenCancel  context.CancelFunc
		)

		BeforeEach(func() {
			gardenContext, gardenCancel = context.WithCancel(context.Background())
			garden.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers"),
					ghttp.RespondWithJSONEncoded(200, []map[string]string{}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", "/containers/some-handle-1"),
					func(w http.ResponseWriter, r *http.Request) {
						<-gardenContext.Done()
					},
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", "/containers/some-handle-2"),
					func(w http.ResponseWriter, r *http.Request) {
						<-gardenContext.Done()
					},
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers"),
					ghttp.RespondWithJSONEncoded(200, []map[string]string{}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", "/containers/some-handle-3"),
					func(w http.ResponseWriter, r *http.Request) {
						<-gardenContext.Done()
					},
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", "/containers/some-handle-4"),
					func(w http.ResponseWriter, r *http.Request) {
						<-gardenContext.Done()
					},
				),
			)
			// First GC Tick
			fakeTSAClient.ContainersToDestroyReturnsOnCall(0, []string{"some-handle-1", "some-handle-2"}, nil)
			// Second GC Tick
			fakeTSAClient.ContainersToDestroyReturnsOnCall(1, []string{"some-handle-3", "some-handle-4"}, nil)

			garden.AllowUnhandledRequests = true

		})
		AfterEach(func() {
			gardenCancel()
		})

		It("request to garden times out eventually", func() {
			Eventually(testLogger.Buffer()).Should(gbytes.Say("failed-to-destroy-container\".*\\(Client.Timeout exceeded while awaiting headers\\)"))
		})
		It("sweeper continues ticking and GC'ing", func() {
			// ensure all 4 DELETEs are issues over 2 successive ticks
			Eventually(func() []string {
				// Gather all containers deleted
				var deleteRequestPaths []string
				for _, req := range garden.ReceivedRequests() {
					if req.Method == http.MethodDelete {
						deleteRequestPaths = append(deleteRequestPaths, req.RequestURI)
					}
				}
				return deleteRequestPaths
			}).Should(ConsistOf(
				"/containers/some-handle-1",
				"/containers/some-handle-2",
				"/containers/some-handle-3",
				"/containers/some-handle-4"))

			// Check calls to TSA for containers to destroy > 1
			Expect(fakeTSAClient.ContainersToDestroyCallCount()).To(BeNumerically(">=", 2))
		})
	})

})
