package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RBAC", func() {

	var team atc.Team
	var pipelineData = []byte(`
---
jobs:
- name: simple
`)

	JustBeforeEach(func() {
		team = atc.Team{
			Name: "some-team",
			Auth: atc.TeamAuth{
				"viewer": map[string][]string{
					"users":  []string{"local:v-user"},
					"groups": []string{},
				},
				"pipeline-operator": map[string][]string{
					"users":  []string{"local:po-user"},
					"groups": []string{},
				},
				"member": map[string][]string{
					"users":  []string{"local:m-user"},
					"groups": []string{},
				},
				"owner": map[string][]string{
					"users":  []string{"local:o-user", "local:test"},
					"groups": []string{},
				},
			},
		}

		setupTeam(atcURL, team)
		setupPipeline(atcURL, team.Name, pipelineData)
	})

	Context("Default RBAC values", func() {

		Context("when there are defined roles for users", func() {
			Context("when the role is viewer", func() {
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

			Context("when the role is pipeline-operator", func() {
				It("should be able to view the pipelines", func() {
					ccClient := login(atcURL, "po-user", "po-user")

					pipelines, err := ccClient.Team(team.Name).ListPipelines()
					Expect(err).ToNot(HaveOccurred())
					Expect(pipelines).To(HaveLen(1))
				})

				It("should NOT be able to set pipelines", func() {
					ccClient := login(atcURL, "po-user", "po-user")

					_, _, _, err := ccClient.Team(team.Name).CreateOrUpdatePipelineConfig("pipeline-new", "0", pipelineData, false)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("forbidden"))
				})
			})

			Context("when the role is member", func() {
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
					createdTeam, _, _, _, err := ccClient.Team(team.Name).CreateOrUpdate(team)

					Expect(err).ToNot(HaveOccurred())
					Expect(createdTeam.Name).To(Equal(team.Name))
					Expect(createdTeam.Auth).To(Equal(team.Auth))
				})
			})
		})
	})

	Context("Customize RBAC", func() {

		var (
			rbac string
			tmp  string
		)

		BeforeEach(func() {
			var err error
			tmp, err = ioutil.TempDir("", fmt.Sprintf("tmp-%d", GinkgoParallelNode()))
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := os.RemoveAll(tmp)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when trying to customize an action that doesn't exist", func() {
			BeforeEach(func() {
				rbac = `
---
viewer:
- NotSaveConfig
`
			})

			It("errors", func() {
				file := filepath.Join(tmp, "rbac-not-action.yml")
				err := ioutil.WriteFile(file, []byte(rbac), 0755)
				Expect(err).ToNot(HaveOccurred())

				cmd.ConfigRBAC = flag.File(file)

				// workaround to avoid panic due to registering http handlers multiple times
				http.DefaultServeMux = new(http.ServeMux)

				_, err = cmd.Runner([]string{})
				Expect(err).To(MatchError(ContainSubstring("failed to customize roles: unknown action NotSaveConfig")))
			})
		})

		Context("when trying to customize a role that doesn't exist", func() {
			BeforeEach(func() {
				rbac = `
---
not-viewer:
- SaveConfig
`
			})

			It("errors", func() {
				file := filepath.Join(tmp, "rbac-not-role.yml")
				err := ioutil.WriteFile(file, []byte(rbac), 0755)
				Expect(err).ToNot(HaveOccurred())

				cmd.ConfigRBAC = flag.File(file)

				// workaround to avoid panic due to registering http handlers multiple times
				http.DefaultServeMux = new(http.ServeMux)

				_, err = cmd.Runner([]string{})
				Expect(err).To(MatchError(ContainSubstring("failed to customize roles: unknown role not-viewer")))
			})
		})

		Context("when successfully customizing a role", func() {
			BeforeEach(func() {
				rbac = `
---
viewer:
- SaveConfig
`
				file := filepath.Join(tmp, "rbac.yml")
				err := ioutil.WriteFile(file, []byte(rbac), 0755)
				Expect(err).ToNot(HaveOccurred())

				cmd.ConfigRBAC = flag.File(file)
			})

			It("viewer should be able to set pipelines", func() {
				ccClient := login(atcURL, "v-user", "v-user")

				_, _, _, err := ccClient.Team(team.Name).CreateOrUpdatePipelineConfig("pipeline-new", "0", pipelineData, false)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
