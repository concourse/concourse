package internal_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/go-concourse/concourse/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("HTTPAgent Client", func() {
	var (
		atcServer *ghttp.Server

		agent HTTPAgent

		tracing bool
	)

	BeforeEach(func() {
		atcServer = ghttp.NewServer()

		agent = NewHTTPAgent(atcServer.URL(), nil, tracing)
	})

	Describe("#Send", func() {
		Describe("Different status codes", func() {
			Describe("403 response", func() {
				BeforeEach(func() {
					atcServer = ghttp.NewServer()

					agent = NewHTTPAgent(atcServer.URL(), nil, tracing)
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/foo"),
							ghttp.RespondWith(http.StatusForbidden, "problem"),
						),
					)
				})
				It("returns back 403", func() {
					resp, err := agent.Send(Request{
						RequestName: atc.DeletePipeline,
						Params: rata.Params{
							"pipeline_name": "foo",
							"team_name":     atc.DefaultTeamName,
						},
					})
					Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})
