package integration_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	concourse "github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"golang.org/x/oauth2"
)

var _ = Describe("ATC Integration Test", func() {
	var (
		atcProcess ifrit.Process
		atcURL     string
	)

	BeforeEach(func() {
		atcURL = fmt.Sprintf("http://localhost:%v", cmd.BindPort)

		runner, err := cmd.Runner([]string{})
		Expect(err).NotTo(HaveOccurred())

		atcProcess = ifrit.Invoke(runner)

		Eventually(func() error {
			_, err := http.Get(atcURL + "/api/v1/info")
			return err
		}, 20*time.Second).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		atcProcess.Signal(os.Interrupt)
		<-atcProcess.Wait()
	})

	Context("Teams", func() {
		var team atc.Team
		var pipelineData = []byte(`
---
jobs:
- name: simple
`)

		BeforeEach(func() {
			team = atc.Team{Name: "some-team"}
		})

		JustBeforeEach(func() {
			setupTeam(atcURL, team)
			setupPipeline(atcURL, team.Name, pipelineData)
		})

		Context("when there are defined roles for users", func() {
			Context("when the role is viewer", func() {
				BeforeEach(func() {
					team.Auth = atc.TeamAuth{
						"viewer": map[string][]string{
							"users":  []string{"local:v-user"},
							"groups": []string{},
						},
						"owner": map[string][]string{
							"users":  []string{"local:test"},
							"groups": []string{},
						},
					}
				})

				It("should be able to view pipelines", func() {
					ccClient := login(atcURL, "v-user", "v-user")

					pipelines, err := ccClient.Team(team.Name).ListPipelines()
					Expect(err).ToNot(HaveOccurred())
					Expect(pipelines).To(HaveLen(1))
				})

				It("should NOT be able to set pipelines", func() {
					ccClient := login(atcURL, "v-user", "v-user")

					_, _, _, err := ccClient.Team(team.Name).CreateOrUpdatePipelineConfig("pipeline-new", "0", pipelineData, false)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("forbidden"))
				})
			})

			Context("when the role is member", func() {
				BeforeEach(func() {
					team.Auth = atc.TeamAuth{
						"member": map[string][]string{
							"users":  []string{"local:m-user"},
							"groups": []string{},
						},
						"owner": map[string][]string{
							"users":  []string{"local:test"},
							"groups": []string{},
						},
					}
				})

				It("should be able to view the pipelines", func() {
					ccClient := login(atcURL, "m-user", "m-user")

					pipelines, err := ccClient.Team(team.Name).ListPipelines()
					Expect(err).ToNot(HaveOccurred())
					Expect(pipelines).To(HaveLen(1))
				})

				It("should be able to set pipelines", func() {
					ccClient := login(atcURL, "m-user", "m-user")

					_, _, _, err := ccClient.Team(team.Name).CreateOrUpdatePipelineConfig("pipeline-new", "0", pipelineData, false)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the role is owner", func() {
				BeforeEach(func() {
					team.Auth = atc.TeamAuth{
						"owner": map[string][]string{
							"users":  []string{"local:o-user", "local:test"},
							"groups": []string{},
						},
					}
				})

				It("should be able to view the pipelines", func() {
					ccClient := login(atcURL, "o-user", "o-user")

					pipelines, err := ccClient.Team(team.Name).ListPipelines()
					Expect(err).ToNot(HaveOccurred())
					Expect(pipelines).To(HaveLen(1))
				})

				It("should be able to set pipelines", func() {
					ccClient := login(atcURL, "o-user", "o-user")

					_, _, _, err := ccClient.Team(team.Name).CreateOrUpdatePipelineConfig("pipeline-new", "0", pipelineData, false)
					Expect(err).ToNot(HaveOccurred())
				})

				It("can update the auth for a team", func() {
					team.Auth = atc.TeamAuth{
						"viewer": map[string][]string{
							"users":  []string{"local:v-user"},
							"groups": []string{},
						},
						"owner": map[string][]string{
							"users":  []string{"local:o-user", "local:test"},
							"groups": []string{},
						},
					}

					ccClient := login(atcURL, "o-user", "o-user")
					createdTeam, _, _, err := ccClient.Team(team.Name).CreateOrUpdate(team)

					Expect(err).ToNot(HaveOccurred())
					Expect(createdTeam.Name).To(Equal(team.Name))
					Expect(createdTeam.Auth).To(Equal(team.Auth))
				})
			})
		})
	})
})

func login(atcURL, username, password string) concourse.Client {
	oauth2Config := oauth2.Config{
		ClientID:     "fly",
		ClientSecret: "Zmx5",
		Endpoint:     oauth2.Endpoint{TokenURL: atcURL + "/sky/token"},
		Scopes:       []string{"openid", "federated:id"},
	}

	token, err := oauth2Config.PasswordCredentialsToken(context.Background(), username, password)
	Expect(err).NotTo(HaveOccurred())

	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return concourse.NewClient(atcURL, httpClient, false)
}

func setupTeam(atcURL string, team atc.Team) {
	ccClient := login(atcURL, "test", "test")
	createdTeam, _, _, err := ccClient.Team(team.Name).CreateOrUpdate(team)

	Expect(err).ToNot(HaveOccurred())
	Expect(createdTeam.Name).To(Equal(team.Name))
	Expect(createdTeam.Auth).To(Equal(team.Auth))
}

func setupPipeline(atcURL, teamName string, config []byte) {
	ccClient := login(atcURL, "test", "test")
	_, _, _, err := ccClient.Team(teamName).CreateOrUpdatePipelineConfig("pipeline-name", "0", config, false)
	Expect(err).ToNot(HaveOccurred())
}
