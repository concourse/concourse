package uaa_test

import (
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/skymarshal/uaa"
	"github.com/concourse/skymarshal/verifier"
	"github.com/dgrijalva/jwt-go"

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
	var jwtToken *jwt.Token
	var nextPageCalled bool

	BeforeEach(func() {
		nextPageCalled = false
		cfAPIServer = ghttp.NewServer()

		verifier = NewSpaceVerifier(
			[]string{"myspace-guid-1", "myspace-guid-2"},
			cfAPIServer.URL(),
		)

		jwtToken = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"exp": time.Now().Add(time.Hour * 72).Unix(),
		})

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

	Context("when token does not contain 'user_id'", func() {
		It("user is not verified", func() {
			Expect(verified).To(BeFalse())
			Expect(verifyErr).To(HaveOccurred())
		})
	})

	Context("when token contains 'user_id'", func() {
		BeforeEach(func() {
			jwtToken = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"exp":     time.Now().Add(time.Hour * 72).Unix(),
				"user_id": "my-user-id",
			})

			accessToken, err := jwtToken.SigningString()
			Expect(err).NotTo(HaveOccurred())

			oauthToken := &oauth2.Token{
				AccessToken: accessToken,
			}
			c := &oauth2.Config{}
			httpClient = c.Client(oauth2.NoContext, oauthToken)
		})

		Context("when user is a space developer", func() {
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
				"next_url": "null",
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
						ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-1/developers", "results-per-page=100"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-2/developers", "results-per-page=100"),
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
						ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-1/developers", "results-per-page=100"),
						ghttp.RespondWith(http.StatusOK, spaceDevelopersResponse),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-2/developers", "results-per-page=100"),
						func(w http.ResponseWriter, req *http.Request) {
							nextPageCalled = true
							w.Write([]byte(spaceDevelopersResponse))
						},
					),
				)
			})

			It("returns false", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeFalse())
				Expect(nextPageCalled).To(BeTrue())
			})
		})

		Context("when CF API responds with error code", func() {
			BeforeEach(func() {
				cfAPIServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-1/developers", "results-per-page=100"),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/spaces/myspace-guid-2/developers", "results-per-page=100"),
						func(w http.ResponseWriter, req *http.Request) {
							nextPageCalled = true
						},
					),
				)
			})

			It("does not return error", func() {
				Expect(verifyErr).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				Expect(verified).To(BeFalse())
			})

			It("makes requests to other spaces in the verifier", func() {
				Expect(nextPageCalled).To(BeTrue())
			})
		})
	})
})
