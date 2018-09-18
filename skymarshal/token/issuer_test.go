package token_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/skymarshal/token/tokenfakes"
)

var _ = Describe("Token Issuer", func() {
	Describe("Issue", func() {
		var (
			duration        time.Duration
			tokenIssuer     token.Issuer
			verifiedClaims  *token.VerifiedClaims
			fakeTeamFactory *dbfakes.FakeTeamFactory
			fakeGenerator   *tokenfakes.FakeGenerator
			fakeToken       *oauth2.Token
		)

		BeforeEach(func() {
			duration = time.Minute
			fakeToken = &oauth2.Token{}

			fakeGenerator = &tokenfakes.FakeGenerator{}
			fakeGenerator.GenerateReturns(fakeToken, nil)

			fakeTeamFactory = &dbfakes.FakeTeamFactory{}
			fakeTeamFactory.GetTeamsReturns([]db.Team{}, nil)

			tokenIssuer = token.NewIssuer(fakeTeamFactory, fakeGenerator, duration)

			verifiedClaims = &token.VerifiedClaims{
				Sub:         "some-sub",
				Email:       "mail@example.com",
				Name:        "Firstname Lastname",
				UserID:      "user-id",
				UserName:    "user-name",
				ConnectorID: "connector-id",
				Groups:      []string{"some-group"},
			}
		})

		Context("without a team factory", func() {
			BeforeEach(func() {
				tokenIssuer = token.NewIssuer(nil, fakeGenerator, duration)
			})

			It("errors", func() {
				_, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("without a token generator", func() {
			BeforeEach(func() {
				tokenIssuer = token.NewIssuer(fakeTeamFactory, nil, duration)
			})

			It("errors", func() {
				_, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the verified token doesn't contain a user id", func() {
			BeforeEach(func() {
				verifiedClaims = &token.VerifiedClaims{ConnectorID: "connector-id"}
			})

			It("errors", func() {
				_, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the verified token doesn't contain a connector id", func() {
			BeforeEach(func() {
				verifiedClaims = &token.VerifiedClaims{UserID: "user-id"}
			})

			It("errors", func() {
				_, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when team factory can't fetch teams", func() {
			BeforeEach(func() {
				fakeTeamFactory.GetTeamsReturns(nil, errors.New("error"))
			})

			It("errors", func() {
				_, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when team factory returns no teams", func() {
			BeforeEach(func() {
				fakeTeamFactory.GetTeamsReturns([]db.Team{}, nil)
			})

			JustBeforeEach(func() {
				skyToken, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).NotTo(HaveOccurred())
				Expect(skyToken).To(Equal(fakeToken))
			})

			It("calls generate with an empty teams claim", func() {
				claims := fakeGenerator.GenerateArgsForCall(0)
				Expect(claims["teams"]).To(HaveLen(0))
			})
		})

		Context("when the team factory returns teams", func() {
			var teams []db.Team
			var fakeTeam1 *dbfakes.FakeTeam
			var fakeTeam2 *dbfakes.FakeTeam

			AssertTokenClaims := func() {
				It("includes expected claims", func() {
					claims := fakeGenerator.GenerateArgsForCall(0)
					Expect(claims["sub"]).To(Equal("some-sub"))
					Expect(claims["email"]).To(Equal("mail@example.com"))
					Expect(claims["name"]).To(Equal("Firstname Lastname"))
					Expect(claims["user_id"]).To(Equal("user-id"))
					Expect(claims["user_name"]).To(Equal("user-name"))
					Expect(claims["exp"]).To(BeNumerically(">", time.Now().Unix()))
					Expect(claims["exp"]).To(BeNumerically("<=", time.Now().Add(duration).Unix()))
					Expect(claims["csrf"]).NotTo(BeEmpty())
				})
			}

			AssertTokenAdminClaims := func() {
				Context("when team is admin", func() {
					BeforeEach(func() {
						fakeTeam1.AdminReturns(true)
					})
					It("includes expected claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["is_admin"]).To(BeTrue())
					})
				})

				Context("when team is not admin", func() {
					BeforeEach(func() {
						fakeTeam1.AdminReturns(false)
					})
					It("includes expected claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["is_admin"]).To(BeFalse())
					})
				})
			}

			BeforeEach(func() {
				fakeTeam1 = &dbfakes.FakeTeam{}
				fakeTeam1.NameReturns("fake-team-1")
				fakeTeam1.AuthReturns(map[string][]string{
					"users": []string{"some-connector:some-user"},
				})

				fakeTeam2 = &dbfakes.FakeTeam{}
				fakeTeam2.NameReturns("fake-team-2")
				fakeTeam2.AuthReturns(map[string][]string{
					"groups": []string{"some-connector:some-exclusive-group"},
				})

				teams = []db.Team{fakeTeam1, fakeTeam2}
				fakeTeamFactory.GetTeamsReturns(teams, nil)
			})

			JustBeforeEach(func() {
				skyToken, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).NotTo(HaveOccurred())
				Expect(skyToken).To(Equal(fakeToken))
			})

			Context("when teams don't have auth configured", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(map[string][]string{})
					fakeTeam2.AuthReturns(map[string][]string{})
				})

				It("calls generate with expected team claims", func() {
					claims := fakeGenerator.GenerateArgsForCall(0)
					Expect(claims["teams"]).To(ContainElement("fake-team-1"))
					Expect(claims["teams"]).To(ContainElement("fake-team-2"))
				})

				AssertTokenClaims()
				AssertTokenAdminClaims()
			})

			Context("when the verified claims has no groups", func() {
				BeforeEach(func() {
					verifiedClaims.Groups = []string{}
				})

				Context("when no teams have auth configured for the user", func() {
					It("calls generate with an empty teams claim", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(HaveLen(0))
					})

					AssertTokenClaims()
				})

				Context("when a team has user auth configured for the userid", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(map[string][]string{
							"users": []string{"connector-id:user-id"},
						})
					})

					It("calls generate with expected team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(ContainElement("fake-team-1"))
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-2"))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})

				Context("when a team has user auth configured for the username", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(map[string][]string{
							"users": []string{"connector-id:user-name"},
						})
					})

					It("calls generate with expected team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(ContainElement("fake-team-1"))
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-2"))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})
			})

			Context("when the verified claims contain an org group", func() {
				BeforeEach(func() {
					verifiedClaims.Groups = []string{"org-1"}
				})

				Context("when a team has group auth configured for an org", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(map[string][]string{
							"groups": []string{"connector-id:org-1"},
						})
					})

					It("calls generate with expect team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(ContainElement("fake-team-1"))
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-2"))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})

				Context("when a team has group auth configured for an org:team", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(map[string][]string{
							"groups": []string{"connector-id:org-1:team-1"},
						})
					})

					It("calls generate with expect team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-1"))
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-2"))
					})

					AssertTokenClaims()
				})
			})

			Context("when the verified claims contain an org:team group", func() {
				BeforeEach(func() {
					verifiedClaims.Groups = []string{"org-1:team-1"}
				})

				Context("when a team has group auth configured for an org", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(map[string][]string{
							"groups": []string{"connector-id:org-1"},
						})
					})

					It("calls generate with expect team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(ContainElement("fake-team-1"))
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-2"))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})

				Context("when a team has group auth configured for an org:team", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(map[string][]string{
							"groups": []string{"connector-id:org-1:team-1"},
						})
					})

					It("calls generate with expect team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(ContainElement("fake-team-1"))
						Expect(claims["teams"]).NotTo(ContainElement("fake-team-2"))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})
			})
		})
	})
})
