package db_test

import (
	"database/sql"
	"fmt"
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
	var teamDBFactory db.TeamDBFactory

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus)
		database = db.NewSQL(dbConn, bus)

		database.DeleteTeamByName(atc.DefaultTeamName)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetTeams", func() {
		It("Gets all saved teams", func() {
			team1 := db.Team{
				Name: "avengers",
			}
			savedTeam1, err := database.CreateTeam(team1)
			Expect(err).NotTo(HaveOccurred())

			team2 := db.Team{
				Name: "aliens",
				BasicAuth: &db.BasicAuth{
					BasicAuthUsername: "fake user",
					BasicAuthPassword: "no, bad",
				},
				GitHubAuth: &db.GitHubAuth{
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
			savedTeam2, err := database.CreateTeam(team2)
			Expect(err).NotTo(HaveOccurred())

			team3 := db.Team{
				Name: "predators",
				UAAAuth: &db.UAAAuth{
					ClientID:     "fake id",
					ClientSecret: "some secret",
					CFSpaces:     []string{"myspace"},
					AuthURL:      "http://auth.url",
					TokenURL:     "http://token.url",
					CFURL:        "http://api.url",
				},
			}
			savedTeam3, err := database.CreateTeam(team3)
			Expect(err).NotTo(HaveOccurred())

			team4 := db.Team{
				Name: "cyborgs",
				GenericOAuth: &db.GenericOAuth{
					DisplayName:   "Cyborgs",
					ClientID:      "some random guid",
					ClientSecret:  "don't tell anyone",
					AuthURL:       "https://auth.url",
					AuthURLParams: map[string]string{"allow_humans": "false"},
				},
			}
			savedTeam4, err := database.CreateTeam(team4)
			Expect(err).NotTo(HaveOccurred())

			actualTeams, err := database.GetTeams()
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTeams).To(ConsistOf(savedTeam1, savedTeam2, savedTeam3, savedTeam4))
		})
	})

	Describe("CreateDefaultTeamIfNotExists", func() {
		It("creates the default team", func() {
			err := database.CreateDefaultTeamIfNotExists()
			Expect(err).NotTo(HaveOccurred())

			var count sql.NullInt64
			dbConn.QueryRow(fmt.Sprintf(`select count(1) from teams where name = '%s'`, atc.DefaultTeamName)).Scan(&count)

			Expect(count.Valid).To(BeTrue())
			Expect(count.Int64).To(Equal(int64(1)))

			team, _, err := teamDBFactory.GetTeamDB(atc.DefaultTeamName).GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(team.Admin).To(BeTrue())
		})

		Context("when the default team already exists", func() {
			BeforeEach(func() {
				defaultTeam := db.Team{
					Name: atc.DefaultTeamName,
				}
				_, err := database.CreateTeam(defaultTeam)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not duplicate the default team", func() {
				err := database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())

				var count sql.NullInt64
				dbConn.QueryRow(fmt.Sprintf(`select count(1) from teams where name = '%s'`, atc.DefaultTeamName)).Scan(&count)

				Expect(count.Valid).To(BeTrue())
				Expect(count.Int64).To(Equal(int64(1)))
			})

			It("sets admin permissions on that team", func() {
				err := database.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())

				var admin bool
				dbConn.QueryRow(fmt.Sprintf(`select admin from teams where name = '%s'`, atc.DefaultTeamName)).Scan(&admin)

				Expect(admin).To(BeTrue())
			})
		})
	})

	Describe("CreateTeam", func() {
		It("saves a team to the db", func() {
			expectedTeam := db.Team{
				Name: "AvengerS",
			}
			expectedSavedTeam, err := database.CreateTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team.Admin).To(Equal(expectedTeam.Admin))
			Expect(expectedSavedTeam.Team.BasicAuth).To(Equal(expectedTeam.BasicAuth))
			Expect(expectedSavedTeam.Team.GitHubAuth).To(Equal(expectedTeam.GitHubAuth))
			Expect(expectedSavedTeam.Team.UAAAuth).To(Equal(expectedTeam.UAAAuth))
			Expect(expectedSavedTeam.Team.GenericOAuth).To(Equal(expectedTeam.GenericOAuth))
			Expect(expectedSavedTeam.Team.Name).To(Equal("AvengerS"))

			savedTeam, found, err := teamDBFactory.GetTeamDB("aVengers").GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))
		})

		It("saves a team to the db with basic auth", func() {
			expectedTeam := db.Team{
				Name: "avengers",
				BasicAuth: &db.BasicAuth{
					BasicAuthUsername: "fake user",
					BasicAuthPassword: "no, bad",
				},
			}
			expectedSavedTeam, err := database.CreateTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team.Name).To(Equal(expectedTeam.Name))

			savedTeam, found, err := teamDBFactory.GetTeamDB("avengers").GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.BasicAuth.BasicAuthUsername).To(Equal(expectedTeam.BasicAuth.BasicAuthUsername))
			Expect(bcrypt.CompareHashAndPassword([]byte(savedTeam.BasicAuth.BasicAuthPassword),
				[]byte(expectedTeam.BasicAuth.BasicAuthPassword))).To(BeNil())
		})

		It("saves a team to the db with GitHub auth", func() {
			expectedTeam := db.Team{
				Name: "avengers",
				GitHubAuth: &db.GitHubAuth{
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
			expectedSavedTeam, err := database.CreateTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team).To(Equal(expectedTeam))

			savedTeam, found, err := teamDBFactory.GetTeamDB("avengers").GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.GitHubAuth).To(Equal(expectedTeam.GitHubAuth))
		})

		It("saves a team to the db with CF auth", func() {
			expectedTeam := db.Team{
				Name: "avengers",
				UAAAuth: &db.UAAAuth{
					ClientID:     "fake id",
					ClientSecret: "some secret",
					CFSpaces:     []string{"myspace"},
					AuthURL:      "http://auth.url",
					TokenURL:     "http://token.url",
					CFURL:        "http://api.url",
				},
			}
			expectedSavedTeam, err := database.CreateTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team).To(Equal(expectedTeam))

			savedTeam, found, err := teamDBFactory.GetTeamDB("avengers").GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.UAAAuth).To(Equal(expectedTeam.UAAAuth))
		})

		It("saves a team to the db with Generic OAuth auth", func() {
			expectedTeam := db.Team{
				Name: "cyborgs",
				GenericOAuth: &db.GenericOAuth{
					DisplayName:   "Cyborgs",
					ClientID:      "some random guid",
					ClientSecret:  "don't tell anyone",
					AuthURL:       "https://auth.url",
					AuthURLParams: map[string]string{"allow_humans": "false"},
				},
			}
			expectedSavedTeam, err := database.CreateTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team).To(Equal(expectedTeam))

			savedTeam, found, err := teamDBFactory.GetTeamDB("cyborgs").GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))

			Expect(savedTeam.GenericOAuth).To(Equal(expectedTeam.GenericOAuth))
		})
	})

	Describe("DeleteTeamByName", func() {
		Context("when the team exists", func() {
			BeforeEach(func() {
				_, err := database.CreateTeam(db.Team{
					Name: "team-name",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the team when the name matches exactly", func() {
				err := database.DeleteTeamByName("team-name")
				Expect(err).NotTo(HaveOccurred())

				var count sql.NullInt64
				dbConn.QueryRow(`select count(1) from teams where name = 'team-name'`).Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count.Valid).To(BeTrue())
				Expect(count.Int64).To(Equal(int64(0)))
			})

			It("deletes the team when the name matches case-insensitively", func() {
				err := database.DeleteTeamByName("TEAM-name")
				Expect(err).NotTo(HaveOccurred())

				var count sql.NullInt64
				dbConn.QueryRow(`select count(1) from teams where name = 'team-name'`).Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count.Valid).To(BeTrue())
				Expect(count.Int64).To(Equal(int64(0)))
			})
		})
	})
})
