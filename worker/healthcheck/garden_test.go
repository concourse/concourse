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

var _ = Describe("garden", func() {
	var (
		gServer *ghttp.Server
		g       *healthcheck.Garden
		err     error
	)

	BeforeEach(func() {
		gServer = ghttp.NewServer()
		g = &healthcheck.Garden{
			Url: "http://" + gServer.Addr(),
		}
	})

	Context("Create", func() {
		var statusCode = 200

		JustBeforeEach(func() {
			ctx, _ := context.WithDeadline(
				context.Background(), time.Now().Add(100*time.Millisecond))
			err = g.Create(ctx, "handle", "/rootfs")
		})

		BeforeEach(func() {
			gServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/containers"),
				ghttp.VerifyJSON(`{"handle":"handle","rootfs":"raw:///rootfs"}`),
				ghttp.RespondWithJSONEncodedPtr(&statusCode, nil),
			))
		})

		It("issues container creation request", func() {
			Expect(gServer.ReceivedRequests()).To(HaveLen(1))
		})

		Context("blocking forever", func() {
			BeforeEach(func() {
				gServer.Reset()
				gServer.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(5 * time.Second)
				})
			})

			It("fails once context expires", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("having positive response", func() {
			It("doesn't fail", func() {
				Expect(err).ToNot(HaveOccurred())
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

	Context("Destroy", func() {
		var statusCode = 200

		JustBeforeEach(func() {
			ctx, _ := context.WithDeadline(
				context.Background(), time.Now().Add(100*time.Millisecond))
			err = g.Destroy(ctx, "handle")
		})

		BeforeEach(func() {
			gServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", MatchRegexp(`/containers/[a-z0-9-]+`)),
					ghttp.RespondWithJSONEncodedPtr(&statusCode, nil),
				))
		})

		It("issues volume deletion request", func() {
			Expect(gServer.ReceivedRequests()).To(HaveLen(1))
		})

		Context("blocking forever", func() {
			BeforeEach(func() {
				gServer.Reset()
				gServer.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(5 * time.Second)
				})
			})

			It("fails once context expires", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("having positive response", func() {
			It("doesn't fail", func() {
				Expect(err).ToNot(HaveOccurred())
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
