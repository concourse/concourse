package tsa_test

import (
	"github.com/concourse/tsa"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/tsa/tsafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Lander", func() {
	var (
		lander *tsa.Lander

		logger             *lagertest.TestLogger
		worker             atc.Worker
		fakeTokenGenerator *tsafakes.FakeTokenGenerator
		fakeATC            *ghttp.Server
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		worker = atc.Worker{
			Name: "some-worker",
		}
		fakeTokenGenerator = new(tsafakes.FakeTokenGenerator)
		fakeTokenGenerator.GenerateTokenReturns("yo", nil)

		fakeATC = ghttp.NewServer()

		atcEndpoint := rata.NewRequestGenerator(fakeATC.URL(), atc.Routes)

		lander = &tsa.Lander{
			ATCEndpoint:    atcEndpoint,
			TokenGenerator: fakeTokenGenerator,
		}
	})

	AfterEach(func() {
		fakeATC.Close()
	})

	It("tells the ATC to land the worker", func() {
		fakeATC.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
			ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
			ghttp.RespondWith(200, nil, nil),
		))

		err := lander.Land(logger, worker)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
	})

	Context("when the ATC responds with a 404", func() {
		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
				ghttp.RespondWith(404, nil, nil),
			))
		})

		It("exits successfully", func() {
			err := lander.Land(logger, worker)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the ATC does not respond to land the worker", func() {
		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/workers/some-worker/land"),
				ghttp.RespondWith(500, nil, nil),
			))
		})

		It("errors", func() {
			err := lander.Land(logger, worker)
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError(ContainSubstring("500")))
			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})
	})
})
