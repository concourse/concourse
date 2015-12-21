package db_test

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("SQL DB Teams", func() {
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

		database.DeleteTeamByName(atc.DefaultTeamName)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("the default team", func() {
		Describe("it exists", func() {
			It("it does not get duplicated", func() {
				err := database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())
				err = database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())
				team, err := database.GetTeamByName(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(team.Name).To(Equal(atc.DefaultTeamName))
			})
		})

		Describe("it does not exist", func() {
			It("it gets created", func() {
				err := database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())
				team, err := database.GetTeamByName(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(team.Name).To(Equal(atc.DefaultTeamName))
			})
		})
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

			githubAuthTeam = db.Team{
				Name: "avengers",
				GitHubAuth: db.GitHubAuth{
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

		Context("required basic auth elements are not present", func() {

			BeforeEach(func() {
				_, err := database.UpdateTeamBasicAuth(basicAuthTeam)
				Expect(err).NotTo(HaveOccurred())
			})

			It("nulls basic auth when has a blank username", func() {
				basicAuthTeam.BasicAuthUsername = ""
				savedTeam, err := database.UpdateTeamBasicAuth(basicAuthTeam)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth.BasicAuthUsername).To(BeEmpty())
				Expect(savedTeam.BasicAuth.BasicAuthPassword).To(BeEmpty())
			})

			It("nulls basic auth when has a blank password", func() {
				basicAuthTeam.BasicAuthPassword = ""
				savedTeam, err := database.UpdateTeamBasicAuth(basicAuthTeam)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.BasicAuth.BasicAuthUsername).To(BeEmpty())
				Expect(savedTeam.BasicAuth.BasicAuthPassword).To(BeEmpty())
			})
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

		Context("required github auth elements are not present", func() {

			BeforeEach(func() {
				_, err := database.UpdateTeamGithubAuth(githubAuthTeam)
				Expect(err).NotTo(HaveOccurred())
			})

			It("nulls basic auth when has a blank clientSecret", func() {
				githubAuthTeam.ClientSecret = ""
				savedTeam, err := database.UpdateTeamGithubAuth(githubAuthTeam)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.ClientID).To(BeEmpty())
				Expect(savedTeam.ClientSecret).To(BeEmpty())
				Expect(savedTeam.Organizations).To(BeEmpty())
				Expect(savedTeam.Teams).To(BeEmpty())
				Expect(savedTeam.Users).To(BeEmpty())
			})

			It("nulls basic auth when has a blank clientID", func() {
				githubAuthTeam.ClientID = ""
				savedTeam, err := database.UpdateTeamGithubAuth(githubAuthTeam)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.ClientID).To(BeEmpty())
				Expect(savedTeam.ClientSecret).To(BeEmpty())
				Expect(savedTeam.Organizations).To(BeEmpty())
				Expect(savedTeam.Teams).To(BeEmpty())
				Expect(savedTeam.Users).To(BeEmpty())
			})
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

	Describe("SaveTeam", func() {
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

		It("saves a team to the db with basic auth", func() {
			expectedTeam := db.Team{
				Name: "avengers",
				BasicAuth: db.BasicAuth{
					BasicAuthUsername: "fake user",
					BasicAuthPassword: "no, bad",
				},
			}
			expectedSavedTeam, err := database.SaveTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team.Name).To(Equal(expectedTeam.Name))

			savedTeam, err := database.GetTeamByName("avengers")
			Expect(err).NotTo(HaveOccurred())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.BasicAuthUsername).To(Equal(expectedTeam.BasicAuthUsername))
			Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuthPassword),
				[]byte(expectedTeam.BasicAuthPassword))).To(BeNil())
		})

		It("saves a team to the db with github auth", func() {
			expectedTeam := db.Team{
				Name: "avengers",
				GitHubAuth: db.GitHubAuth{
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
			expectedSavedTeam, err := database.SaveTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team).To(Equal(expectedTeam))

			savedTeam, err := database.GetTeamByName("avengers")
			Expect(err).NotTo(HaveOccurred())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.ClientID).To(Equal(expectedTeam.ClientID))
			Expect(savedTeam.ClientSecret).To(Equal(expectedTeam.ClientSecret))
			Expect(savedTeam.Organizations).To(Equal(expectedTeam.Organizations))
			Expect(savedTeam.Teams).To(Equal(expectedTeam.Teams))
			Expect(savedTeam.Users).To(Equal(expectedTeam.Users))
		})
	})
})
