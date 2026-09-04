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
							Handle:     "some-volume",
							Path:       "/some/path",
							Properties: map[string]string{},
							Privileged: false,
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

				wg.Go(func() {
					_, err = volume.StreamOut(requestCtx, ".", "gzip")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("context canceled"))
				})

				time.AfterFunc(100*time.Millisecond, cancelStream)

				wg.Wait()
			})
		})
	})

	Context("when calling CleanupOrphanedVolumes", func() {
		var (
			gServer *ghttp.Server
		)

		BeforeEach(func() {
			gServer = ghttp.NewServer()
		})

		AfterEach(func() {
			gServer.Close()
		})

		It("returns nil on 204 response", func() {
			gServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/volumes/cleanup-orphans"),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
			)

			c := client.New(gServer.URL(), http.DefaultTransport)

			err := c.CleanupOrphanedVolumes(context.Background())
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error on 500 response", func() {
			gServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/volumes/cleanup-orphans"),
					ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, map[string]string{
						"error": "cleanup-failed",
					}),
				),
			)

			c := client.New(gServer.URL(), http.DefaultTransport)

			err := c.CleanupOrphanedVolumes(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cleanup-failed"))
		})
	})
})
