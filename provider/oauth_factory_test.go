package provider_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/uaa"
	"github.com/concourse/atc/db"

	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OAuthFactory", func() {
	var fakeTeamDB *dbfakes.FakeTeamDB
	var oauthFactory provider.OAuthFactory

	BeforeEach(func() {
		fakeTeamDB = new(dbfakes.FakeTeamDB)
		fakeTeamDBFactory := new(dbfakes.FakeTeamDBFactory)
		fakeTeamDBFactory.GetTeamDBReturns(fakeTeamDB)
		oauthFactory = provider.NewOAuthFactory(
			lagertest.NewTestLogger("test"),
			fakeTeamDBFactory,
			"http://foo.bar",
			auth.OAuthRoutes,
			auth.OAuthCallback,
		)
	})

	Describe("Get Providers", func() {
		Context("when GitHub provider is setup", func() {
			BeforeEach(func() {
				savedTeam := db.SavedTeam{
					Team: db.Team{
						Name: atc.DefaultTeamName,
						GitHubAuth: &db.GitHubAuth{
							ClientID:     "user1",
							ClientSecret: "password1",
							Users:        []string{"thecandyman"},
						},
					},
				}
				fakeTeamDB.GetTeamReturns(savedTeam, true, nil)
			})

			It("returns back GitHub's auth provider", func() {
				providers, err := oauthFactory.GetProviders(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(providers).To(HaveLen(1))
				Expect(providers[github.ProviderName]).NotTo(BeNil())
			})
		})

		Context("when UAA provider is setup", func() {
			BeforeEach(func() {
				savedTeam := db.SavedTeam{
					Team: db.Team{
						Name: atc.DefaultTeamName,
						UAAAuth: &db.UAAAuth{
							ClientID:     "user1",
							ClientSecret: "password1",
							CFSpaces:     []string{"myspace"},
						},
					},
				}
				fakeTeamDB.GetTeamReturns(savedTeam, true, nil)
			})

			It("returns back UAA's auth provider", func() {
				providers, err := oauthFactory.GetProviders(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(providers).To(HaveLen(1))
				Expect(providers[uaa.ProviderName]).NotTo(BeNil())
			})
		})

		Context("when UAA provider has an invalid ssl cert", func() {
			BeforeEach(func() {
				savedTeam := db.SavedTeam{
					Team: db.Team{
						Name: atc.DefaultTeamName,
						GitHubAuth: &db.GitHubAuth{
							ClientID:     "user1",
							ClientSecret: "password1",
							Users:        []string{"thecandyman"},
						},
						UAAAuth: &db.UAAAuth{
							ClientID:     "user1",
							ClientSecret: "password1",
							CFSpaces:     []string{"myspace"},
							CFCACert:     "some-invalid-cert",
						},
					},
				}
				fakeTeamDB.GetTeamReturns(savedTeam, true, nil)
			})

			It("returns the other providers", func() {
				providers, err := oauthFactory.GetProviders(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(providers).To(HaveLen(1))
				Expect(providers[github.ProviderName]).NotTo(BeNil())
			})
		})

		Context("when no provider is setup", func() {
			BeforeEach(func() {
				savedTeam := db.SavedTeam{
					Team: db.Team{
						Name: atc.DefaultTeamName,
					},
				}
				fakeTeamDB.GetTeamReturns(savedTeam, true, nil)
			})

			It("returns an empty map", func() {
				providers, err := oauthFactory.GetProviders(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(providers).To(BeEmpty())
			})
		})

		Context("when team does not exist", func() {
			BeforeEach(func() {
				fakeTeamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
			})

			It("returns an error", func() {
				_, err := oauthFactory.GetProviders(atc.DefaultTeamName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("team not found"))
			})
		})
	})
})
