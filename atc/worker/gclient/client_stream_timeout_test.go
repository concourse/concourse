package gclient_test

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/atc/worker/transport/transportfakes"
	"github.com/concourse/retryhttp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("stream http client", func() {
	var (
		gServer    *ghttp.Server
		ctx        context.Context
		cancelFunc context.CancelFunc
	)

	BeforeEach(func() {
		gServer = ghttp.NewServer()
		ctx, cancelFunc = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancelFunc()
		gServer.Close()
	})

	Context("blocking forever", func() {
		BeforeEach(func() {
			gServer.Reset()
			gServer.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
				<-ctx.Done()
			})
		})

		It("fails once context expires", func() {
			fakeTransport := new(transportfakes.FakeTransportDB)
			fakeLogger := lager.NewLogger("test")
			hostname := new(string)
			*hostname = gServer.Addr()

			clientFactory := gclient.NewGardenClientFactory(
				fakeTransport,
				fakeLogger,
				"wont-talk-to-you",
				hostname,
				retryhttp.NewExponentialBackOffFactory(1*time.Second),
				1*time.Second,
			)

			client := clientFactory.NewClient()
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Client.Timeout"))
		})
	})
})
