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
			expectedURL := "/api/v1/auth/methods"

			expectedAuthMethods = []atc.AuthMethod{
				{
					Type:        atc.AuthTypeBasic,
					DisplayName: "Basic",
					AuthURL:     "/login/basic",
				},
				{
					Type:        atc.AuthTypeOAuth,
					DisplayName: "GitHub",
					AuthURL:     "/auth/github",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedAuthMethods),
				),
			)
		})

		It("returns all the auth methods", func() {
			pipelines, err := client.ListAuthMethods()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelines).To(Equal(expectedAuthMethods))
		})
	})
})
