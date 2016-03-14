package db_test

import (
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("SQL DB Teams", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database db.DB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(dbConn, bus)

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
			BeforeEach(func() {
				defaultTeam := db.Team{
					Name: atc.DefaultTeamName,
				}
				_, err := database.SaveTeam(defaultTeam)
				Expect(err).NotTo(HaveOccurred())
			})

			It("it does not get duplicated", func() {
				err := database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())

				team, found, err := database.GetTeamByName(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(team.Name).To(Equal(atc.DefaultTeamName))
				Expect(team.Admin).To(BeTrue())
			})

			Context("and it does not have admin permissions", func() {
				It("it sets admin permissions on that team", func() {
					err := database.CreateDefaultTeamIfNotExists()
					Expect(err).NotTo(HaveOccurred())

					team, _, err := database.GetTeamByName(atc.DefaultTeamName)
					Expect(err).NotTo(HaveOccurred())
					Expect(team.Admin).To(BeTrue())
				})
			})
		})

		Describe("it does not exist", func() {
			It("it gets created", func() {
				err := database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())
				team, found, err := database.GetTeamByName(atc.DefaultTeamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(team.Name).To(Equal(atc.DefaultTeamName))
				Expect(team.Admin).To(BeTrue())
			})
		})
	})

	Describe("Get Team by name", func() {
		Context("when team does not exist", func() {
			It("returns false with no error", func() {
				_, found, err := database.GetTeamByName("Venture")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("UpdateTeam", func() {
		var basicAuthTeam, gitHubAuthTeam db.Team

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

			gitHubAuthTeam = db.Team{
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
			savedTeam, err := database.UpdateTeamGitHubAuth(gitHubAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			Expect(savedTeam.ClientID).To(Equal(gitHubAuthTeam.ClientID))
			Expect(savedTeam.ClientSecret).To(Equal(gitHubAuthTeam.ClientSecret))
			Expect(savedTeam.Organizations).To(Equal(gitHubAuthTeam.Organizations))
			Expect(savedTeam.Teams).To(Equal(gitHubAuthTeam.Teams))
			Expect(savedTeam.Users).To(Equal(gitHubAuthTeam.Users))
		})

		It("saves gitHub auth team info without over writing the basic auth", func() {
			_, err := database.UpdateTeamGitHubAuth(gitHubAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			savedTeam, err := database.UpdateTeamBasicAuth(basicAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			Expect(savedTeam.ClientID).To(Equal(gitHubAuthTeam.ClientID))
			Expect(savedTeam.ClientSecret).To(Equal(gitHubAuthTeam.ClientSecret))
			Expect(savedTeam.Organizations).To(Equal(gitHubAuthTeam.Organizations))
			Expect(savedTeam.Teams).To(Equal(gitHubAuthTeam.Teams))
			Expect(savedTeam.Users).To(Equal(gitHubAuthTeam.Users))
		})

		Context("required GitHub auth elements are not present", func() {
			BeforeEach(func() {
				_, err := database.UpdateTeamGitHubAuth(gitHubAuthTeam)
				Expect(err).NotTo(HaveOccurred())
			})

			It("nulls basic auth when has a blank clientSecret", func() {
				gitHubAuthTeam.ClientSecret = ""
				savedTeam, err := database.UpdateTeamGitHubAuth(gitHubAuthTeam)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.ClientID).To(BeEmpty())
				Expect(savedTeam.ClientSecret).To(BeEmpty())
				Expect(savedTeam.Organizations).To(BeEmpty())
				Expect(savedTeam.Teams).To(BeEmpty())
				Expect(savedTeam.Users).To(BeEmpty())
			})

			It("nulls basic auth when has a blank clientID", func() {
				gitHubAuthTeam.ClientID = ""
				savedTeam, err := database.UpdateTeamGitHubAuth(gitHubAuthTeam)
				Expect(err).NotTo(HaveOccurred())

				Expect(savedTeam.ClientID).To(BeEmpty())
				Expect(savedTeam.ClientSecret).To(BeEmpty())
				Expect(savedTeam.Organizations).To(BeEmpty())
				Expect(savedTeam.Teams).To(BeEmpty())
				Expect(savedTeam.Users).To(BeEmpty())
			})
		})

		It("saves basic auth team info without over writing the GitHub auth", func() {
			_, err := database.UpdateTeamBasicAuth(basicAuthTeam)
			Expect(err).NotTo(HaveOccurred())

			savedTeam, err := database.UpdateTeamGitHubAuth(gitHubAuthTeam)
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

			savedTeam, found, err := database.GetTeamByName("avengers")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
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

			savedTeam, found, err := database.GetTeamByName("avengers")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.BasicAuthUsername).To(Equal(expectedTeam.BasicAuthUsername))
			Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuthPassword),
				[]byte(expectedTeam.BasicAuthPassword))).To(BeNil())
		})

		It("saves a team to the db with GitHub auth", func() {
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

			savedTeam, found, err := database.GetTeamByName("avengers")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.ClientID).To(Equal(expectedTeam.ClientID))
			Expect(savedTeam.ClientSecret).To(Equal(expectedTeam.ClientSecret))
			Expect(savedTeam.Organizations).To(Equal(expectedTeam.Organizations))
			Expect(savedTeam.Teams).To(Equal(expectedTeam.Teams))
			Expect(savedTeam.Users).To(Equal(expectedTeam.Users))
		})
	})
})
