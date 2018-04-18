package genericoauth_oidc_test

import (
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/skymarshal/genericoauth_oidc"
	"github.com/concourse/skymarshal/verifier"
	"github.com/dgrijalva/jwt-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GroupsVerifier", func() {

	var verifier verifier.Verifier
	var httpClient *http.Client
	var verified bool
	var verifyErr error

	generateHttpClient := func (jwtToken *jwt.Token) {
		idToken, err := jwtToken.SignedString([]byte("secrettoken"))
		Expect(err).NotTo(HaveOccurred())
	
		oauthToken := &oauth2.Token{
			AccessToken: "accessToken",
		}
		rawToken := make(map[string]interface{})
		rawToken["id_token"] = idToken
		oauthToken = oauthToken.WithExtra(rawToken)
	
		c := &oauth2.Config{}
		httpClient = c.Client(oauth2.NoContext, oauthToken)
	}

	generateJWTTokenWithGroup := func (sub string, group string, customGroupName string) {
		if customGroupName == "" {
			customGroupName = "groups"
		}
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": sub,
			customGroupName: group,
			"exp": time.Now().Add(time.Hour * 72).Unix(),
		})
	
		generateHttpClient(jwtToken)
	}

	generateJWTTokenWithGroups := func (sub string, groups []string, customGroupName string) {
		if customGroupName == "" {
			customGroupName = "groups"
		}
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": sub,
			customGroupName: groups,
			"exp": time.Now().Add(time.Hour * 72).Unix(),
		})
	
		generateHttpClient(jwtToken)
	}

	generateJWTTokenWithEmpty := func (sub string) {
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": sub,
			"exp": time.Now().Add(time.Hour * 72).Unix(),
		})
	
		generateHttpClient(jwtToken)
	}

	JustBeforeEach(func() {
		verified, verifyErr = verifier.Verify(lagertest.NewTestLogger("test"), httpClient)
	})

	Describe("validate oidc token", func() {

		Context("when token contains valid uid", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser", "seconduser"},
					[]string{"group3"},
					"",
				)
		
				generateJWTTokenWithGroups("seconduser", []string{"group1", "group2"}, "")
			})

			It("returns true", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeTrue())
			})
		})

		Context("when token contains valid group", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"testuser"},
					[]string{"group2"},
					"",
				)
		
				generateJWTTokenWithGroups("mainuser", []string{"group1", "group2"}, "")
			})

			It("returns true", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeTrue())
			})
		})

		Context("when token does not contain valid authz data", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"testuser"},
					[]string{"group3", "group4"},
					"",
				)
		
				generateJWTTokenWithGroups("mainuser", []string{"group1", "group2"}, "")
			})

			It("returns false", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeFalse())
			})
		})

		Context("when token does not contain id_token field", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser"},
					[]string{"group1", "group2"},
					"",
				)

				oauthToken := &oauth2.Token{
					AccessToken: "accessToken",
				}
			
				c := &oauth2.Config{}
				httpClient = c.Client(oauth2.NoContext, oauthToken)
			})

			It("fails validation", func() {
				Expect(verifyErr).To(HaveOccurred())
			})
		})

		Context("when token contains single correct value in the group", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser"},
					[]string{"group1", "group2"},
					"",
				)

				generateJWTTokenWithGroup("testuser", "group1", "")
			})

			It("returns true", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeTrue())
			})
		})

		Context("when token contains single incorrect value in the group", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser"},
					[]string{"group1", "group2"},
					"",
				)

				generateJWTTokenWithGroup("testuser", "group3", "")
			})

			It("returns false", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeFalse())
			})
		})

		Context("when token does not contain groups field", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser"},
					[]string{"group1", "group2"},
					"",
				)

				generateJWTTokenWithEmpty("testuser")
			})

			It("returns false", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeFalse())
			})
		})
	})

	Describe("validate oidc token with custom valid group name", func() {

		Context("when token contains valid uid", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser"},
					[]string{"group3", "group4"},
					"customgroupname",
				)
		
				generateJWTTokenWithGroups("mainuser", []string{"group1", "group2"}, "customgroupname")
			})

			It("returns true", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeTrue())
			})
		})

		Context("when token contains valid group", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"testuser"},
					[]string{"group2"},
					"customgroupname",
				)
		
				generateJWTTokenWithGroups("mainuser", []string{"group1", "group2"}, "customgroupname")
			})

			It("returns true", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeTrue())
			})
		})
	})

	Describe("validate oidc token with custom invalid group name", func() {
		Context("when token contains valid uid", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"mainuser"},
					[]string{"group3"},
					"customgroupname",
				)
		
				generateJWTTokenWithGroups("mainuser", []string{"group1", "group2"}, "")
			})

			It("returns true", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeTrue())
			})
		})

		Context("when token contains valid group", func() {
			BeforeEach(func() {
				verifier = NewGroupsVerifier(
					[]string{"testuser"},
					[]string{"group2"},
					"customgroupname",
				)
		
				generateJWTTokenWithGroups("mainuser", []string{"group1", "group2"}, "")
			})

			It("returns false", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(verified).To(BeFalse())
			})
		})
	})
})
