package provider_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/github"
	. "github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/provider/fakes"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OauthFactory", func() {
	var fakeFactoryDB *fakes.FakeFactoryDB
	var oauthFactory OauthFactory

	BeforeEach(func() {
		fakeFactoryDB = new(fakes.FakeFactoryDB)
		oauthFactory = NewOauthFactory(
			fakeFactoryDB,
			"http://foo.bar",
			auth.OAuthRoutes,
			auth.OAuthCallback,
		)
	})

	Describe("Get Providers", func() {
		Describe("Github Provider", func() {
			BeforeEach(func() {
				savedTeam := db.SavedTeam{
					Team: db.Team{
						Name: atc.DefaultTeamName,
						GitHubAuth: db.GitHubAuth{
							ClientID:     "user1",
							ClientSecret: "password1",
						},
					},
				}
				fakeFactoryDB.GetTeamByNameReturns(savedTeam, nil)
			})

			It("returns back Github's auth provider", func() {
				providers, err := oauthFactory.GetProviders(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(providers).To(HaveLen(1))
				Expect(providers[github.ProviderName]).NotTo(BeNil())
			})
		})
	})
})
