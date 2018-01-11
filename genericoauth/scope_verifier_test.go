package genericoauth_test

import (
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/skymarshal/genericoauth"
	"github.com/concourse/skymarshal/verifier"
	"github.com/dgrijalva/jwt-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ScopeVerifier", func() {
	var verifier verifier.Verifier
	var httpClient *http.Client
	var verified bool
	var verifyErr error
	var jwtToken *jwt.Token

	BeforeEach(func() {

		verifier = NewScopeVerifier(
			"mainteam",
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

	Context("when token does not contain 'scope'", func() {
		It("user is not verified", func() {
			Expect(verified).To(BeFalse())
			Expect(verifyErr).To(HaveOccurred())
		})
	})

	Context("when user has proper scope", func() {
		BeforeEach(func() {
			jwtToken = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"exp":   time.Now().Add(time.Hour * 72).Unix(),
				"scope": []string{"read", "write", "mainteam"},
			})

			accessToken, err := jwtToken.SigningString()
			Expect(err).NotTo(HaveOccurred())

			oauthToken := &oauth2.Token{
				AccessToken: accessToken,
			}
			c := &oauth2.Config{}
			httpClient = c.Client(oauth2.NoContext, oauthToken)
		})

		It("returns true", func() {
			Expect(verifyErr).NotTo(HaveOccurred())
			Expect(verified).To(BeTrue())
		})

	})

	Context("and user does not have proper scope", func() {
		BeforeEach(func() {
			jwtToken = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"exp":   time.Now().Add(time.Hour * 72).Unix(),
				"scope": []string{"read"},
			})

			accessToken, err := jwtToken.SigningString()
			Expect(err).NotTo(HaveOccurred())

			oauthToken := &oauth2.Token{
				AccessToken: accessToken,
			}
			c := &oauth2.Config{}
			httpClient = c.Client(oauth2.NoContext, oauthToken)
		})

		It("returns false", func() {
			Expect(verifyErr).NotTo(HaveOccurred())
			Expect(verified).To(BeFalse())
		})
	})
})
