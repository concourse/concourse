package healthcheck_test

import (
	"context"
	"net/http"
	"time"

	"github.com/concourse/concourse/worker/healthcheck"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("baggageclaim", func() {
	var (
		bcServer *ghttp.Server
		bc       *healthcheck.Baggageclaim
		err      error
		volume   *healthcheck.Volume
	)

	BeforeEach(func() {
		bcServer = ghttp.NewServer()
		bc = &healthcheck.Baggageclaim{
			Url: "http://" + bcServer.Addr(),
		}
	})

	Context("Create", func() {
		var statusCode = 200
		const ttl = 3 * time.Second

		JustBeforeEach(func() {
			ctx, _ := context.WithDeadline(
				context.Background(), time.Now().Add(100*time.Millisecond))
			volume, err = bc.Create(ctx, "handle", ttl)
		})

		BeforeEach(func() {
			expectedVol := healthcheck.Volume{Handle: "handle", Path: "/rootfs", TTL: ttl}

			bcServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/volumes"),
				ghttp.VerifyJSON(`{"handle": "handle","strategy":{"type": "empty"},"ttl":3}`),
				ghttp.RespondWithJSONEncodedPtr(&statusCode, &expectedVol),
			))
		})

		It("issues volume creation request", func() {
			Expect(bcServer.ReceivedRequests()).To(HaveLen(1))
		})

		Context("having positive response", func() {
			It("doesn't fail", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns a proper volume", func() {
				Expect(volume.Handle).To(Equal("handle"))
				Expect(volume.Path).To(Equal("/rootfs"))
			})
		})

		Context("blocking forever", func() {
			BeforeEach(func() {
				bcServer.Reset()
				bcServer.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(5 * time.Second)
				})
			})

			It("fails once context expires", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("having negative response", func() {
			BeforeEach(func() {
				statusCode = 500
			})

			It("fails", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
