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

var _ = Describe("OAuthFactory", func() {
	var fakeFactoryDB *fakes.FakeFactoryDB
	var oauthFactory OAuthFactory

	BeforeEach(func() {
		fakeFactoryDB = new(fakes.FakeFactoryDB)
		oauthFactory = NewOAuthFactory(
			fakeFactoryDB,
			"http://foo.bar",
			auth.OAuthRoutes,
			auth.OAuthCallback,
		)
	})

	Describe("Get Providers", func() {
		Describe("GitHub Provider", func() {
			Context("when the provider is setup", func() {
				BeforeEach(func() {
					savedTeam := db.SavedTeam{
						Team: db.Team{
							Name: atc.DefaultTeamName,
							GitHubAuth: db.GitHubAuth{
								ClientID:     "user1",
								ClientSecret: "password1",
								Users:        []string{"thecandyman"},
							},
						},
					}
					fakeFactoryDB.GetTeamByNameReturns(savedTeam, nil)
				})

				It("returns back GitHub's auth provider", func() {
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
					fakeFactoryDB.GetTeamByNameReturns(savedTeam, nil)
				})

				It("returns an empty map", func() {
					providers, err := oauthFactory.GetProviders(atc.DefaultTeamName)
					Expect(err).NotTo(HaveOccurred())
					Expect(providers).To(BeEmpty())
				})
			})
		})
	})
})
