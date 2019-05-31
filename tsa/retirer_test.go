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

var _ = Describe("Retirer", func() {
	var (
		retirer *tsa.Retirer

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
		fakeTokenGenerator.GenerateSystemTokenReturns("no", nil)
		fakeTokenGenerator.GenerateTeamTokenReturns("yo", nil)

		fakeATC = ghttp.NewServer()

		atcEndpoint := rata.NewRequestGenerator(fakeATC.URL(), atc.Routes)

		retirer = &tsa.Retirer{
			ATCEndpoint:    atcEndpoint,
			TokenGenerator: fakeTokenGenerator,
		}
	})

	AfterEach(func() {
		fakeATC.Close()
	})

	Context("when the worker request is for a team-owned worker", func() {
		BeforeEach(func() {
			worker.Team = "some-team"
		})

		It("generates a team-specific token", func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/retire"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
				ghttp.RespondWith(200, nil, nil),
			))

			err := retirer.Retire(ctx, worker)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})

	It("tells the ATC to retire the worker", func() {
		fakeATC.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/retire"),
			ghttp.VerifyHeaderKV("Authorization", "Bearer no"),
			ghttp.RespondWith(200, nil, nil),
		))

		err := retirer.Retire(ctx, worker)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
	})

	Context("when the ATC responds with a 403", func() {
		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/retire"),
				ghttp.RespondWith(403, nil, nil),
			))
		})

		It("errors", func() {
			err := retirer.Retire(ctx, worker)
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError(ContainSubstring("403")))
			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the ATC responds with a 404", func() {
		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/retire"),
				ghttp.RespondWith(404, nil, nil),
			))
		})

		It("exits successfully", func() {
			err := retirer.Retire(ctx, worker)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the ATC does not respond to retire the worker", func() {
		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/retire"),
				ghttp.RespondWith(500, nil, nil),
			))
		})

		It("errors", func() {
			err := retirer.Retire(ctx, worker)
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError(ContainSubstring("500")))
			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})
})
