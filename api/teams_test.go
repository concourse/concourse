package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/provider/providerfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng/dbngfakes"
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
		fakeTeam *dbngfakes.FakeTeam
	)

	BeforeEach(func() {
		fakeTeam = new(dbngfakes.FakeTeam)

	})

	Describe("GET /api/v1/teams", func() {
		var response *http.Response

		JustBeforeEach(func() {
			path := fmt.Sprintf("%s/api/v1/teams", server.URL)

			request, err := http.NewRequest("GET", path, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the database returns an error", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("some error")
				teamServerDB.GetTeamsReturns(nil, disaster)
			})

			It("returns 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the database returns teams", func() {
			BeforeEach(func() {
				teamServerDB.GetTeamsReturns([]db.SavedTeam{
					{
						ID: 5,
						Team: db.Team{
							Name: "avengers",
						},
					},
					{
						ID: 9,
						Team: db.Team{
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
						},
					},
					{
						ID: 22,
						Team: db.Team{
							Name: "predators",
							UAAAuth: &db.UAAAuth{
								ClientID:     "fake id",
								ClientSecret: "some secret",
								CFSpaces:     []string{"myspace"},
								AuthURL:      "http://auth.url",
								TokenURL:     "http://token.url",
								CFURL:        "http://api.url",
							},
						},
					},
					{
						ID: 23,
						Team: db.Team{
							Name: "cyborgs",
							GenericOAuth: &db.GenericOAuth{
								DisplayName:   "Cyborgs",
								ClientID:      "some random guid",
								ClientSecret:  "don't tell anyone",
								AuthURL:       "https://auth.url",
								AuthURLParams: map[string]string{"allow_humans": "false"},
								Scope:         "readonly",
								TokenURL:      "https://token.url",
							},
						},
					},
				}, nil)
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns the teams", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[
					{
						"id": 5,
						"name": "avengers"
					},
					{
						"id": 9,
						"name": "aliens"
					},
					{
						"id": 22,
						"name": "predators"
					},
					{
					  "id": 23,
						"name": "cyborgs"
				  }
				]`))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name", func() {
		var (
			response *http.Response

			atcTeam atc.Team
		)

		BeforeEach(func() {
			fakeTeam.IDReturns(5)
			fakeTeam.NameReturns("some-team")

			atcTeam = atc.Team{}
		})

		JustBeforeEach(func() {
			path := fmt.Sprintf("%s/api/v1/teams/some-team", server.URL)

			var err error
			request, err := http.NewRequest("PUT", path, jsonEncode(atcTeam))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		authorizedTeamTests := func() {
			Context("when the team has basic auth configured", func() {
				Context("when the basic auth is invalid", func() {
					Context("when only password is given", func() {
						BeforeEach(func() {
							atcTeam = atc.Team{
								BasicAuth: &atc.BasicAuth{
									BasicAuthPassword: "Batman",
								},
							}
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})

					Context("when only username is given", func() {
						BeforeEach(func() {
							atcTeam = atc.Team{
								BasicAuth: &atc.BasicAuth{
									BasicAuthUsername: "Ironman",
								},
							}
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})
				})

				Context("when the basic auth is valid", func() {
					BeforeEach(func() {
						atcTeam = atc.Team{
							BasicAuth: &atc.BasicAuth{
								BasicAuthPassword: "Kool",
								BasicAuthUsername: "Aid",
							},
						}
					})

					Context("when the team is found", func() {
						BeforeEach(func() {
							dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
						})

						It("updates basic auth", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
							Expect(fakeTeam.UpdateBasicAuthCallCount()).To(Equal(1))

							updatedBasicAuth := fakeTeam.UpdateBasicAuthArgsForCall(0)
							Expect(updatedBasicAuth).To(Equal(atcTeam.BasicAuth))
						})

						Context("when updating basic auth fails", func() {
							BeforeEach(func() {
								fakeTeam.UpdateBasicAuthReturns(errors.New("stop trying to make fetch happen"))
							})

							It("returns 500 Internal Server error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})
				})
			})

			Context("when the team has provider auth configured", func() {
				var (
					fakeProviderName    = "FakeProvider"
					fakeProviderFactory *providerfakes.FakeTeamProvider
				)
				BeforeEach(func() {
					fakeProviderFactory = new(providerfakes.FakeTeamProvider)
					provider.Register(fakeProviderName, fakeProviderFactory)
				})
				Context("when the provider is not found", func() {
					BeforeEach(func() {
						data := []byte(`{"mcdonalds": "fries"}`)
						atcTeam = atc.Team{
							Auth: map[string]*json.RawMessage{
								"fake-suraci": (*json.RawMessage)(&data),
							},
						}
					})

					It("returns a 400 Bad Request", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})
				Context("when the provider is found", func() {
					Context("when the auth is malformed", func() {
						BeforeEach(func() {
							data := []byte(`{"cold": "fries"}`)
							atcTeam = atc.Team{
								Auth: map[string]*json.RawMessage{
									fakeProviderName: (*json.RawMessage)(&data),
								},
							}
							fakeProviderFactory.UnmarshalConfigReturns(nil, errors.New("nope not this time"))
						})

						It("returns a 400 Bad Request", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})
					Context("when the auth is formatted correctly", func() {
						var fakeAuthConfig *providerfakes.FakeAuthConfig
						BeforeEach(func() {
							fakeAuthConfig = new(providerfakes.FakeAuthConfig)
							data := []byte(`{"mcdonalds":"fries"}`)
							atcTeam = atc.Team{
								Auth: map[string]*json.RawMessage{
									fakeProviderName: (*json.RawMessage)(&data),
								},
							}
							fakeProviderFactory.UnmarshalConfigReturns(fakeAuthConfig, nil)
						})
						Context("when the auth is invalid", func() {
							BeforeEach(func() {
								fakeAuthConfig.ValidateReturns(errors.New("nopeeee"))
							})
							It("returns a 400 Bad Request", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})
						})

						Context("when the auth is valid", func() {
							BeforeEach(func() {
								fakeAuthConfig.ValidateReturns(nil)
							})

							Context("when the team is found", func() {
								BeforeEach(func() {
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
							})
						})
					})
				})
			})
		}

		Context("when the requester team is authorized as an admin team", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("magic-admin-team", true, true)
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
					}))
				})

				Context("when it fails to create team", func() {
					BeforeEach(func() {
						dbTeamFactory.CreateTeamReturns(nil, errors.New("it is never going to happen"))
					})

					It("returns a 500 Internal Server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when the requester team is authorized as the team being set", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", false, true)
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
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns(atc.DefaultTeamName, true, true)
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

				It("returns 204 No Content", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNoContent))
				})

				It("deletes the team from the DB", func() {
					Expect(teamServerDB.DeleteTeamByNameCallCount()).To(Equal(1))
					//TODO delete the build events via a table drop rather
				})

				Context("when trying to delete the admin team", func() {
					BeforeEach(func() {
						teamName = atc.DefaultTeamName
						savedTeam = db.SavedTeam{
							ID: 2,
							Team: db.Team{
								Name:  teamName,
								Admin: true,
							},
						}
						teamDB.GetTeamReturns(savedTeam, true, nil)
						teamServerDB.GetTeamsReturns([]db.SavedTeam{savedTeam}, nil)
					})

					It("returns 403 Forbidden and backs off", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
						Expect(teamServerDB.DeleteTeamByNameCallCount()).To(Equal(0))
					})
				})

				Context("when there's a problem deleting the team", func() {
					BeforeEach(func() {
						teamServerDB.DeleteTeamByNameReturns(errors.New("disaster"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when team does not exist", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
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
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns(atc.DefaultTeamName, false, true)
			})

			It("returns 403 forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when the requester's team cannot be determined", func() {
			JustBeforeEach(func() {
				path := fmt.Sprintf("%s/api/v1/teams/%s", server.URL, teamName)

				var err error
				request, err = http.NewRequest("DELETE", path, jsonEncode(team))
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("", false, false)
			})

			It("returns 500 internal server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
