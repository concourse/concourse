package integration_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RBAC", func() {

	Context("Teams", func() {
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
					createdTeam, _, _, err := ccClient.Team(team.Name).CreateOrUpdate(team)

					Expect(err).ToNot(HaveOccurred())
					Expect(createdTeam.Name).To(Equal(team.Name))
					Expect(createdTeam.Auth).To(Equal(team.Auth))
				})
			})
		})
	})
})
