package github_test

import (
	"fmt"
	"net/http"

	gogithub "github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/skymarshal/github"
)

var _ = Describe("Client", func() {
	var (
		githubServer *ghttp.Server

		client github.Client

		proxiedClient *http.Client
	)

	BeforeEach(func() {
		githubServer = ghttp.NewServer()

		client = github.NewClient("")

		proxiedClient = &http.Client{
			Transport: proxiedTransport{githubServer},
		}
	})

	Describe("CurrentUser", func() {
		Context("when getting the current user succeeds", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/user"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, gogithub.User{
							Login: gogithub.String("some-user"),
						}),
					),
				)
			})

			It("returns the user's login", func() {
				user, err := client.CurrentUser(proxiedClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(user).To(Equal("some-user"))
			})
		})

		Context("when getting the current user fails", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/user"),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.CurrentUser(proxiedClient)
				Expect(err).To(BeAssignableToTypeOf(&gogithub.ErrorResponse{}))
			})
		})
	})

	Describe("Organizations", func() {
		Context("when listing organization succeeds", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/user/orgs", "page=1"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Organization{
								{Login: gogithub.String("org-1")},
								{Login: gogithub.String("org-2")},
							},
							http.Header{
								"Link": []string{
									fmt.Sprintf(`<http://%s/user/orgs?page=2>; rel="next"`, githubServer.Addr()),
									fmt.Sprintf(`<http://%s/user/orgs?page=2>; rel="last"`, githubServer.Addr()),
								},
							},
						),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/user/orgs", "page=2"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Organization{
								{Login: gogithub.String("org-3")},
							},
							http.Header{
								"Link": []string{
									fmt.Sprintf(`<http://%s/user/orgs?page=1>; rel="first"`, githubServer.Addr()),
									fmt.Sprintf(`<http://%s/user/orgs?page=1>; rel="prev"`, githubServer.Addr()),
								},
							},
						),
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
	})

	Describe("Teams", func() {
		Context("when listing teams succeeds", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/user/teams", "page=1"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Team{
								{Name: gogithub.String("Team 1"), Slug: gogithub.String("team-1"),
									Organization: &gogithub.Organization{Login: gogithub.String("org-1")}},
								{Name: gogithub.String("Team 2"), Slug: gogithub.String("team-2"),
									Organization: &gogithub.Organization{Login: gogithub.String("org-1")}},
							},
							http.Header{
								"Link": []string{
									fmt.Sprintf(`<http://%s/user/teams?page=2>; rel="next"`, githubServer.Addr()),
									fmt.Sprintf(`<http://%s/user/teams?page=2>; rel="last"`, githubServer.Addr()),
								},
							},
						),
					),

					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/user/teams", "page=2"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Team{
								{Name: gogithub.String("Team 3"), Slug: gogithub.String("team-3"),
									Organization: &gogithub.Organization{Login: gogithub.String("org-2")}},
							},
							http.Header{
								"Link": []string{
									fmt.Sprintf(`<http://%s/user/teams?page=1>; rel="first"`, githubServer.Addr()),
									fmt.Sprintf(`<http://%s/user/teams?page=1>; rel="prev"`, githubServer.Addr()),
								},
							},
						),
					),
				)
			})

			It("returns the map of organization to team names", func() {
				teams, err := client.Teams(proxiedClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(teams).To(HaveLen(2))
				Expect(teams["org-1"]).To(ConsistOf([]string{"Team 1", "team-1", "Team 2", "team-2"}))
				Expect(teams["org-2"]).To(ConsistOf([]string{"Team 3", "team-3"}))
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

	Describe("Github Enterprise", func() {
		BeforeEach(func() {
			client = github.NewClient("https://github.example.com/api/v3/")
		})

		Context("when getting the current user succeeds", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v3/user"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, gogithub.User{
							Login: gogithub.String("some-user"),
						}),
					),
				)
			})

			It("returns the user's login", func() {
				user, err := client.CurrentUser(proxiedClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(user).To(Equal("some-user"))
			})
		})

		Context("when listing teams succeeds", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v3/user/teams", "page=1"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Team{
								{Name: gogithub.String("Team 1"), Slug: gogithub.String("team-1"),
									Organization: &gogithub.Organization{Login: gogithub.String("org-1")}},
								{Name: gogithub.String("Team 2"), Slug: gogithub.String("team-2"),
									Organization: &gogithub.Organization{Login: gogithub.String("org-1")}},
							},
							http.Header{
								"Link": []string{
									`<https://github.example.com/api/v3/user/teams?page=2>; rel="next"`,
									`<https://github.example.com/api/v3/user/teams?page=2>; rel="last"`,
								},
							},
						),
					),

					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v3/user/teams", "page=2"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Team{
								{Name: gogithub.String("Team 3"), Slug: gogithub.String("team-3"),
									Organization: &gogithub.Organization{Login: gogithub.String("org-2")}},
							},
							http.Header{
								"Link": []string{
									`<https://github.example.com/api/v3/user/teams?page=1>; rel="first"`,
									`<https://github.example.com/api/v3/user/teams?page=1>; rel="prev"`,
								},
							},
						),
					),
				)
			})

			It("returns the map of organization to team names", func() {
				teams, err := client.Teams(proxiedClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(teams).To(HaveLen(2))
				Expect(teams["org-1"]).To(ConsistOf([]string{"Team 1", "team-1", "Team 2", "team-2"}))
				Expect(teams["org-2"]).To(ConsistOf([]string{"Team 3", "team-3"}))
			})
		})

		Context("when listing organization succeeds", func() {
			BeforeEach(func() {
				githubServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v3/user/orgs", "page=1"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Organization{
								{Login: gogithub.String("org-1")},
								{Login: gogithub.String("org-2")},
							},
							http.Header{
								"Link": []string{
									`<https://github.example.com/api/v3/user/orgs?page=2>; rel="next"`,
									`<https://github.example.com/api/v3/user/orgs?page=2>; rel="last"`,
								},
							},
						),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v3/user/orgs", "page=2"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogithub.Organization{
								{Login: gogithub.String("org-3")},
							},
							http.Header{
								"Link": []string{
									`<https://github.example.com/api/v3/user/orgs?page=1>; rel="first"`,
									`<https://github.example.com/api/v3/user/orgs?page=1>; rel="prev"`,
								},
							},
						),
					),
				)
			})

			It("returns the list of organization names", func() {
				orgs, err := client.Organizations(proxiedClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(orgs).To(Equal([]string{"org-1", "org-2", "org-3"}))
			})
		})

		Context("when invalid URL is provded", func() {
			BeforeEach(func() {
				client = github.NewClient("%invalid-url")
			})

			It("returns an error", func() {
				_, err := client.CurrentUser(proxiedClient)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid github auth API URL '%invalid-url'"))
			})
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
