package db_test

import (
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = XDescribe("TeamDB", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		database      db.DB
		teamDBFactory db.TeamDBFactory

		teamDB            db.TeamDB
		otherTeamDB       db.TeamDB
		nonExistentTeamDB db.TeamDB
		savedTeam         db.SavedTeam
		otherSavedTeam    db.SavedTeam
		pipelineDBFactory db.PipelineDBFactory
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(lockfakes.FakeConnector)
		retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := lock.NewLockFactory(retryableConn)
		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		database = db.NewSQL(dbConn, bus, lockFactory)

		team := db.Team{Name: "TEAM-name"}
		var err error
		savedTeam, err = database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())

		teamDB = teamDBFactory.GetTeamDB("team-NAME")
		nonExistentTeamDB = teamDBFactory.GetTeamDB("non-existent-name")

		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		team = db.Team{Name: "other-team-name"}
		otherSavedTeam, err = database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())
		otherTeamDB = teamDBFactory.GetTeamDB("other-team-name")
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetPipelineByName", func() {
		var savedPipeline db.SavedPipeline
		BeforeEach(func() {
			var err error
			savedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = otherTeamDB.SaveConfigToBeDeprecated("pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the pipeline with the case insensitive name that belongs to the team", func() {
			actualPipeline, found, err := teamDB.GetPipelineByName("pipeline-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualPipeline).To(Equal(savedPipeline))
		})
	})

	Describe("GetPipelines", func() {
		var savedPipeline1 db.SavedPipeline
		var savedPipeline2 db.SavedPipeline

		BeforeEach(func() {
			var err error
			savedPipeline1, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			savedPipeline2, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			otherSavedPublicPipeline, _, err := otherTeamDB.SaveConfigToBeDeprecated("other-team-pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = otherTeamDB.SaveConfigToBeDeprecated("other-team-pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(otherSavedPublicPipeline)
			err = pipelineDB.Expose()
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns only the pipelines that belong to team (case insensitive)", func() {
			savedPipelines, err := teamDB.GetPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(savedPipelines).To(HaveLen(2))
			Expect(savedPipelines).To(ConsistOf(savedPipeline1, savedPipeline2))
		})
	})

	Describe("GetPublicPipelines", func() {
		var publicPipeline db.SavedPipeline

		BeforeEach(func() {
			_, _, err := teamDB.SaveConfigToBeDeprecated("private-pipeline", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			publicPipeline, _, err = teamDB.SaveConfigToBeDeprecated("public-pipeline", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB := pipelineDBFactory.Build(publicPipeline)
			err = pipelineDB.Expose()
			Expect(err).NotTo(HaveOccurred())

			// update expectations
			publicPipeline.Public = true
		})

		It("returns only the pipelines that belong to team (case insensitive)", func() {
			savedPipelines, err := teamDB.GetPublicPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(savedPipelines).To(HaveLen(1))
			Expect(savedPipelines).To(ConsistOf(publicPipeline))
		})
	})

	Describe("GetPrivateAndAllPublicPipelines", func() {
		var savedPipeline1 db.SavedPipeline
		var savedPipeline2 db.SavedPipeline
		var savedPipeline3 db.SavedPipeline
		var otherSavedPublicPipeline1 db.SavedPipeline
		var otherSavedPublicPipeline2 db.SavedPipeline
		var otherSavedPublicPipeline3 db.SavedPipeline
		BeforeEach(func() {
			var err error
			savedPipeline1, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			savedPipeline2, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			savedPipeline3, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-c", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			otherSavedPublicPipeline1, _, err = otherTeamDB.SaveConfigToBeDeprecated("other-team-pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			otherSavedPublicPipeline2, _, err = otherTeamDB.SaveConfigToBeDeprecated("other-team-pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			otherSavedPublicPipeline3, _, err = otherTeamDB.SaveConfigToBeDeprecated("other-team-pipeline-name-c", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			pipelineDB1 := pipelineDBFactory.Build(savedPipeline1)
			err = pipelineDB1.Expose()
			Expect(err).NotTo(HaveOccurred())

			pipelineDB2 := pipelineDBFactory.Build(savedPipeline2)
			err = pipelineDB2.Expose()
			Expect(err).NotTo(HaveOccurred())

			otherPipelineDB1 := pipelineDBFactory.Build(otherSavedPublicPipeline1)
			err = otherPipelineDB1.Expose()
			Expect(err).NotTo(HaveOccurred())

			otherPipelineDB3 := pipelineDBFactory.Build(otherSavedPublicPipeline3)
			err = otherPipelineDB3.Expose()
			Expect(err).NotTo(HaveOccurred())

			// update expectations
			savedPipeline1.Public = true
			savedPipeline2.Public = true
			otherSavedPublicPipeline1.Public = true
			otherSavedPublicPipeline3.Public = true

			err = teamDB.OrderPipelines([]string{"pipeline-name-b", "pipeline-name-c", "pipeline-name-a"})
			Expect(err).NotTo(HaveOccurred())

			err = otherTeamDB.OrderPipelines([]string{"other-team-pipeline-name-c", "other-team-pipeline-name-b", "other-team-pipeline-name-a"})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the pipelines of the team first, followed by public pipelines from other teams (case insensitive)", func() {
			savedPipelines, err := teamDB.GetPrivateAndAllPublicPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(savedPipelines).To(Equal([]db.SavedPipeline{
				savedPipeline2,
				savedPipeline3,
				savedPipeline1,
				otherSavedPublicPipeline3,
				otherSavedPublicPipeline1,
			}))

			savedPipelines, err = otherTeamDB.GetPrivateAndAllPublicPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(savedPipelines).To(Equal([]db.SavedPipeline{
				otherSavedPublicPipeline3,
				otherSavedPublicPipeline2,
				otherSavedPublicPipeline1,
				savedPipeline2,
				savedPipeline1,
			}))
		})

	})

	Describe("OrderPipelines", func() {
		var savedPipeline1 db.SavedPipeline
		var savedPipeline2 db.SavedPipeline
		var otherTeamSavedPipeline1 db.SavedPipeline
		var otherTeamSavedPipeline2 db.SavedPipeline

		BeforeEach(func() {
			var err error
			savedPipeline1, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			savedPipeline2, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			otherTeamSavedPipeline1, _, err = otherTeamDB.SaveConfigToBeDeprecated("pipeline-name-a", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			otherTeamSavedPipeline2, _, err = otherTeamDB.SaveConfigToBeDeprecated("pipeline-name-b", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
		})

		It("orders pipelines that belong to team (case insensitive)", func() {
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
		var genericOAuth *db.GenericOAuth

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

			genericOAuth = &db.GenericOAuth{
				DisplayName:   "Cyborgs",
				ClientID:      "some random guid",
				ClientSecret:  "don't tell anyone",
				AuthURL:       "https://auth.url",
				AuthURLParams: map[string]string{"option": "1"},
				Scope:         "read",
				TokenURL:      "https://token.url",
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

			It("saves basic auth team info without overwriting the Generic OAuth info", func() {
				_, err := teamDB.UpdateGenericOAuth(genericOAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateBasicAuth(basicAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GenericOAuth).To(Equal(genericOAuth))
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

			It("saves github auth team info without over writing the Generic OAuth info", func() {
				_, err := teamDB.UpdateGenericOAuth(genericOAuth)
				Expect(err).NotTo(HaveOccurred())

				savedTeam, err := teamDB.UpdateGitHubAuth(gitHubAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.GenericOAuth).To(Equal(genericOAuth))
			})
		})

		Describe("UpdateUAAAuth", func() {
			It("saves cf auth team info to the existing team", func() {
				savedTeam, err := teamDB.UpdateUAAAuth(uaaAuth)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.UAAAuth).To(Equal(uaaAuth))
			})
		})

		Describe("UpdateGenericOAuth", func() {
			It("saves generic oauth info to the existing team", func() {
				savedTeam, err := teamDB.UpdateGenericOAuth(genericOAuth)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedTeam.GenericOAuth).To(Equal(genericOAuth))
			})
		})
	})

	Describe("GetTeam", func() {
		It("returns the saved team", func() {
			actualTeam, found, err := teamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualTeam.Name).To(Equal("TEAM-name"))
		})

		It("returns false with no error when the team does not exist", func() {
			_, found, err := nonExistentTeamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("CreateOneOffBuild", func() {
		var (
			oneOffBuild db.Build
			err         error
		)

		BeforeEach(func() {
			oneOffBuild, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can create one-off builds with increasing names", func() {
			nextOneOffBuild, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextOneOffBuild.ID()).NotTo(BeZero())
			Expect(nextOneOffBuild.ID()).NotTo(Equal(oneOffBuild.ID()))
			Expect(nextOneOffBuild.JobName()).To(BeZero())
			Expect(nextOneOffBuild.Name()).To(Equal("2"))
			Expect(nextOneOffBuild.TeamName()).To(Equal(savedTeam.Name))
			Expect(nextOneOffBuild.Status()).To(Equal(db.StatusPending))
		})
	})

	Describe("Workers", func() {
		BeforeEach(func() {
			_, err := database.SaveWorker(db.WorkerInfo{
				GardenAddr: "1.2.3.4",
				Name:       "my-team-worker",
				TeamID:     savedTeam.ID,
			}, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.SaveWorker(db.WorkerInfo{
				GardenAddr: "1.2.3.5",
				Name:       "other-team-worker",
				TeamID:     otherSavedTeam.ID,
			}, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.SaveWorker(db.WorkerInfo{
				GardenAddr: "1.2.3.6",
				Name:       "shared-worker",
			}, 5*time.Minute)

			_, err = database.SaveWorker(db.WorkerInfo{
				GardenAddr: "1.2.3.7",
				Name:       "expired-worker",
				TeamID:     savedTeam.ID,
			}, -time.Minute)

			Expect(err).NotTo(HaveOccurred())
		})

		It("returns non-expired workers that belong to current team or that do not belong to any team", func() {
			workers, err := teamDB.Workers()
			Expect(err).NotTo(HaveOccurred())
			Expect(workers).To(HaveLen(2))
			Expect([]db.SavedWorker{
				{
					WorkerInfo: db.WorkerInfo{
						Name:   workers[0].Name,
						TeamID: workers[0].TeamID,
					},
					TeamName: workers[0].TeamName,
				},
				{
					WorkerInfo: db.WorkerInfo{
						Name:   workers[1].Name,
						TeamID: workers[1].TeamID,
					},
					TeamName: workers[1].TeamName,
				},
			}).To(ConsistOf([]db.SavedWorker{
				{
					WorkerInfo: db.WorkerInfo{
						Name:   "my-team-worker",
						TeamID: savedTeam.ID,
					},
					TeamName: savedTeam.Name,
				},
				{
					WorkerInfo: db.WorkerInfo{
						Name:   "shared-worker",
						TeamID: 0,
					},
					TeamName: "",
				},
			}))

			otherTeamWorkers, err := otherTeamDB.Workers()
			Expect(err).NotTo(HaveOccurred())
			Expect(otherTeamWorkers).To(HaveLen(2))
			Expect([]string{
				otherTeamWorkers[0].Name,
				otherTeamWorkers[1].Name,
			}).To(ConsistOf([]string{"other-team-worker", "shared-worker"}))
		})

		It("returns shared workers if current team has no workers", func() {
			noWorkersTeamDB := teamDBFactory.GetTeamDB("no-workers-team-name")
			workers, err := noWorkersTeamDB.Workers()
			Expect(err).NotTo(HaveOccurred())
			Expect(workers).To(HaveLen(1))
			Expect(workers[0].Name).To(Equal("shared-worker"))
		})

		It("reaps expired workers", func() {
			_, err := database.SaveWorker(db.WorkerInfo{
				GardenAddr: "1.2.3.7",
				Name:       "expired-worker",
				TeamID:     savedTeam.ID,
			}, 1*time.Nanosecond)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(2 * time.Nanosecond)

			workers, err := teamDB.Workers()
			Expect(err).NotTo(HaveOccurred())
			Expect(workers).To(HaveLen(2))
			Expect([]string{
				workers[0].Name,
				workers[1].Name,
			}).To(ConsistOf([]string{"my-team-worker", "shared-worker"}))
		})
	})

	Describe("GetPrivateAndPublicBuilds", func() {
		Context("when there are no builds", func() {
			It("returns an empty list of builds", func() {
				builds, pagination, err := teamDB.GetPrivateAndPublicBuilds(db.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(pagination.Next).To(BeNil())
				Expect(pagination.Previous).To(BeNil())
				Expect(builds).To(BeEmpty())
			})
		})

		Context("when there are builds", func() {
			var allBuilds [5]db.Build
			var pipelineDB db.PipelineDB
			var pipelineBuilds [2]db.Build

			BeforeEach(func() {
				for i := 0; i < 3; i++ {
					build, err := teamDB.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())
					allBuilds[i] = build
				}

				config := atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
				}
				pipeline, _, err := teamDB.SaveConfigToBeDeprecated("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pipelineDB = pipelineDBFactory.Build(pipeline)

				for i := 3; i < 5; i++ {
					build, err := pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
					allBuilds[i] = build
					pipelineBuilds[i-3] = build
				}
			})

			It("returns all team builds with correct pagination", func() {
				builds, pagination, err := teamDB.GetPrivateAndPublicBuilds(db.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[4]))
				Expect(builds[1]).To(Equal(allBuilds[3]))

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[3].ID(), Limit: 2}))

				builds, pagination, err = teamDB.GetPrivateAndPublicBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))

				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID(), Limit: 2}))

				builds, pagination, err = teamDB.GetPrivateAndPublicBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0]).To(Equal(allBuilds[0]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[0].ID(), Limit: 2}))
				Expect(pagination.Next).To(BeNil())

				builds, pagination, err = teamDB.GetPrivateAndPublicBuilds(*pagination.Previous)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID(), Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID(), Limit: 2}))
			})

			Context("when there are builds that belong to different teams", func() {
				var teamABuilds [3]db.Build
				var teamBBuilds [3]db.Build

				var caseInsensitiveTeamADB db.TeamDB
				var caseInsensitiveTeamBDB db.TeamDB

				BeforeEach(func() {
					_, err := database.CreateTeam(db.Team{Name: "team-a"})
					Expect(err).NotTo(HaveOccurred())

					_, err = database.CreateTeam(db.Team{Name: "team-b"})
					Expect(err).NotTo(HaveOccurred())

					caseInsensitiveTeamADB = teamDBFactory.GetTeamDB("team-A")
					caseInsensitiveTeamBDB = teamDBFactory.GetTeamDB("team-B")

					for i := 0; i < 3; i++ {
						teamABuilds[i], err = caseInsensitiveTeamADB.CreateOneOffBuild()
						Expect(err).NotTo(HaveOccurred())

						teamBBuilds[i], err = caseInsensitiveTeamBDB.CreateOneOffBuild()
						Expect(err).NotTo(HaveOccurred())
					}
				})

				Context("when other team builds are private", func() {
					It("returns only builds for requested team", func() {
						builds, _, err := caseInsensitiveTeamADB.GetPrivateAndPublicBuilds(db.Page{Limit: 10})
						Expect(err).NotTo(HaveOccurred())

						Expect(len(builds)).To(Equal(3))
						Expect(builds).To(ConsistOf(teamABuilds))

						builds, _, err = caseInsensitiveTeamBDB.GetPrivateAndPublicBuilds(db.Page{Limit: 10})
						Expect(err).NotTo(HaveOccurred())

						Expect(len(builds)).To(Equal(3))
						Expect(builds).To(ConsistOf(teamBBuilds))
					})
				})

				Context("when other team builds are public", func() {
					BeforeEach(func() {
						pipelineDB.Expose()
					})

					It("returns builds for requested team and public builds", func() {
						builds, _, err := caseInsensitiveTeamADB.GetPrivateAndPublicBuilds(db.Page{Limit: 10})
						Expect(err).NotTo(HaveOccurred())

						Expect(builds).To(HaveLen(5))
						expectedBuilds := []db.Build{}
						for _, b := range teamABuilds {
							expectedBuilds = append(expectedBuilds, b)
						}
						for _, b := range pipelineBuilds {
							expectedBuilds = append(expectedBuilds, b)
						}
						Expect(builds).To(ConsistOf(expectedBuilds))
					})
				})
			})
		})
	})
})
