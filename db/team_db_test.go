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
	var (
		dbConn   db.Conn
		listener *pq.Listener

		database      db.DB
		teamDBFactory db.TeamDBFactory

		teamDB                db.TeamDB
		caseInsensitiveTeamDB db.TeamDB
		nonExistentTeamDB     db.TeamDB
		savedTeam             db.SavedTeam

		pipelineDBFactory db.PipelineDBFactory
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus)
		database = db.NewSQL(dbConn, bus)

		team := db.Team{Name: "team-name"}
		var err error
		savedTeam, err = database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())

		teamDB = teamDBFactory.GetTeamDB("team-name")
		caseInsensitiveTeamDB = teamDBFactory.GetTeamDB("TEAM-name")
		nonExistentTeamDB = teamDBFactory.GetTeamDB("non-existent-name")

		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus)
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
		var uaaAuth *db.UAAAuth

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

			uaaAuth = &db.UAAAuth{
				ClientID:     "fake id",
				ClientSecret: "some secret",
				AuthURL:      "https://some.auth.url",
				TokenURL:     "https://some.token.url",
				CFSpaces:     []string{"a", "b", "c"},
				CFURL:        "https://some.api.url",
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
				_, err := teamDB.UpdateUAAAuth(uaaAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.UAAAuth).To(Equal(uaaAuth))
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
				_, err := teamDB.UpdateUAAAuth(uaaAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.UAAAuth).To(Equal(uaaAuth))
			})

			It("saves github auth team info to the existing team when team name is case-insensitive", func() {
				savedTeam, err := caseInsensitiveTeamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GitHubAuth).To(Equal(gitHubAuth))
			})
		})

		Describe("UpdateUAAAuth", func() {
			It("saves cf auth team info to the existing team", func() {
				savedTeam, err := teamDB.UpdateUAAAuth(uaaAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.UAAAuth).To(Equal(uaaAuth))
			})

			It("saves cf auth team info to the existing team when team name is caseinsensitive", func() {
				savedTeam, err := caseInsensitiveTeamDB.UpdateUAAAuth(uaaAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.UAAAuth).To(Equal(uaaAuth))
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

	Describe("CreateOneOffBuild", func() {
		var (
			oneOffBuildDB db.BuildDB
			err           error
		)

		BeforeEach(func() {
			oneOffBuildDB, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can create one-off builds with increasing names", func() {
			nextOneOffBuildDB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextOneOffBuildDB.GetID()).NotTo(BeZero())
			Expect(nextOneOffBuildDB.GetID()).NotTo(Equal(oneOffBuildDB.GetID()))
			Expect(nextOneOffBuildDB.GetJobName()).To(BeZero())
			Expect(nextOneOffBuildDB.GetName()).To(Equal("2"))
			Expect(nextOneOffBuildDB.GetTeamName()).To(Equal(savedTeam.Name))
			Expect(nextOneOffBuildDB.GetStatus()).To(Equal(db.StatusPending))
		})

		It("also creates buildpreparation", func() {
			buildPrep, found, err := oneOffBuildDB.GetPreparation()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(buildPrep.BuildID).To(Equal(oneOffBuildDB.GetID()))
		})
	})

	Describe("GetBuilds", func() {
		Context("when there are no builds", func() {
			It("returns an empty list of builds", func() {
				builds, pagination, err := teamDB.GetBuilds(db.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(pagination.Next).To(BeNil())
				Expect(pagination.Previous).To(BeNil())
				Expect(builds).To(BeEmpty())
			})
		})

		Context("when there are builds", func() {
			var allBuilds [5]db.BuildDB

			BeforeEach(func() {
				for i := 0; i < 3; i++ {
					buildDB, err := teamDB.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())
					allBuilds[i] = buildDB
				}

				config := atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
				}
				pipeline, _, err := teamDB.SaveConfig("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pipelineDB := pipelineDBFactory.Build(pipeline)

				for i := 3; i < 5; i++ {
					build, err := pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
					allBuilds[i] = build
				}
			})

			It("returns all builds that have been started, regardless of pipeline", func() {
				builds, pagination, err := teamDB.GetBuilds(db.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[4]))
				Expect(builds[1]).To(Equal(allBuilds[3]))

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[3].GetID(), Limit: 2}))

				builds, pagination, err = teamDB.GetBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].GetID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].GetID(), Limit: 2}))

				builds, pagination, err = teamDB.GetBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0]).To(Equal(allBuilds[0]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[0].GetID(), Limit: 2}))
				Expect(pagination.Next).To(BeNil())

				builds, pagination, err = teamDB.GetBuilds(*pagination.Previous)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].GetID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].GetID(), Limit: 2}))
			})

			Context("when there are builds that belong to different teams", func() {
				var teamABuilds [3]db.BuildDB
				var teamBBuilds [3]db.BuildDB

				var teamADB db.TeamDB
				var teamBDB db.TeamDB

				BeforeEach(func() {
					_, err := database.CreateTeam(db.Team{Name: "team-a"})
					Expect(err).NotTo(HaveOccurred())

					_, err = database.CreateTeam(db.Team{Name: "team-b"})
					Expect(err).NotTo(HaveOccurred())

					teamADB = teamDBFactory.GetTeamDB("team-a")
					teamBDB = teamDBFactory.GetTeamDB("team-b")

					for i := 0; i < 3; i++ {
						teamABuilds[i], err = teamADB.CreateOneOffBuild()
						Expect(err).NotTo(HaveOccurred())

						teamBBuilds[i], err = teamBDB.CreateOneOffBuild()
						Expect(err).NotTo(HaveOccurred())
					}
				})

				It("returns only builds for requested team", func() {
					builds, _, err := teamADB.GetBuilds(db.Page{Limit: 10})
					Expect(err).NotTo(HaveOccurred())

					Expect(len(builds)).To(Equal(3))
					Expect(builds).To(ConsistOf(teamABuilds))

					builds, _, err = teamBDB.GetBuilds(db.Page{Limit: 10})
					Expect(err).NotTo(HaveOccurred())

					Expect(len(builds)).To(Equal(3))
					Expect(builds).To(ConsistOf(teamBBuilds))
				})
			})
		})
	})

	Describe("GetBuildDB", func() {
		It("returns build that belong to current team", func() {
			originalBuildDB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			buildDB, found, err := teamDB.GetBuildDB(originalBuildDB.GetID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(buildDB.GetID()).To(Equal(originalBuildDB.GetID()))
		})

		It("does not return build that belongs to another team", func() {
			team := db.Team{Name: "another-team-name"}
			_, err := database.CreateTeam(team)
			Expect(err).NotTo(HaveOccurred())
			anotherTeamDB := teamDBFactory.GetTeamDB("another-team-name")
			anotherTeamBuildDB, err := anotherTeamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			buildDB, found, err := teamDB.GetBuildDB(anotherTeamBuildDB.GetID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(buildDB).To(BeNil())
		})
	})
})
