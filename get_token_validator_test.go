package auth_test

import (
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetTokenValidator", func() {
	var (
		validator         auth.Validator
		fakeTeam          *dbfakes.FakeTeam
		fakeTeamFactory   *dbfakes.FakeTeamFactory
		request           *http.Request
		isAuthenticated   bool
		username          string
		password          string
		encryptedPassword []byte
	)

	BeforeEach(func() {
		username = "username"
		password = "password"

		var err error
		encryptedPassword, err = bcrypt.GenerateFromPassword([]byte(password), 4)
		Expect(err).ToNot(HaveOccurred())
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeam.NameReturns(atc.DefaultTeamName)

		validator = auth.NewGetTokenValidator(fakeTeamFactory)
		request, err = http.NewRequest("GET", "http://example.com", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		isAuthenticated = validator.IsAuthenticated(request)
	})

	Context("when the team cannot be found", func() {
		BeforeEach(func() {
			fakeTeamFactory.FindTeamReturns(fakeTeam, false, nil)
		})

		It("returns false", func() {
			Expect(isAuthenticated).To(BeFalse())
		})
	})

	Context("when there is an error finding the team", func() {
		BeforeEach(func() {
			fakeTeamFactory.FindTeamReturns(fakeTeam, false, errors.New("cannot-find-any-mcdonalds-coupons"))
		})

		It("returns false", func() {
			Expect(isAuthenticated).To(BeFalse())
		})
	})

	Context("when the team can be found", func() {
		BeforeEach(func() {
			fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
		})

		Context("when team has no auth configured", func() {
			It("returns true", func() {
				Expect(isAuthenticated).To(BeTrue())
			})
		})

		Context("when team has basic auth configured", func() {
			BeforeEach(func() {
				fakeTeam.BasicAuthReturns(&atc.BasicAuth{
					BasicAuthUsername: username,
					BasicAuthPassword: string(encryptedPassword),
				})

				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			Context("when the request has correct credentials", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username+":"+password))
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})
			})

			Context("when the request has incorrect credentials", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username+":bogus"))
				})

				It("returns false", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})
		})
	})
})
