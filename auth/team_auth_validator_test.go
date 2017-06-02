package auth_test

import (
	"encoding/json"
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/provider/providerfakes"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamAuthValidator", func() {
	var (
		validator        auth.Validator
		fakeTeam         *dbfakes.FakeTeam
		fakeTeamFactory  *dbfakes.FakeTeamFactory
		fakeTeamProvider *providerfakes.FakeTeamProvider
		jwtValidator     *authfakes.FakeValidator

		authProvider      map[string]*json.RawMessage
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

		jwtValidator = new(authfakes.FakeValidator)
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeamProvider = new(providerfakes.FakeTeamProvider)
		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeam.NameReturns(atc.DefaultTeamName)

		validator = auth.NewTeamAuthValidator(fakeTeamFactory, jwtValidator)

		request, err = http.NewRequest("GET", "http://example.com", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		isAuthenticated = validator.IsAuthenticated(request)
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

		Context("when team has provider auth configured", func() {
			BeforeEach(func() {
				provider.Register("fake-provider", fakeTeamProvider)
				data := []byte(`
				{
					"ClientID": "mcdonalds",
					"ClientSecret": "discounts"
				}`)
				authProvider = map[string]*json.RawMessage{
					"fake-provider": (*json.RawMessage)(&data),
				}
				fakeTeam.AuthReturns(authProvider)
				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("delegates to jwtValidator", func() {
				Expect(jwtValidator.IsAuthenticatedCallCount()).To(Equal(1))
				Expect(jwtValidator.IsAuthenticatedArgsForCall(0)).To(Equal(request))
			})

			Context("when jwtValidator returns false", func() {
				BeforeEach(func() {
					jwtValidator.IsAuthenticatedReturns(false)
				})

				It("returns false", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})

			Context("when jwtValidator returns true", func() {
				BeforeEach(func() {
					jwtValidator.IsAuthenticatedReturns(true)
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})
			})
		})

		Context("when team has provider auth and basic auth configured", func() {
			BeforeEach(func() {
				data := []byte(`
				{
					"ClientID": "mcdonalds",
					"ClientSecret": "discounts"
				}`)
				authProvider = map[string]*json.RawMessage{
					"fake-provider": (*json.RawMessage)(&data),
				}
				fakeTeam.AuthReturns(authProvider)
				fakeTeam.BasicAuthReturns(&atc.BasicAuth{
					BasicAuthUsername: username,
					BasicAuthPassword: string(encryptedPassword),
				})

				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			Context("when basic auth fails and oauth succeeds", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username+":bogus"))
					jwtValidator.IsAuthenticatedReturns(true)
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})
			})

			Context("when basic auth succeeds and oauth fails", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username+":"+password))
					jwtValidator.IsAuthenticatedReturns(false)
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})
			})

			Context("when basic auth fails and oauth fails", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "Basic "+b64(username))
					jwtValidator.IsAuthenticatedReturns(false)
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeFalse())
				})
			})
		})
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
})
