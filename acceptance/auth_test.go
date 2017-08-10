package acceptance_test

import (
	"errors"
	"net/http"

	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Auth", func() {
	var atcCommand *ATCCommand

	BeforeEach(func() {
		_, _, err := defaultTeam.SavePipeline(atc.DefaultPipelineName, atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		atcCommand.Stop()
	})

	Describe("GitHub Auth", func() {
		It("requires client id and client secret to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GITHUB_AUTH_NO_CLIENT_SECRET)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth."))
		})

		It("requires organizations, teams or users to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GITHUB_AUTH_NO_TEAM)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("at least one of the following is required for github-auth: organizations, teams, users."))
		})
	})

	Describe("GitHub Enterprise Auth", func() {
		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GITHUB_ENTERPRISE_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		It("forces a redirect to override github", func() {
			request, err := http.NewRequest("GET", atcCommand.URL("/auth/github?redirect=%2F&team_name=main"), nil)

			client := new(http.Client)
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("error")
			}
			resp, err := client.Do(request)
			Expect(err.Error()).To(ContainSubstring("https://github.example.com/login/oauth/authorize"))
			Expect(resp.StatusCode).To(Equal(http.StatusTemporaryRedirect))
		})
	})

	Describe("Basic Auth", func() {
		It("errors when only username is specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, BASIC_AUTH_NO_PASSWORD)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --basic-auth-password to use basic auth"))
		})

		It("errors when only password is specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, BASIC_AUTH_NO_USERNAME)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --basic-auth-username to use basic auth"))
		})
	})

	Describe("No authentication via no auth flag", func() {
		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, NO_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		It("logs in without authentication", func() {
			request, err := http.NewRequest("GET", atcCommand.URL("/"), nil)
			resp, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.Path).To(Equal("/"))
		})
	})

	Describe("when auth is not configured", func() {
		It("returns an error", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, NOT_CONFIGURED_AUTH)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must configure basic auth, OAuth, UAAAuth, or provide no-auth flag"))
		})
	})

	Describe("UAA Auth", func() {
		It("forces a redirect to UAA auth URL", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, UAA_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
			request, err := http.NewRequest("GET", atcCommand.URL("/auth/uaa?redirect=%2F&team_name=main"), nil)

			client := new(http.Client)
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("error")
			}
			resp, err := client.Do(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("https://uaa.example.com/oauth/authorize"))
			Expect(resp.StatusCode).To(Equal(http.StatusTemporaryRedirect))
		})

		It("requires client id and client secret to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, UAA_AUTH_NO_CLIENT_SECRET)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --uaa-auth-client-id and --uaa-auth-client-secret to use UAA OAuth"))
		})

		It("requires space guid to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, UAA_AUTH_NO_SPACE)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --uaa-auth-cf-space to use UAA OAuth"))
		})

		It("requires auth, token and api url to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, UAA_AUTH_NO_TOKEN_URL)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --uaa-auth-auth-url, --uaa-auth-token-url and --uaa-auth-cf-url to use UAA OAuth"))
		})
	})

	Describe("Generic OAuth Auth", func() {
		It("forces a redirect to the auth URL", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GENERIC_OAUTH_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("GET", atcCommand.URL("/auth/oauth?redirect=%2F&team_name=main"), nil)

			client := new(http.Client)
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("error")
			}
			resp, err := client.Do(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("https://goa.example.com/oauth/authorize"))
			Expect(resp.StatusCode).To(Equal(http.StatusTemporaryRedirect))
		})

		It("shows the option on the login page", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GENERIC_OAUTH_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())

			var page *agouti.Page
			page, err = agoutiDriver.NewPage()
			Expect(err).NotTo(HaveOccurred())

			Expect(page.Navigate(atcCommand.URL("/teams/main/login"))).To(Succeed())

			Eventually(page.FindByLink("login with Example")).Should(BeFound())
		})

		It("can pass parameters to the auth URL", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GENERIC_OAUTH_AUTH_PARAMS)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("GET", atcCommand.URL("/auth/oauth?redirect=%2F&team_name=main"), nil)

			client := new(http.Client)
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("error")
			}
			resp, err := client.Do(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("https://goa.example.com/oauth/authorize"))
			Expect(err.Error()).To(ContainSubstring("param1=value1"))
			Expect(err.Error()).To(ContainSubstring("param2=value2"))
			Expect(resp.StatusCode).To(Equal(http.StatusTemporaryRedirect))
		})

		It("requires client id and client secret to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GENERIC_OAUTH_AUTH_NO_CLIENT_SECRET)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --generic-oauth-client-id and --generic-oauth-client-secret to use Generic OAuth"))
		})

		It("requires authorization url and token url to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GENERIC_OAUTH_AUTH_NO_TOKEN_URL)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --generic-oauth-auth-url and --generic-oauth-token-url to use Generic OAuth"))
		})

		It("requires display name to be specified", func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, GENERIC_OAUTH_AUTH_NO_DISPLAY_NAME)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --generic-oauth-display-name to use Generic OAuth"))
		})
	})
})
