package acceptance_test

import (
	"errors"
	"net/http"
	"time"

	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Auth", func() {
	var atcCommand *ATCCommand
	var dbListener *pq.Listener
	var teamDB db.TeamDB

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbConn, bus)

		err := sqlDB.DeleteTeamByName(atc.DefaultPipelineName)
		Expect(err).NotTo(HaveOccurred())
		_, err = sqlDB.CreateTeam(db.Team{Name: atc.DefaultTeamName})
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus)
		teamDB = teamDBFactory.GetTeamDB(atc.DefaultTeamName)

		_, _, err = teamDB.SaveConfig(atc.DefaultPipelineName, atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("GitHub Auth", func() {
		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, GITHUB_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		It("forces a redirect to /teams/main/login", func() {
			request, err := http.NewRequest("GET", atcCommand.URL("/teams/main/pipelines/main"), nil)
			resp, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.Path).To(Equal("/teams/main/login"))

			team, _, err := teamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(team.GitHubAuth.ClientID).To(Equal("admin"))
			Expect(team.GitHubAuth.ClientSecret).To(Equal("password"))
			Expect(team.GitHubAuth.Organizations).To(Equal([]string{"myorg"}))
		})
	})

	Describe("GitHub Enterprise Auth", func() {
		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, GITHUB_ENTERPRISE_AUTH)
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
		var response *http.Response
		var responseErr error

		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when requesting protected endpoint", func() {
			BeforeEach(func() {
				request, err := http.NewRequest("GET", atcCommand.URL("/teams/main/pipelines/main"), nil)
				Expect(err).NotTo(HaveOccurred())
				response, responseErr = http.DefaultClient.Do(request)
			})

			It("forces a redirect to /teams/main/login", func() {
				Expect(responseErr).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(response.Request.URL.Path).To(Equal("/teams/main/login"))
			})
		})

		Context("when requesting a team-specific route as not authenticated", func() {
			BeforeEach(func() {
				_, err := sqlDB.CreateTeam(db.Team{
					Name: "some-team",
					BasicAuth: &db.BasicAuth{
						BasicAuthUsername: "username",
						BasicAuthPassword: "passord",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				request, err := http.NewRequest("POST", atcCommand.URL("/teams/some-team/pipelines/some-pipeline/jobs/foo/builds"), nil)
				Expect(err).NotTo(HaveOccurred())
				response, responseErr = http.DefaultClient.Do(request)
			})

			It("forces a redirect to /teams/:team_name/login", func() {
				Expect(responseErr).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(response.Request.URL.Path).To(Equal("/teams/some-team/login"))
			})
		})

		Context("when requesting another team-specific route but not authorized", func() {
			BeforeEach(func() {
				_, err := sqlDB.CreateTeam(db.Team{Name: "some-team"})
				Expect(err).NotTo(HaveOccurred())

				request, err := http.NewRequest("POST", atcCommand.URL("/teams/some-team/pipelines/some-pipeline/jobs/foo/builds"), nil)
				Expect(err).NotTo(HaveOccurred())
				response, responseErr = http.DefaultClient.Do(request)
			})

			It("forces a redirect to /teams/:team_name/login", func() {
				Expect(responseErr).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(response.Request.URL.Path).To(Equal("/teams/some-team/login"))
			})
		})

		It("errors when only username is specified", func() {
			cmd := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH_NO_PASSWORD)
			session, err := cmd.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --basic-auth-password to use basic auth"))
		})

		It("errors when only password is specified", func() {
			cmd := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH_NO_USERNAME)
			session, err := cmd.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --basic-auth-username to use basic auth"))
		})
	})

	Describe("No authentication via development mode", func() {
		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, DEVELOPMENT_MODE)
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
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, NOT_CONFIGURED_AUTH)
			session, err := atcCommand.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must configure basic auth, OAuth, or turn on development mode"))
		})
	})

	Describe("UAA Auth", func() {
		BeforeEach(func() {
			atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, UAA_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		It("forces a redirect to UAA auth URL", func() {
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
			cmd := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, UAA_AUTH_NO_CLIENT_SECRET)
			session, err := cmd.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --uaa-auth-client-id and --uaa-auth-client-secret to use UAA OAuth"))
		})

		It("requires space guid to be specified", func() {
			cmd := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, UAA_AUTH_NO_SPACE)
			session, err := cmd.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --uaa-auth-cf-space to use UAA OAuth"))
		})

		It("requires auth, token and api url to be specified", func() {
			cmd := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, UAA_AUTH_NO_TOKEN_URL)
			session, err := cmd.StartAndWait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("must specify --uaa-auth-auth-url, --uaa-auth-token-url and --uaa-auth-cf-url to use UAA OAuth"))
		})
	})
})
