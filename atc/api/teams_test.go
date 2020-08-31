package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func jsonEncode(object interface{}) *bytes.Buffer {
	reqPayload, err := json.Marshal(object)
	Expect(err).NotTo(HaveOccurred())

	return bytes.NewBuffer(reqPayload)
}

var _ = Describe("Teams API", func() {
	var (
		fakeTeam *dbfakes.FakeTeam
	)

	BeforeEach(func() {
		fakeTeam = new(dbfakes.FakeTeam)
	})

	Describe("GET /api/v1/teams", func() {
		var (
			response      *http.Response
			fakeTeamOne   *dbfakes.FakeTeam
			fakeTeamTwo   *dbfakes.FakeTeam
			fakeTeamThree *dbfakes.FakeTeam
			teamNames     []string
		)

		JustBeforeEach(func() {
			path := fmt.Sprintf("%s/api/v1/teams", server.URL)

			request, err := http.NewRequest("GET", path, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeTeamOne = new(dbfakes.FakeTeam)
			fakeTeamTwo = new(dbfakes.FakeTeam)
			fakeTeamThree = new(dbfakes.FakeTeam)

			teamNames = []string{"avengers", "aliens", "predators"}

			fakeTeamOne.IDReturns(5)
			fakeTeamOne.NameReturns(teamNames[0])
			fakeTeamOne.AuthReturns(atc.TeamAuth{
				"owner": map[string][]string{
					"groups": []string{}, "users": []string{"local:username"},
				},
			})

			fakeTeamTwo.IDReturns(9)
			fakeTeamTwo.NameReturns(teamNames[1])
			fakeTeamTwo.AuthReturns(atc.TeamAuth{
				"owner": map[string][]string{
					"groups": []string{}, "users": []string{"local:username"},
				},
			})

			fakeTeamThree.IDReturns(22)
			fakeTeamThree.NameReturns(teamNames[2])
			fakeTeamThree.AuthReturns(atc.TeamAuth{
				"owner": map[string][]string{
					"groups": []string{}, "users": []string{"local:username"},
				},
			})

			fakeAccess.IsAuthorizedReturnsOnCall(0, true)
			fakeAccess.IsAuthorizedReturnsOnCall(1, false)
			fakeAccess.IsAuthorizedReturnsOnCall(2, true)
		})

		Context("when the database call succeeds", func() {
			BeforeEach(func() {
				dbTeamFactory.GetTeamsReturns([]db.Team{fakeTeamOne, fakeTeamTwo, fakeTeamThree}, nil)
			})

			It("should return the teams the user is authorized for", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[
 					{
 						"id": 5,
 						"name": "avengers",
						"auth": { "owner":{"users":["local:username"],"groups":[]}}
 					},
 					{
 						"id": 22,
 						"name": "predators",
						"auth": { "owner":{"users":["local:username"],"groups":[]}}
 					}
 				]`))
			})
		})

		Context("when the database call returns an error", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("some error")
				dbTeamFactory.GetTeamsReturns(nil, disaster)
			})

			It("returns 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name", func() {
		var response *http.Response
		var fakeTeam *dbfakes.FakeTeam

		BeforeEach(func() {
			fakeTeam = new(dbfakes.FakeTeam)
			fakeTeam.IDReturns(1)
			fakeTeam.NameReturns("a-team")
			fakeTeam.AuthReturns(atc.TeamAuth{
				"owner": map[string][]string{
					"groups": {}, "users": {"local:username"},
				},
			})
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/teams/a-team", server.URL), nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated and not admin", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
				fakeAccess.IsAdminReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when not authenticated to specified team, but have admin authority", func() {
			BeforeEach(func() {
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)
				fakeAccess.IsAuthorizedReturns(false)
			})

			It("returns 200 ok", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				expectedHeaderEntries := map[string]string{
					"Content-Type": "application/json",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
			})

			It("returns a team JSON", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`
				{
					"id": 1,
					"name": "a-team",
					"auth": {
						"owner": {
							"groups": [],
							"users": [
								"local:username"
							]
						}
					}
				}`))
			})
		})

		Context("when authenticated to specified team", func() {
			BeforeEach(func() {
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(false)
				fakeAccess.IsAuthorizedReturns(true)
			})

			It("returns 200 ok", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				expectedHeaderEntries := map[string]string{
					"Content-Type": "application/json",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
			})

			It("returns a team JSON", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`
				{
					"id": 1,
					"name": "a-team",
					"auth": {
						"owner": {
							"groups": [],
							"users": [
								"local:username"
							]
						}
					}
				}`))
			})
		})

		Context("when authenticated as another team", func() {
			BeforeEach(func() {
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				fakeAccess.IsAuthenticatedReturns(true)
			})

			It("return 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})
	Describe("PUT /api/v1/teams/:team_name", func() {
		var (
			response *http.Response
			teamAuth atc.TeamAuth
			atcTeam  atc.Team
			path     string
		)

		BeforeEach(func() {
			fakeTeam.IDReturns(5)
			fakeTeam.NameReturns("some-team")

			teamAuth = atc.TeamAuth{
				"owner": map[string][]string{
					"groups": {}, "users": {"local:username"},
				},
			}
			atcTeam = atc.Team{Auth: teamAuth}
			path = fmt.Sprintf("%s/api/v1/teams/some-team", server.URL)
		})

		JustBeforeEach(func() {
			var err error
			request, err := http.NewRequest("PUT", path, jsonEncode(atcTeam))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		authorizedTeamTests := func() {
			Context("when the team exists", func() {
				BeforeEach(func() {
					atcTeam = atc.Team{
						Auth: atc.TeamAuth{
							"owner": map[string][]string{
								"users": []string{"local:username"},
							},
						},
					}
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("updates provider auth", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					Expect(fakeTeam.UpdateProviderAuthCallCount()).To(Equal(1))

					updatedProviderAuth := fakeTeam.UpdateProviderAuthArgsForCall(0)
					Expect(updatedProviderAuth).To(Equal(atcTeam.Auth))
				})

				Context("when updating provider auth fails", func() {
					BeforeEach(func() {
						fakeTeam.UpdateProviderAuthReturns(errors.New("stop trying to make fetch happen"))
					})

					It("returns 500 Internal Server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
				Context("when provider auth is empty", func() {
					BeforeEach(func() {
						atcTeam = atc.Team{}
						dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
					})

					It("does not update provider auth", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(fakeTeam.UpdateProviderAuthCallCount()).To(Equal(0))
					})
				})

				Context("when provider auth is invalid", func() {
					BeforeEach(func() {
						atcTeam = atc.Team{
							Auth: atc.TeamAuth{
								"owner": {
									//"users": []string{},
									//"groups": []string{},
								},
							},
						}
						dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
					})

					It("does not update provider auth", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(fakeTeam.UpdateProviderAuthCallCount()).To(Equal(0))
					})
				})
			})
		}

		Context("when the requester team is authorized as an admin team", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)
			})

			authorizedTeamTests()

			Context("when the team is not found", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, nil)
					dbTeamFactory.CreateTeamReturns(fakeTeam, nil)
				})

				It("creates the team", func() {
					Expect(response.StatusCode).To(Equal(http.StatusCreated))
					Expect(dbTeamFactory.CreateTeamCallCount()).To(Equal(1))

					createdTeam := dbTeamFactory.CreateTeamArgsForCall(0)
					Expect(createdTeam).To(Equal(atc.Team{
						Name: "some-team",
						Auth: teamAuth,
					}))
				})

				It("delete the teams in cache", func() {
					Expect(dbTeamFactory.NotifyCacherCallCount()).To(Equal(1))
				})

				Context("when it fails to create team", func() {
					BeforeEach(func() {
						dbTeamFactory.CreateTeamReturns(nil, errors.New("it is never going to happen"))
					})

					It("returns a 500 Internal Server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})

					It("does not delete the teams in cache", func() {
						Expect(dbTeamFactory.NotifyCacherCallCount()).To(Equal(0))
					})
				})

				Context("when the team's name is an invalid identifier", func() {
					BeforeEach(func() {
						path = fmt.Sprintf("%s/api/v1/teams/_some-team", server.URL)
						fakeTeam.NameReturns("_some-team")
					})

					It("returns a warning in the response body", func() {
						Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"warnings": [
										{
											"type": "invalid_identifier",
											"message": "team: '_some-team' is not a valid identifier: must start with a lowercase letter"
										}
									],
									"team": {
										"id": 5,
										"name": "_some-team"
									}
								}`))
					})
				})
			})
		})

		Context("when the requester team is authorized as the team being set", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			authorizedTeamTests()

			Context("when the team is not found", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, nil)
					dbTeamFactory.CreateTeamReturns(fakeTeam, nil)
				})

				It("does not create the team", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					Expect(dbTeamFactory.CreateTeamCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("DELETE /api/v1/teams/:team_name", func() {
		var request *http.Request
		var response *http.Response

		var teamName string

		BeforeEach(func() {
			teamName = "team venture"

			fakeTeam.IDReturns(2)
			fakeTeam.NameReturns(teamName)
		})

		Context("when the requester is authenticated for some admin team", func() {
			JustBeforeEach(func() {
				path := fmt.Sprintf("%s/api/v1/teams/%s", server.URL, teamName)

				var err error
				request, err = http.NewRequest("DELETE", path, nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)
			})

			Context("when there's a problem finding teams", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, errors.New("a dingo ate my baby!"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when team exists", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("returns 204 No Content", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNoContent))
				})

				It("receives the correct team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal(teamName))
				})
				It("deletes the team from the DB", func() {
					Expect(fakeTeam.DeleteCallCount()).To(Equal(1))
					//TODO delete the build events via a table drop rather
				})

				Context("when trying to delete the admin team", func() {
					BeforeEach(func() {
						teamName = atc.DefaultTeamName
						fakeTeam.AdminReturns(true)
						dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
						dbTeamFactory.GetTeamsReturns([]db.Team{fakeTeam}, nil)
					})

					It("returns 403 Forbidden and backs off", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
						Expect(fakeTeam.DeleteCallCount()).To(Equal(0))
					})
				})

				Context("when there's a problem deleting the team", func() {
					BeforeEach(func() {
						fakeTeam.DeleteReturns(errors.New("disaster"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when team does not exist", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, nil)
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when the requester belongs to a non-admin team", func() {
			JustBeforeEach(func() {
				path := fmt.Sprintf("%s/api/v1/teams/%s", server.URL, "non-admin-team")

				var err error
				request, err = http.NewRequest("DELETE", path, nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())

			})

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(false)
			})

			It("returns 403 forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/rename", func() {
		var response *http.Response
		var requestBody string
		var teamName string

		JustBeforeEach(func() {
			request, err := http.NewRequest(
				"PUT",
				server.URL+"/api/v1/teams/"+teamName+"/rename",
				bytes.NewBufferString(requestBody),
			)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			requestBody = `{"name":"some-new-name"}`
			fakeTeam.IDReturns(2)
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})
			Context("when requester belongs to an admin team", func() {
				BeforeEach(func() {
					teamName = "a-team"
					fakeTeam.NameReturns(teamName)
					fakeAccess.IsAdminReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs teamDB with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("renames the team to the name provided", func() {
					Expect(fakeTeam.RenameCallCount()).To(Equal(1))
					Expect(fakeTeam.RenameArgsForCall(0)).To(Equal("some-new-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when a warning occurs", func() {

					BeforeEach(func() {
						requestBody = `{"name":"_some-new-name"}`
					})

					It("returns a warning in the response body", func() {
						Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
							{
								"warnings": [
								{
									"type": "invalid_identifier",
									"message": "team: '_some-new-name' is not a valid identifier: must start with a lowercase letter"
								}
								]
							}`))
					})
				})
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					teamName = "a-team"
					fakeTeam.NameReturns(teamName)
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs teamDB with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("renames the team to the name provided", func() {
					Expect(fakeTeam.RenameCallCount()).To(Equal(1))
					Expect(fakeTeam.RenameArgsForCall(0)).To(Equal("some-new-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					teamName = "a-team"
					fakeTeam.NameReturns(teamName)
					fakeAccess.IsAuthorizedReturns(false)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("returns 403 Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					Expect(fakeTeam.RenameCallCount()).To(Equal(0))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				Expect(fakeTeam.RenameCallCount()).To(Equal(0))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/builds", func() {
		var (
			response    *http.Response
			queryParams string
			teamName    string
		)

		BeforeEach(func() {
			teamName = "some-team"
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/" + teamName + "/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				Expect(fakeTeam.BuildsCallCount()).To(Equal(0))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			Context("when no params are passed", func() {
				It("does not set defaults for since and until", func() {
					Expect(fakeTeam.BuildsCallCount()).To(Equal(1))

					page := fakeTeam.BuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						Limit: 100,
					}))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?from=2&to=3&limit=8"
				})

				It("passes them through", func() {
					Expect(fakeTeam.BuildsCallCount()).To(Equal(1))

					page := fakeTeam.BuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						From:  db.NewIntPtr(2),
						To:    db.NewIntPtr(3),
						Limit: 8,
					}))
				})
			})

			Context("when getting the builds succeeds", func() {
				var returnedBuilds []db.Build

				BeforeEach(func() {
					queryParams = "?since=5&limit=2"

					build1 := new(dbfakes.FakeBuild)
					build1.IDReturns(4)
					build1.NameReturns("2")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.TeamNameReturns("some-team")
					build1.StatusReturns(db.BuildStatusStarted)
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(2)
					build2.NameReturns("1")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.TeamNameReturns("some-team")
					build2.StatusReturns(db.BuildStatusSucceeded)
					build2.StartTimeReturns(time.Unix(101, 0))
					build2.EndTimeReturns(time.Unix(200, 0))

					returnedBuilds = []db.Build{build1, build2}
					fakeTeam.BuildsReturns(returnedBuilds, db.Pagination{}, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				It("returns the builds", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
					{
						"id": 4,
						"name": "2",
						"job_name": "some-job",
						"status": "started",
						"api_url": "/api/v1/builds/4",
						"pipeline_name":"some-pipeline",
						"team_name": "some-team",
						"start_time": 1,
						"end_time": 100
					},
					{
						"id": 2,
						"name": "1",
						"job_name": "some-job",
						"status": "succeeded",
						"api_url": "/api/v1/builds/2",
						"pipeline_name": "some-pipeline",
						"team_name": "some-team",
						"start_time": 101,
						"end_time": 200
					}
				]`))
				})

				Context("when next/previous pages are available", func() {
					BeforeEach(func() {
						fakeTeam.BuildsReturns(returnedBuilds, db.Pagination{
							Newer: &db.Page{From: db.NewIntPtr(4), Limit: 2},
							Older: &db.Page{To: db.NewIntPtr(2), Limit: 2},
						}, nil)
					})

					It("returns Link headers per rfc5988", func() {
						Expect(response.Header["Link"]).To(ConsistOf([]string{
							fmt.Sprintf(`<%s/api/v1/teams/some-team/builds?from=4&limit=2>; rel="previous"`, externalURL),
							fmt.Sprintf(`<%s/api/v1/teams/some-team/builds?to=2&limit=2>; rel="next"`, externalURL),
						}))
					})
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					fakeTeam.BuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})
})
