package db_test

import (
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamDB", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database db.DB
	var teamDBFactory db.TeamDBFactory

	var teamDB db.TeamDB
	var caseInsensitiveTeamDB db.TeamDB
	var nonExistentTeamDB db.TeamDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		teamDBFactory = db.NewTeamDBFactory(dbConn)
		database = db.NewSQL(dbConn, bus)

		team := db.Team{Name: "team-name"}
		_, err := database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())

		teamDB = teamDBFactory.GetTeamDB("team-name")
		caseInsensitiveTeamDB = teamDBFactory.GetTeamDB("TEAM-name")
		nonExistentTeamDB = teamDBFactory.GetTeamDB("non-existent-name")
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetPipielineByName", func() {
		var savedPipeline db.SavedPipeline
		BeforeEach(func() {
			var err error
			savedPipeline, _, err = teamDB.SaveConfig("pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			team := db.Team{Name: "other-team-name"}
			_, err = database.CreateTeam(team)
			Expect(err).NotTo(HaveOccurred())
			otherTeamDB := teamDBFactory.GetTeamDB("other-team-name")
			_, _, err = otherTeamDB.SaveConfig("pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the pipeline with the name that belongs to the team", func() {
			actualPipeline, err := teamDB.GetPipelineByName("pipeline-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualPipeline).To(Equal(savedPipeline))
		})
	})

	Describe("GetPipelines", func() {
		var savedPipeline1 db.SavedPipeline
		var savedPipeline2 db.SavedPipeline

		BeforeEach(func() {
			var err error
			savedPipeline1, _, err = teamDB.SaveConfig("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			savedPipeline2, _, err = teamDB.SaveConfig("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			team := db.Team{Name: "other-team-name"}
			_, err = database.CreateTeam(team)
			Expect(err).NotTo(HaveOccurred())
			otherTeamDB := teamDBFactory.GetTeamDB("other-team-name")

			_, _, err = otherTeamDB.SaveConfig("other-team-pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = otherTeamDB.SaveConfig("other-team-pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns pipelines that belong to team", func() {
			savedPipelines, err := teamDB.GetPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(savedPipelines).To(HaveLen(2))
			Expect(savedPipelines).To(ConsistOf(savedPipeline1, savedPipeline2))
		})
	})

	Describe("OrderPipelines", func() {
		var otherTeamDB db.TeamDB
		var savedPipeline1 db.SavedPipeline
		var savedPipeline2 db.SavedPipeline
		var otherTeamSavedPipeline1 db.SavedPipeline
		var otherTeamSavedPipeline2 db.SavedPipeline

		BeforeEach(func() {
			var err error
			savedPipeline1, _, err = teamDB.SaveConfig("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			savedPipeline2, _, err = teamDB.SaveConfig("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			team := db.Team{Name: "other-team-name"}
			_, err = database.CreateTeam(team)
			Expect(err).NotTo(HaveOccurred())
			otherTeamDB = teamDBFactory.GetTeamDB("other-team-name")

			otherTeamSavedPipeline1, _, err = otherTeamDB.SaveConfig("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			otherTeamSavedPipeline2, _, err = otherTeamDB.SaveConfig("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
		})

		It("orders pipelines that belong to team", func() {
			err := teamDB.OrderPipelines([]string{"pipeline-name-b", "pipeline-name-a"})
			Expect(err).NotTo(HaveOccurred())

			err = otherTeamDB.OrderPipelines([]string{"pipeline-name-a", "pipeline-name-b"})
			Expect(err).NotTo(HaveOccurred())

			orderedPipelines, err := teamDB.GetPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(orderedPipelines).To(HaveLen(2))
			Expect(orderedPipelines[0].ID).To(Equal(savedPipeline2.ID))
			Expect(orderedPipelines[1].ID).To(Equal(savedPipeline1.ID))

			otherTeamOrderedPipelines, err := otherTeamDB.GetPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(otherTeamOrderedPipelines).To(HaveLen(2))
			Expect(otherTeamOrderedPipelines[0].ID).To(Equal(otherTeamSavedPipeline1.ID))
			Expect(otherTeamOrderedPipelines[1].ID).To(Equal(otherTeamSavedPipeline2.ID))
		})
	})

	Describe("Updating Auth", func() {
		var basicAuth *db.BasicAuth
		var gitHubAuth *db.GitHubAuth
		var cfAuth *db.CFAuth

		BeforeEach(func() {
			basicAuth = &db.BasicAuth{
				BasicAuthUsername: "fake user",
				BasicAuthPassword: "no, bad",
			}

			gitHubAuth = &db.GitHubAuth{
				ClientID:      "fake id",
				ClientSecret:  "some secret",
				Organizations: []string{"a", "b", "c"},
				Teams: []db.GitHubTeam{
					{
						OrganizationName: "org1",
						TeamName:         "teama",
					},
					{
						OrganizationName: "org2",
						TeamName:         "teamb",
					},
				},
				Users: []string{"user1", "user2", "user3"},
			}

			cfAuth = &db.CFAuth{
				ClientID:     "fake id",
				ClientSecret: "some secret",
				Spaces:       []string{"a", "b", "c"},
				AuthURL:      "https://some.auth.url",
				TokenURL:     "https://some.token.url",
				APIURL:       "https://some.api.url",
			}
		})

		Describe("UpdateBasicAuth", func() {
			It("saves basic auth team info without overwriting the github auth", func() {
				_, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GitHubAuth).To(Equal(gitHubAuth))
			})

			It("saves basic auth team info without overwriting the cf auth", func() {
				_, err := teamDB.UpdateCFAuth(cfAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.CFAuth).To(Equal(cfAuth))
			})

			It("saves basic auth team info to the existing team", func() {
				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth.BasicAuthUsername).To(Equal(basicAuth.BasicAuthUsername))
				Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuth.BasicAuthPassword),
					[]byte(basicAuth.BasicAuthPassword))).To(BeNil())
			})

			It("nulls basic auth when has a blank username", func() {
				basicAuth.BasicAuthUsername = ""
				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth).To(BeNil())
			})

			It("nulls basic auth when has a blank password", func() {
				basicAuth.BasicAuthPassword = ""
				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth).To(BeNil())
			})

			It("saves basic auth team info to the existing team when team name is case-insensitive", func() {
				savedTeam, err := caseInsensitiveTeamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth.BasicAuthUsername).To(Equal(basicAuth.BasicAuthUsername))
				Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuth.BasicAuthPassword),
					[]byte(basicAuth.BasicAuthPassword))).To(BeNil())
			})
		})

		Describe("UpdateGitHubAuth", func() {
			It("saves github auth team info to the existing team", func() {
				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GitHubAuth).To(Equal(gitHubAuth))
			})

			It("nulls github auth when has a blank clientSecret", func() {
				gitHubAuth.ClientSecret = ""
				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GitHubAuth).To(BeNil())
			})

			It("nulls github auth when has a blank clientID", func() {
				gitHubAuth.ClientID = ""
				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GitHubAuth).To(BeNil())
			})

			It("saves github auth team info without over writing the basic auth", func() {
				_, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth.BasicAuthUsername).To(Equal(basicAuth.BasicAuthUsername))
				Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuth.BasicAuthPassword),
					[]byte(basicAuth.BasicAuthPassword))).To(BeNil())
			})

			It("saves github auth team info without over writing the cf auth", func() {
				_, err := teamDB.UpdateCFAuth(cfAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.CFAuth).To(Equal(cfAuth))
			})

			It("saves github auth team info to the existing team when team name is case-insensitive", func() {
				savedTeam, err := caseInsensitiveTeamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GitHubAuth).To(Equal(gitHubAuth))
			})
		})

		Describe("UpdateCFAuth", func() {
			It("saves cf auth team info to the existing team", func() {
				savedTeam, err := teamDB.UpdateCFAuth(cfAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.CFAuth).To(Equal(cfAuth))
			})

			It("saves cf auth team info to the existing team when team name is caseinsensitive", func() {
				savedTeam, err := caseInsensitiveTeamDB.UpdateCFAuth(cfAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.CFAuth).To(Equal(cfAuth))
			})
		})
	})

	Describe("GetTeam", func() {
		It("returns the saved team", func() {
			actualTeam, found, err := teamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualTeam.Name).To(Equal("team-name"))
		})

		It("returns the saved team when team name is case-insensitive", func() {
			actualTeam, found, err := caseInsensitiveTeamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualTeam.Name).To(Equal("team-name"))
		})

		It("returns false with no error when the team does not exist", func() {
			_, found, err := nonExistentTeamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})
})
