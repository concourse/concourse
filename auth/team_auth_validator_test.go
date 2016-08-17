package auth_test

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamAuthValidator", func() {
	var (
		validator    auth.Validator
		team         db.SavedTeam
		teamDB       *dbfakes.FakeTeamDB
		jwtValidator *authfakes.FakeValidator

		request           *http.Request
		isAuthenticated   bool
		username          string
		password          string
		encryptedPassword []byte
	)

	BeforeEach(func() {
		team = db.SavedTeam{
			Team: db.Team{
				Name: atc.DefaultTeamName,
			},
		}

		username = "username"
		password = "password"

		var err error
		encryptedPassword, err = bcrypt.GenerateFromPassword([]byte(password), 4)
		Expect(err).ToNot(HaveOccurred())

		jwtValidator = new(authfakes.FakeValidator)
		teamDBFactory := new(dbfakes.FakeTeamDBFactory)
		teamDB = new(dbfakes.FakeTeamDB)
		teamDBFactory.GetTeamDBReturns(teamDB)

		validator = auth.NewTeamAuthValidator(teamDBFactory, jwtValidator)

		request, err = http.NewRequest("GET", "http://example.com", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		isAuthenticated = validator.IsAuthenticated(request)
	})

	Context("when the team can be found", func() {
		BeforeEach(func() {
			teamDB.GetTeamReturns(team, true, nil)
		})

		Context("when team has no auth configured", func() {
			It("returns true", func() {
				Expect(isAuthenticated).To(BeTrue())
			})
		})

		Context("when team has basic auth configured", func() {
			BeforeEach(func() {
				team.BasicAuth = &db.BasicAuth{
					BasicAuthUsername: username,
					BasicAuthPassword: string(encryptedPassword),
				}
				teamDB.GetTeamReturns(team, true, nil)
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

		Context("when team has uaa auth configured", func() {
			BeforeEach(func() {
				team.UAAAuth = &db.UAAAuth{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				}
				teamDB.GetTeamReturns(team, true, nil)
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

		Context("when team has github auth configured", func() {
			BeforeEach(func() {
				team.GitHubAuth = &db.GitHubAuth{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				}
				teamDB.GetTeamReturns(team, true, nil)
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

		Context("when team has oauth and basic auth configured", func() {
			BeforeEach(func() {
				team.GitHubAuth = &db.GitHubAuth{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				}
				team.BasicAuth = &db.BasicAuth{
					BasicAuthUsername: username,
					BasicAuthPassword: string(encryptedPassword),
				}
				teamDB.GetTeamReturns(team, true, nil)
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
			teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
		})

		It("returns false", func() {
			Expect(isAuthenticated).To(BeFalse())
		})
	})
})
