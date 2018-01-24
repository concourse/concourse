package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"golang.org/x/crypto/bcrypt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/basicauth"
	"github.com/concourse/skymarshal/noauth"
)

var _ = Describe("BasicAuthValidator", func() {
	var (
		logger          lager.Logger
		validator       auth.Validator
		fakeTeam        *dbfakes.FakeTeam
		fakeTeamFactory *dbfakes.FakeTeamFactory
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("auth")
		logger.RegisterSink(lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG))

		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeam.IDReturns(0)
		fakeTeam.NameReturns("main")

		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)

		validator = auth.NewBasicAuthValidator(logger, fakeTeamFactory)
	})

	Context("No Auth", func() {

		BeforeEach(func() {
			config, _ := json.Marshal(noauth.NoAuthConfig{true})

			fakeTeam.AuthReturns(map[string]*json.RawMessage{
				"noauth": (*json.RawMessage)(&config),
			})
		})

		Describe("IsAuthenticated", func() {
			var (
				request *http.Request

				isAuthenticated bool
			)

			BeforeEach(func() {
				var err error
				request, err = http.NewRequest("GET", "http://example.com", nil)
				Expect(err).ToNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				isAuthenticated = validator.IsAuthenticated(request)
			})

			Context("when its configured", func() {
				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})
			})
		})
	})

	Context("Basic Auth", func() {
		username := "username"
		password := "password"

		BeforeEach(func() {
			encrypted, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
			config, _ := json.Marshal(basicauth.BasicAuthConfig{username, string(encrypted)})

			fakeTeam.AuthReturns(map[string]*json.RawMessage{
				"basicauth": (*json.RawMessage)(&config),
			})
		})

		Describe("IsAuthenticated", func() {
			var (
				request *http.Request

				isAuthenticated bool
			)

			BeforeEach(func() {
				var err error
				request, err = http.NewRequest("GET", "http://example.com", nil)
				Expect(err).ToNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				isAuthenticated = validator.IsAuthenticated(request)
			})

			Context("when the team lookup fails", func() {
				BeforeEach(func() {
					fakeTeamFactory.FindTeamReturns(nil, true, errors.New("some-error"))
				})

				It("returns false", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})

			Context("when the team is not found in the db", func() {
				BeforeEach(func() {
					fakeTeamFactory.FindTeamReturns(nil, false, nil)
				})

				It("returns false", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})

			Context("when the request's basic auth header has the correct credentials", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username+":"+password))
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})

				Context("with different casing", func() {
					BeforeEach(func() {
						request.Header.Set("Authorization", "bAsIc "+b64(username+":"+password))
					})

					It("returns true", func() {
						Expect(isAuthenticated).To(BeTrue())
					})
				})
			})

			Context("when the request's basic auth header has incorrect credentials", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username+":bogus-"+password))
				})

				It("returns false", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})

			Context("when the request's Authorization header isn't basic auth", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Bearer "+b64(username+":"+password))
				})

				It("returns false", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})
		})
	})
})

func b64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
