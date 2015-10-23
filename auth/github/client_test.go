package github_test

import (
	"net/http"

	gogithub "github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc/auth/github"
)

var _ = Describe("Client", func() {
	var (
		githubServer *ghttp.Server

		client github.Client

		proxiedClient *http.Client
	)

	BeforeEach(func() {
		githubServer = ghttp.NewServer()

		client = github.NewClient()

		proxiedClient = &http.Client{
			Transport: proxiedTransport{githubServer},
		}
	})

	Context("when listing organization succeeds", func() {
		BeforeEach(func() {
			githubServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/user/orgs"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []gogithub.Organization{
						{Login: gogithub.String("org-1")},
						{Login: gogithub.String("org-2")},
						{Login: gogithub.String("org-3")},
					}),
				),
			)
		})

		It("returns the list of organization names", func() {
			orgs, err := client.Organizations(proxiedClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(orgs).To(Equal([]string{"org-1", "org-2", "org-3"}))
		})
	})

	Context("when listing organization fails", func() {
		BeforeEach(func() {
			githubServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/user/orgs"),
					ghttp.RespondWith(http.StatusUnauthorized, ""),
				),
			)
		})

		It("returns an error", func() {
			_, err := client.Organizations(proxiedClient)
			Expect(err).To(BeAssignableToTypeOf(&gogithub.ErrorResponse{}))
		})
	})

	Context("when listing teams succeeds", func() {
		BeforeEach(func() {
			githubServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/user/teams"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []gogithub.Team{
						{Slug: gogithub.String("team-1"),
							Organization: &gogithub.Organization{Login: gogithub.String("org-1")}},
						{Slug: gogithub.String("team-2"),
							Organization: &gogithub.Organization{Login: gogithub.String("org-1")}},
						{Slug: gogithub.String("team-3"),
							Organization: &gogithub.Organization{Login: gogithub.String("org-2")}},
					}),
				),
			)
		})

		It("returns the map of organization to team names", func() {
			teams, err := client.Teams(proxiedClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(teams).To(HaveLen(2))
			Expect(teams["org-1"]).To(ConsistOf([]string{"team-1", "team-2"}))
			Expect(teams["org-2"]).To(ConsistOf([]string{"team-3"}))
		})
	})

	Context("when listing teams fails", func() {
		BeforeEach(func() {
			githubServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/user/teams"),
					ghttp.RespondWith(http.StatusUnauthorized, ""),
				),
			)
		})

		It("returns an error", func() {
			_, err := client.Teams(proxiedClient)
			Expect(err).To(BeAssignableToTypeOf(&gogithub.ErrorResponse{}))
		})
	})
})

type proxiedTransport struct {
	proxy *ghttp.Server
}

func (t proxiedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := *req.URL
	newURL.Scheme = "http"
	newURL.Host = t.proxy.Addr()

	newReq := *req
	newReq.URL = &newURL

	return (&http.Transport{}).RoundTrip(&newReq)
}
