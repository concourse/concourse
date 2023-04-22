package client_test

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/client"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("baggageclaim http client", func() {
	Context("when making a streamIn/streamOut api call ", func() {
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

		Context("when context is canceled", func() {

			BeforeEach(func() {
				gServer.Reset()
				gServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-volume",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-volume"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							"some-volume",
							"/some/path",
							map[string]string{},
							false,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-volume"),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-volume/stream-out"),
						func(writer http.ResponseWriter, r *http.Request) {
							<-ctx.Done()
						},
					),
				)
			})

			It("should stop streaming and end the request with an error", func() {

				c := client.New(gServer.URL(), http.DefaultTransport)

				volume, err := c.CreateVolume(context.Background(), "some-volume", baggageclaim.VolumeSpec{Properties: map[string]string{}, Privileged: false})
				Expect(err).ToNot(HaveOccurred())

				var wg sync.WaitGroup
				requestCtx, cancelStream := context.WithCancel(context.Background())

				wg.Add(1)
				go func() {
					_, err = volume.StreamOut(requestCtx, ".", "gzip")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("context canceled"))

					wg.Done()
				}()

				time.AfterFunc(100*time.Millisecond, cancelStream)

				wg.Wait()
			})
		})
	})
})
