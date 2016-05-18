package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func jsonEncode(object interface{}) *bytes.Buffer {
	reqPayload, err := json.Marshal(object)
	Expect(err).NotTo(HaveOccurred())

	return bytes.NewBuffer(reqPayload)
}

func atcDBTeamEquality(atcTeam atc.Team, dbTeam db.Team) {
	Expect(dbTeam.Name).To(Equal(atcTeam.Name))
	Expect(dbTeam.BasicAuth.BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
	Expect(dbTeam.BasicAuth.BasicAuthPassword).To(Equal(atcTeam.BasicAuth.BasicAuthPassword))
	Expect(dbTeam.GitHubAuth.ClientID).To(Equal(atcTeam.GitHubAuth.ClientID))
	Expect(dbTeam.GitHubAuth.ClientSecret).To(Equal(atcTeam.GitHubAuth.ClientSecret))
	Expect(dbTeam.GitHubAuth.Organizations).To(Equal(atcTeam.GitHubAuth.Organizations))
	Expect(dbTeam.GitHubAuth.Users).To(Equal(atcTeam.GitHubAuth.Users))
	Expect(len(dbTeam.GitHubAuth.Teams)).To(Equal(len(atcTeam.GitHubAuth.Teams)))
	for i, atcGitHubTeam := range atcTeam.GitHubAuth.Teams {
		dbGitHubTeam := dbTeam.GitHubAuth.Teams[i]
		Expect(dbGitHubTeam.OrganizationName).To(Equal(atcGitHubTeam.OrganizationName))
		Expect(dbGitHubTeam.TeamName).To(Equal(atcGitHubTeam.TeamName))
	}
}

var _ = Describe("Auth API", func() {
	Describe("PUT /api/v1/teams/:team_name", func() {
		var request *http.Request
		var response *http.Response

		var team atc.Team
		var savedTeam db.SavedTeam
		var teamName string

		BeforeEach(func() {
			teamName = "team venture"

			team = atc.Team{}
			savedTeam = db.SavedTeam{
				ID: 2,
				Team: db.Team{
					Name: teamName,
				},
			}
		})

		JustBeforeEach(func() {
			path := fmt.Sprintf("%s/api/v1/teams/%s", server.URL, teamName)

			var err error
			request, err = http.NewRequest("PUT", path, jsonEncode(team))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the requester is authenticated for the right team", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns(atc.DefaultTeamName, 1, true, true)
			})

			Describe("request body validation", func() {
				Describe("basic authenticaiton", func() {
					Context("BasicAuthUsername not filled in", func() {
						BeforeEach(func() {
							team = atc.Team{
								BasicAuth: atc.BasicAuth{
									BasicAuthPassword: "Batman",
								},
							}
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})

					Context("BasicAuthPassword not filled in", func() {
						BeforeEach(func() {
							team = atc.Team{
								BasicAuth: atc.BasicAuth{
									BasicAuthUsername: "Hank Venture",
								},
							}
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})
				})

				Describe("GitHub authenticaiton", func() {
					Context("ClientID not filled in", func() {
						BeforeEach(func() {
							team = atc.Team{
								GitHubAuth: atc.GitHubAuth{
									ClientSecret: "09262-8765-001",
								},
							}
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})

					Context("ClientSecret not filled in", func() {
						BeforeEach(func() {
							team = atc.Team{
								GitHubAuth: atc.GitHubAuth{
									ClientID: "Brock Samson",
								},
							}
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})

					Context("require at least one org, org/team, or username", func() {
						Context("when all are missing", func() {
							BeforeEach(func() {
								team = atc.Team{
									GitHubAuth: atc.GitHubAuth{
										ClientID:     "Brock Samson",
										ClientSecret: "09262-8765-001",
									},
								}
							})

							It("returns a 400 Bad Request", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})
						})

						Context("when passed organizations", func() {
							BeforeEach(func() {
								team = atc.Team{
									GitHubAuth: atc.GitHubAuth{
										ClientID:      "Brock Samson",
										ClientSecret:  "09262-8765-001",
										Organizations: []string{"United States Armed Forces", "Office of Secret Intelligence", "Team Venture", "S.P.H.I.N.X."},
									},
								}
							})

							It("does not error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})
						})

						Context("when passed a team", func() {
							BeforeEach(func() {
								team = atc.Team{
									GitHubAuth: atc.GitHubAuth{
										ClientID:     "Brock Samson",
										ClientSecret: "09262-8765-001",
										Teams: []atc.GitHubTeam{
											{
												OrganizationName: "Office of Secret Intelligence",
												TeamName:         "Secret Agent",
											},
										},
									},
								}
							})

							It("does not error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})
						})

						Context("when passed users", func() {
							BeforeEach(func() {
								team = atc.Team{
									GitHubAuth: atc.GitHubAuth{
										ClientID:     "S.P.H.I.N.X.",
										ClientSecret: "SPHINX Rising",
										Users: []string{
											"Col. Hunter Gathers",
											"Holy Diver/Shore Leave",
											"Mile High/Sky Pilot",
											"Brock Samson",
											"Unnamed German plastic surgeon",
										},
									},
								}
							})

							It("does not error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})
						})
					})
				})
			})

			Context("when there's a problem finding teams", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(db.SavedTeam{}, false, errors.New("a dingo ate my baby!"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when team exists", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(savedTeam, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the updated team", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{
					"id": 2,
					"name": "team venture"
				}`))

					Expect(teamsDB.CreateTeamCallCount()).To(Equal(0))
				})

				Context("updating authentication", func() {
					var basicAuth atc.BasicAuth
					var gitHubAuth atc.GitHubAuth

					BeforeEach(func() {
						basicAuth = atc.BasicAuth{
							BasicAuthUsername: "Dean Venture",
							BasicAuthPassword: "Giant Boy Detective",
						}

						gitHubAuth = atc.GitHubAuth{
							ClientID:     "Dean Venture",
							ClientSecret: "Giant Boy Detective",
							Users:        []string{"Dean Venture"},
						}
					})

					Context("when passed basic auth credentials", func() {
						BeforeEach(func() {
							teamDB.UpdateTeamBasicAuthStub = func(submittedTeam db.Team) (db.SavedTeam, error) {
								team.Name = teamName
								atcDBTeamEquality(team, submittedTeam)
								savedTeam.Team = submittedTeam
								return savedTeam, nil
							}

							team.BasicAuth = basicAuth
						})

						It("updates the basic auth for that team", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
							Expect(teamDB.UpdateTeamBasicAuthCallCount()).To(Equal(1))
						})
					})

					Context("when passed GitHub auth credentials", func() {
						BeforeEach(func() {
							teamDB.UpdateTeamGitHubAuthStub = func(submittedTeam db.Team) (db.SavedTeam, error) {
								team.Name = teamName
								atcDBTeamEquality(team, submittedTeam)
								savedTeam.Team = submittedTeam
								return savedTeam, nil
							}

							team.GitHubAuth = gitHubAuth
						})

						It("updates the GitHub auth for that team", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
							Expect(teamDB.UpdateTeamGitHubAuthCallCount()).To(Equal(1))
						})
					})
				})
			})

			Context("when team does not exist", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)

					teamsDB.CreateTeamStub = func(submittedTeam db.Team) (db.SavedTeam, error) {
						team.Name = teamName
						atcDBTeamEquality(team, submittedTeam)
						return savedTeam, nil
					}
				})

				It("returns 201 Created", func() {
					Expect(response.StatusCode).To(Equal(http.StatusCreated))
				})

				It("returns the new team", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{
					"id": 2,
					"name": "team venture"
				}`))

					Expect(teamsDB.CreateTeamCallCount()).To(Equal(1))
				})

				Context("when there's a problem saving teams", func() {
					BeforeEach(func() {
						teamsDB.CreateTeamReturns(db.SavedTeam{}, errors.New("Do not be too hasty in entering that room. I had Taco Bell for lunch!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("with authentication", func() {
					var basicAuth atc.BasicAuth
					var gitHubAuth atc.GitHubAuth

					BeforeEach(func() {
						basicAuth = atc.BasicAuth{
							BasicAuthUsername: "Dean Venture",
							BasicAuthPassword: "Giant Boy Detective",
						}

						gitHubAuth = atc.GitHubAuth{
							ClientID:     "Dean Venture",
							ClientSecret: "Giant Boy Detective",
							Users:        []string{"Dean Venture"},
						}
					})

					Context("when passed basic auth credentials", func() {
						BeforeEach(func() {
							team.BasicAuth = basicAuth
						})

						It("updates the basic auth for that team", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(teamsDB.CreateTeamCallCount()).To(Equal(1))
						})
					})

					Context("when passed GitHub auth credentials", func() {
						BeforeEach(func() {
							team.GitHubAuth = gitHubAuth
						})

						It("updates the GitHub auth for that team", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(teamsDB.CreateTeamCallCount()).To(Equal(1))
						})
					})
				})
			})
		})

		Context("when the requester belongs to a non-admin team", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("non-admin-team", 5, false, true)
			})

			It("returns 403 forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when the requester's team cannot be determined", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("", 0, false, false)
			})

			It("returns 500 internal server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
