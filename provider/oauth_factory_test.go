package provider_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/genericoauth"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/uaa"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OAuthFactory", func() {
	var oauthFactory provider.OAuthFactory

	BeforeEach(func() {
		oauthFactory = provider.NewOAuthFactory(
			lagertest.NewTestLogger("test"),
			"http://foo.bar",
			auth.OAuthRoutes,
			auth.OAuthCallback,
		)
	})

	Describe("GetProvider", func() {
		Context("when asking for github provider", func() {
			Context("when github provider is setup", func() {
				It("returns back GitHub's auth provider", func() {
					provider, found, err := oauthFactory.GetProvider(db.SavedTeam{
						Team: db.Team{
							Name: "some-team",
							GitHubAuth: &db.GitHubAuth{
								ClientID:     "user1",
								ClientSecret: "password1",
								Users:        []string{"thecandyman"},
							},
						},
					}, github.ProviderName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(provider).NotTo(BeNil())
				})
			})

			Context("when github provider is not setup", func() {
				It("returns false", func() {
					_, found, err := oauthFactory.GetProvider(db.SavedTeam{
						Team: db.Team{
							Name: "some-team",
						},
					}, github.ProviderName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when asking for uaa provider", func() {
			Context("when UAA provider is setup", func() {
				It("returns back UAA's auth provider", func() {
					provider, found, err := oauthFactory.GetProvider(db.SavedTeam{
						Team: db.Team{
							Name: "some-team",
							UAAAuth: &db.UAAAuth{
								ClientID:     "user1",
								ClientSecret: "password1",
							},
						},
					}, uaa.ProviderName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(provider).NotTo(BeNil())
				})
			})

			Context("when uaa provider is not setup", func() {
				It("returns false", func() {
					_, found, err := oauthFactory.GetProvider(db.SavedTeam{
						Team: db.Team{
							Name: "some-team",
						},
					}, uaa.ProviderName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when asking for goa provider", func() {
			Context("when Generic OAuth provider is setup", func() {
				It("returns back GOA's auth provider", func() {
					provider, found, err := oauthFactory.GetProvider(db.SavedTeam{
						Team: db.Team{
							Name: "some-team",
							GenericOAuth: &db.GenericOAuth{
								ClientID:     "user1",
								ClientSecret: "password1",
							},
						},
					}, genericoauth.ProviderName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(provider).NotTo(BeNil())
				})
			})

			Context("when Generic OAuth provider is not setup", func() {
				It("returns false", func() {
					_, found, err := oauthFactory.GetProvider(db.SavedTeam{
						Team: db.Team{
							Name: "some-team",
						},
					}, genericoauth.ProviderName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when asking for unknown provider", func() {
			It("returns false", func() {
				_, found, err := oauthFactory.GetProvider(db.SavedTeam{
					Team: db.Team{
						Name: "some-team",
					},
				}, "bogus")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
