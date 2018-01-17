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
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/provider/providerfakes"
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
				dbTeamFactory.GetTeamsReturns(nil, disaster)
			})

			It("returns 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the database returns teams", func() {
			var (
				fakeTeamOne   *dbfakes.FakeTeam
				fakeTeamTwo   *dbfakes.FakeTeam
				fakeTeamThree *dbfakes.FakeTeam
			)
			BeforeEach(func() {
				fakeTeamOne = new(dbfakes.FakeTeam)
				fakeTeamTwo = new(dbfakes.FakeTeam)
				fakeTeamThree = new(dbfakes.FakeTeam)

				fakeTeamOne.IDReturns(5)
				fakeTeamOne.NameReturns("avengers")

				fakeTeamTwo.IDReturns(9)
				fakeTeamTwo.NameReturns("aliens")
				fakeTeamTwo.BasicAuthReturns(&atc.BasicAuth{
					BasicAuthUsername: "fake user",
					BasicAuthPassword: "no, bad",
				})

				data := []byte(`{"hello": "world"}`)
				fakeTeamThree.IDReturns(22)
				fakeTeamThree.NameReturns("predators")
				fakeTeamThree.AuthReturns(map[string]*json.RawMessage{
					"fake-provider": (*json.RawMessage)(&data),
				})
				dbTeamFactory.GetTeamsReturns([]db.Team{fakeTeamOne, fakeTeamTwo, fakeTeamThree}, nil)
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
				jwtValidator.IsAuthenticatedReturns(true)
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
				jwtValidator.IsAuthenticatedReturns(true)
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

		var team db.Team
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
				jwtValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns(atc.DefaultTeamName, true, true)
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
				jwtValidator.IsAuthenticatedReturns(true)
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
				jwtValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("", false, false)
			})

			It("returns 500 internal server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/rename", func() {
		var response *http.Response
		var teamName string

		JustBeforeEach(func() {
			request, err := http.NewRequest(
				"PUT",
				server.URL+"/api/v1/teams/"+teamName+"/rename",
				bytes.NewBufferString(`{"name":"some-new-name"}`),
			)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeTeam.IDReturns(2)
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				jwtValidator.IsAuthenticatedReturns(true)
			})
			Context("when requester belongs to an admin team", func() {
				BeforeEach(func() {
					teamName = "a-team"
					fakeTeam.NameReturns(teamName)
					userContextReader.GetTeamReturns(atc.DefaultTeamName, true, true)
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

				It("returns 204 no content", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNoContent))
				})
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					teamName = "a-team"
					fakeTeam.NameReturns(teamName)
					userContextReader.GetTeamReturns("a-team", true, true)
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

				It("returns 204 no content", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNoContent))
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					teamName = "a-team"
					fakeTeam.NameReturns(teamName)
					userContextReader.GetTeamReturns("another-team", false, true)
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
				jwtValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				Expect(fakeTeam.RenameCallCount()).To(Equal(0))
			})
		})
	})
})
