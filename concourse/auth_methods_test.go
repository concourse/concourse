package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Auth Methods", func() {
	Describe("ListAuthMethods", func() {
		var expectedAuthMethods []atc.AuthMethod

		BeforeEach(func() {
			expectedAuthMethods = []atc.AuthMethod{
				{
					Type:        atc.AuthTypeBasic,
					DisplayName: "Basic",
					AuthURL:     "/teams/some-team/login/basic",
				},
				{
					Type:        atc.AuthTypeOAuth,
					DisplayName: "GitHub",
					AuthURL:     "/auth/github",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedAuthMethods),
				),
			)
		})

		It("returns all the auth methods", func() {
			pipelines, err := team.ListAuthMethods()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelines).To(Equal(expectedAuthMethods))
		})
	})
})
