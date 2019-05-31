package tsa_test

import (
	"context"

	"github.com/concourse/concourse/v5/tsa"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/tsa/tsafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Deleter", func() {
	var (
		deleter *tsa.Deleter

		ctx                context.Context
		worker             atc.Worker
		fakeTokenGenerator *tsafakes.FakeTokenGenerator
		fakeATC            *ghttp.Server
	)

	BeforeEach(func() {
		ctx = lagerctx.NewContext(context.Background(), lagertest.NewTestLogger("test"))

		worker = atc.Worker{
			Name: "some-worker",
		}
		fakeTokenGenerator = new(tsafakes.FakeTokenGenerator)
		fakeTokenGenerator.GenerateSystemTokenReturns("yo", nil)

		fakeATC = ghttp.NewServer()

		atcEndpoint := rata.NewRequestGenerator(fakeATC.URL(), atc.Routes)

		deleter = &tsa.Deleter{
			ATCEndpoint:    atcEndpoint,
			TokenGenerator: fakeTokenGenerator,
		}
	})

	AfterEach(func() {
		fakeATC.Close()
	})

	It("tells the ATC to retire the worker", func() {
		fakeATC.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
			ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
			ghttp.RespondWith(200, nil, nil),
		))

		err := deleter.Delete(ctx, worker)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
	})

	Context("when the ATC does not respond to retire the worker", func() {
		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("DELETE", "/api/v1/workers/some-worker"),
				ghttp.RespondWith(500, nil, nil),
			))
		})

		It("errors", func() {
			err := deleter.Delete(ctx, worker)
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError(ContainSubstring("500")))
			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})
})
