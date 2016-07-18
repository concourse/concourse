package uaa_test

import (
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pivotal-golang/lager/lagertest"
	"golang.org/x/oauth2"

	. "github.com/concourse/atc/auth/uaa"
	"github.com/concourse/atc/auth/verifier"

	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SpaceVerifier", func() {
	var verifier verifier.Verifier
	var cfAPIServer *ghttp.Server
	var httpClient *http.Client
	var verified bool
	var verifyErr error

	BeforeEach(func() {
		cfAPIServer = ghttp.NewServer()

		verifier = NewSpaceVerifier(
			[]string{"myspace-guid-1", "myspace-guid-2"},
			cfAPIServer.URL(),
		)

		jwtToken := jwt.New(jwt.SigningMethodHS256)
		jwtToken.Claims["exp"] = time.Now().Add(time.Hour * 72).Unix()
		jwtToken.Claims["user-id"] = "my-user-id"

		accessToken, err := jwtToken.SigningString()
		Expect(err).NotTo(HaveOccurred())

		oauthToken := &oauth2.Token{
			AccessToken: accessToken,
		}
		c := &oauth2.Config{}
		httpClient = c.Client(oauth2.NoContext, oauthToken)
	})

	JustBeforeEach(func() {
		verified, verifyErr = verifier.Verify(lagertest.NewTestLogger("test"), httpClient)
	})

	Context("when user is a space developer", func() {
		var nextPageCalled bool
		BeforeEach(func() {
			firstPageResponse := `{
			"next_url": "/next-url",
			"resources": [
				{
					"metadata": {
						"guid": "other-user-id-1"
					}
				},
				{
					"metadata": {
						"guid": "another-user-id"
					}
				}
			]
			}`
			spaceDevelopersResponse := `{
			"next_url": null,
				"resources": [
					{
						"metadata": {
							"guid": "other-user-id-2"
						}
					},
					{
						"metadata": {
							"guid": "my-user-id"
						}
					}
				]
			}`
			cfAPIServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-1/developers?results-per-page=100"),
					ghttp.RespondWith(http.StatusOK, `{}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-2/developers?results-per-page=100"),
					ghttp.RespondWith(http.StatusOK, firstPageResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/next-url"),
					func(w http.ResponseWriter, req *http.Request) {
						nextPageCalled = true
						w.Write([]byte(spaceDevelopersResponse))
					},
				),
			)
		})

		It("returns true", func() {
			Expect(verifyErr).NotTo(HaveOccurred())
			Expect(verified).To(BeTrue())
		})

		It("follows next page", func() {
			Expect(nextPageCalled).To(BeTrue())
		})
	})

	Context("when user is not a space developer", func() {
		BeforeEach(func() {
			spaceDevelopersResponse := `{
				"resources": [
					{
						"metadata": {
							"guid": "unknown-user-id"
						}
					}
				]
			}`
			cfAPIServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-1/developers?results-per-page=100"),
					ghttp.RespondWith(http.StatusOK, spaceDevelopersResponse),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-2/developers?results-per-page=100"),
					ghttp.RespondWith(http.StatusOK, spaceDevelopersResponse),
				),
			)
		})

		It("returns false", func() {
			Expect(verifyErr).NotTo(HaveOccurred())
			Expect(verified).To(BeFalse())
		})
	})

	Context("when CF API responds with error", func() {
		BeforeEach(func() {
			cfAPIServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-1/developers?results-per-page=100"),
					ghttp.RespondWith(http.StatusUnauthorized, ""),
				),
			)
		})

		It("returns error", func() {
			Expect(verifyErr).To(HaveOccurred())
			Expect(verifyErr.Error()).To(ContainSubstring("unexpected response"))
		})
	})
})
