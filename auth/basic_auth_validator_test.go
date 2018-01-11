package auth_test

import (
	"encoding/base64"
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"golang.org/x/crypto/bcrypt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/auth"
)

var _ = Describe("BasicAuthValidator", func() {
	username := "username"
	password := "password"

	var (
		logger          lager.Logger
		validator       auth.Validator
		fakeTeam        *dbfakes.FakeTeam
		fakeTeamFactory *dbfakes.FakeTeamFactory
	)
	BeforeEach(func() {
		encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 4)
		Expect(err).ToNot(HaveOccurred())

		logger = lagertest.NewTestLogger("auth")
		logger.RegisterSink(lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG))

		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeam.NameReturns("main")
		fakeTeam.BasicAuthReturns(&atc.BasicAuth{username, string(encryptedPassword)})

		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)

		validator = auth.NewBasicAuthValidator(logger, fakeTeamFactory)
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

		Context("when the request's basic auth header has incorrect correct credentials", func() {
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

func b64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
