package token_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/skymarshal/token/tokenfakes"
	"golang.org/x/oauth2"
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
				verifiedClaims.UserID = ""
			})

			It("errors", func() {
				_, err := tokenIssuer.Issue(verifiedClaims)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the verified token doesn't contain a connector id", func() {
			BeforeEach(func() {
				verifiedClaims.ConnectorID = ""
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
						fakeTeam1.AuthReturns(atc.TeamAuth{"owner": {}})
					})
					It("includes expected claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["is_admin"]).To(BeTrue())
					})
				})

				Context("when team is not admin", func() {
					BeforeEach(func() {
						fakeTeam1.AdminReturns(false)
						fakeTeam1.AuthReturns(atc.TeamAuth{"owner": {}})
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
				fakeTeam1.AuthReturns(atc.TeamAuth{
					"owner": {"users": []string{"some-connector:some-user"}},
				})

				fakeTeam2 = &dbfakes.FakeTeam{}
				fakeTeam2.NameReturns("fake-team-2")
				fakeTeam2.AuthReturns(atc.TeamAuth{
					"owner": {"groups": []string{"some-connector:some-exclusive-group"}},
				})

				teams = []db.Team{fakeTeam1, fakeTeam2}
				fakeTeamFactory.GetTeamsReturns(teams, nil)
			})

			Context("when the verified claims don't match any db teams", func() {
				FIt("errors and does not return a sky token", func() {
					verifiedClaims.ConnectorID = "some-connector"
					skyToken, err := tokenIssuer.Issue(verifiedClaims)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("user doesn't belong to any team"))
					Expect(skyToken).To(BeNil())
				})
			})

			Context("when the verified claims match one or more db teams", func() {
				JustBeforeEach(func() {
					skyToken, err := tokenIssuer.Issue(verifiedClaims)
					Expect(err).NotTo(HaveOccurred())
					Expect(skyToken).To(Equal(fakeToken))
				})

				Context("when teams don't have roles configured", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(atc.TeamAuth{})
						fakeTeam2.AuthReturns(atc.TeamAuth{})
					})

					It("calls generate with expected team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(HaveLen(0))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})

				Context("when teams don't have auth configured", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(atc.TeamAuth{"owner": {}})
						fakeTeam2.AuthReturns(atc.TeamAuth{"owner": {}})
					})

					It("calls generate with expected team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ContainElement("owner")))
						Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-2", ContainElement("owner")))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})

				Context("when the verified claims has no groups", func() {
					BeforeEach(func() {
						verifiedClaims.Groups = []string{}
					})

					Context("when a team has user auth configured for the userid", func() {
						BeforeEach(func() {
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"users": []string{"connector-id:user-id"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ContainElement("owner")))
							Expect(claims["teams"]).NotTo(HaveKey("fake-team-2"))
						})

						AssertTokenClaims()
						AssertTokenAdminClaims()
					})

					Context("when a team has user auth configured for the username", func() {
						BeforeEach(func() {
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"users": []string{"connector-id:user-name"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ContainElement("owner")))
							Expect(claims["teams"]).NotTo(HaveKey("fake-team-2"))
						})

						AssertTokenClaims()
						AssertTokenAdminClaims()
					})
				})

				Context("when a team has different roles configured", func() {
					BeforeEach(func() {
						fakeTeam1.AuthReturns(atc.TeamAuth{
							"owner":  {"users": []string{"connector-id:user-id"}},
							"member": {"users": []string{"connector-id:user-id"}},
							"viewer": {"users": []string{"connector-id:user-id"}},
						})
					})

					It("calls generate with expected team claims", func() {
						claims := fakeGenerator.GenerateArgsForCall(0)
						Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ConsistOf("owner", "member", "viewer")))
					})

					AssertTokenClaims()
					AssertTokenAdminClaims()
				})

				Context("when the verified claims contain an org group", func() {
					BeforeEach(func() {
						verifiedClaims.Groups = []string{"org-1"}
					})

					Context("when a team has group auth configured for an org", func() {
						BeforeEach(func() {
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"groups": []string{"connector-id:org-1"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ContainElement("owner")))
							Expect(claims["teams"]).NotTo(HaveKey("fake-team-2"))
						})

						AssertTokenClaims()
						AssertTokenAdminClaims()
					})

					Context("when a team has group auth configured for an org:team", func() {
						BeforeEach(func() {
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"groups": []string{"connector-id:org-1:team-1"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveLen(0))
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
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"groups": []string{"connector-id:org-1"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ContainElement("owner")))
							Expect(claims["teams"]).NotTo(HaveKey("fake-team-2"))
						})

						AssertTokenClaims()
						AssertTokenAdminClaims()
					})

					Context("when a team has group auth configured for an org:team", func() {
						BeforeEach(func() {
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"groups": []string{"connector-id:org-1:team-1"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveKeyWithValue("fake-team-1", ContainElement("owner")))
							Expect(claims["teams"]).NotTo(HaveKey("fake-team-2"))
						})

						AssertTokenClaims()
						AssertTokenAdminClaims()
					})
				})

				Context("when the verified claims has no username", func() {
					BeforeEach(func() {
						verifiedClaims.UserName = ""
					})

					Context("when the team auth is configured with only the connector", func() {
						BeforeEach(func() {
							fakeTeam1.AuthReturns(atc.TeamAuth{
								"owner": {"users": []string{"connector-id:"}},
							})
							fakeTeam2.AuthReturns(atc.TeamAuth{
								"owner": {"users": []string{"connector-id:"}},
							})
						})

						It("calls generate with expected team claims", func() {
							claims := fakeGenerator.GenerateArgsForCall(0)
							Expect(claims["teams"]).To(HaveLen(0))
						})
					})
				})
			})
		})
	})
})
