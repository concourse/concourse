package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Auth Token", func() {
	Describe("AuthToken", func() {
		var expectedAuthToken atc.AuthToken

		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/auth/token"

			expectedAuthToken = atc.AuthToken{
				Type:  "Bearer",
				Value: "gobbeldigook",
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedAuthToken),
				),
			)
		})

		It("returns the user's auth token", func() {
			token, err := team.AuthToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(Equal(expectedAuthToken))
		})
	})
})
