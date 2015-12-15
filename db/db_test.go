package db_test

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lib/pq"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
)

var _ = Describe("SQL DB", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var database db.DB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("UpdateTeam", func() {
		var basicAuthTeam, githubAuthTeam db.Team

		BeforeEach(func() {
			expectedTeam := db.Team{
				Name: "avengers",
			}
			_, err := database.SaveTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())

			basicAuthTeam = db.Team{
				Name: "avengers",
				BasicAuth: db.BasicAuth{
					BasicAuthUsername: "fake user",
					BasicAuthPassword: "no, bad",
				},
			}

			githubAuthTeam = db.Team{Name: "avengers",
				GithubAuth: db.GithubAuth{
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
				},
			}

		})

		It("saves basic auth team info to the existing team", func() {
			savedTeam, err := database.UpdateTeamBasicAuth(basicAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			Expect(savedTeam.BasicAuthUsername).To(Equal(basicAuthTeam.BasicAuthUsername))
			Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuthPassword),
				[]byte(basicAuthTeam.BasicAuthPassword))).To(BeNil())
		})

		It("saves oauth team info to the existing team", func() {
			savedTeam, err := database.UpdateTeamGithubAuth(githubAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			Expect(savedTeam.ClientID).To(Equal(githubAuthTeam.ClientID))
			Expect(savedTeam.ClientSecret).To(Equal(githubAuthTeam.ClientSecret))
			Expect(savedTeam.Organizations).To(Equal(githubAuthTeam.Organizations))
			Expect(savedTeam.Teams).To(Equal(githubAuthTeam.Teams))
			Expect(savedTeam.Users).To(Equal(githubAuthTeam.Users))
		})

		It("saves github auth team info without over writing the basic auth", func() {
			_, err := database.UpdateTeamGithubAuth(githubAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			savedTeam, err := database.UpdateTeamBasicAuth(basicAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			Expect(savedTeam.ClientID).To(Equal(githubAuthTeam.ClientID))
			Expect(savedTeam.ClientSecret).To(Equal(githubAuthTeam.ClientSecret))
			Expect(savedTeam.Organizations).To(Equal(githubAuthTeam.Organizations))
			Expect(savedTeam.Teams).To(Equal(githubAuthTeam.Teams))
			Expect(savedTeam.Users).To(Equal(githubAuthTeam.Users))
		})

		It("saves basic auth team info without over writing the github auth", func() {
			_, err := database.UpdateTeamBasicAuth(basicAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			savedTeam, err := database.UpdateTeamGithubAuth(githubAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			Expect(savedTeam.BasicAuthUsername).To(Equal(basicAuthTeam.BasicAuthUsername))
			Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuthPassword),
				[]byte(basicAuthTeam.BasicAuthPassword))).To(BeNil())
		})
	})

	Describe("CreateTeam", func() {
		It("saves a team to the db", func() {
			expectedTeam := db.Team{
				Name: "avengers",
			}
			expectedSavedTeam, err := database.SaveTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team).To(Equal(expectedTeam))

			savedTeam, err := database.GetTeamByName("avengers")
			Expect(err).NotTo(HaveOccurred())
			Expect(savedTeam).To(Equal(expectedSavedTeam))
		})
	})

	Describe("CreatePipe", func() {
		It("saves a pipe to the db", func() {
			myGuid, err := uuid.NewV4()
			Expect(err).NotTo(HaveOccurred())

			err = database.CreatePipe(myGuid.String(), "a-url")
			Expect(err).NotTo(HaveOccurred())

			pipe, err := database.GetPipe(myGuid.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(pipe.ID).To(Equal(myGuid.String()))
			Expect(pipe.URL).To(Equal("a-url"))
		})
	})

	It("saves and propagates events correctly", func() {
		build, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		Expect(build.Name).To(Equal("1"))

		By("allowing you to subscribe when no events have yet occurred")
		events, err := database.GetBuildEvents(build.ID, 0)
		Expect(err).NotTo(HaveOccurred())

		defer events.Close()

		By("saving them in order")
		err = database.SaveBuildEvent(build.ID, event.Log{
			Payload: "some ",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(events.Next()).To(Equal(event.Log{
			Payload: "some ",
		}))

		err = database.SaveBuildEvent(build.ID, event.Log{
			Payload: "log",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(events.Next()).To(Equal(event.Log{
			Payload: "log",
		}))

		By("allowing you to subscribe from an offset")
		eventsFrom1, err := database.GetBuildEvents(build.ID, 1)
		Expect(err).NotTo(HaveOccurred())

		defer eventsFrom1.Close()

		Expect(eventsFrom1.Next()).To(Equal(event.Log{
			Payload: "log",
		}))

		By("notifying those waiting on events as soon as they're saved")
		nextEvent := make(chan atc.Event)
		nextErr := make(chan error)

		go func() {
			event, err := events.Next()
			if err != nil {
				nextErr <- err
			} else {
				nextEvent <- event
			}
		}()

		Consistently(nextEvent).ShouldNot(Receive())
		Consistently(nextErr).ShouldNot(Receive())

		err = database.SaveBuildEvent(build.ID, event.Log{
			Payload: "log 2",
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(nextEvent).Should(Receive(Equal(event.Log{
			Payload: "log 2",
		})))

		By("returning ErrBuildEventStreamClosed for Next calls after Close")
		events3, err := database.GetBuildEvents(build.ID, 0)
		Expect(err).NotTo(HaveOccurred())

		err = events3.Close()
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			_, err := events3.Next()
			return err
		}).Should(Equal(db.ErrBuildEventStreamClosed))
	})

	It("saves and emits status events", func() {
		build, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		Expect(build.Name).To(Equal("1"))

		By("allowing you to subscribe when no events have yet occurred")
		events, err := database.GetBuildEvents(build.ID, 0)
		Expect(err).NotTo(HaveOccurred())

		defer events.Close()

		By("emitting a status event when started")
		started, err := database.StartBuild(build.ID, "engine", "metadata")
		Expect(err).NotTo(HaveOccurred())
		Expect(started).To(BeTrue())

		startedBuild, found, err := database.GetBuild(build.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(events.Next()).To(Equal(event.Status{
			Status: atc.StatusStarted,
			Time:   startedBuild.StartTime.Unix(),
		}))

		By("emitting a status event when finished")
		err = database.FinishBuild(build.ID, db.StatusSucceeded)
		Expect(err).NotTo(HaveOccurred())

		finishedBuild, found, err := database.GetBuild(build.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(events.Next()).To(Equal(event.Status{
			Status: atc.StatusSucceeded,
			Time:   finishedBuild.EndTime.Unix(),
		}))

		By("ending the stream when finished")
		_, err = events.Next()
		Expect(err).To(Equal(db.ErrEndOfBuildEventStream))
	})
})
