package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth API", func() {
	Describe("GET /api/v1/teams/:team_name/auth/token", func() {
		var request *http.Request
		var response *http.Response

		var savedTeam db.SavedTeam

		BeforeEach(func() {
			savedTeam = db.SavedTeam{
				ID: 0,
				Team: db.Team{
					Name:  "some-team",
					Admin: true,
				},
			}

			teamDB.GetTeamReturns(savedTeam, true, nil)

			var err error
			request, err = http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/auth/token", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when the request's authorization is some other form", func() {
				BeforeEach(func() {
					request.Header.Add("Authorization", "Basic grylls")
				})

				Context("when generating the token succeeds", func() {
					BeforeEach(func() {
						fakeTokenGenerator.GenerateTokenReturns("some type", "some value", nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns application/json", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns a token valid for 1 day", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{"type":"some type","value":"some value"}`))

						expiration, teamName, isAdmin := fakeTokenGenerator.GenerateTokenArgsForCall(0)
						Expect(expiration).To(BeTemporally("~", time.Now().Add(24*time.Hour), time.Minute))
						Expect(teamName).To(Equal(savedTeam.Name))
						Expect(isAdmin).To(Equal(savedTeam.Admin))
					})
				})

				Context("when generating the token fails", func() {
					BeforeEach(func() {
						fakeTokenGenerator.GenerateTokenReturns("", "", errors.New("nope"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the team can't be found", func() {
					BeforeEach(func() {
						fakeTokenGenerator.GenerateTokenReturns("", "", errors.New("nope"))
						teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
					})

					It("returns unauthorized", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not generate a token", func() {
				Expect(fakeTokenGenerator.GenerateTokenCallCount()).To(Equal(0))
			})
		})
	})

	Describe("GET /api/v1/teams/some-team/auth/methods", func() {
		Context("when providers are present", func() {
			var request *http.Request
			var response *http.Response

			var savedTeam db.SavedTeam

			BeforeEach(func() {
				savedTeam = db.SavedTeam{
					ID: 0,
					Team: db.Team{
						Name: "some-team",
						BasicAuth: &db.BasicAuth{
							BasicAuthUsername: "user",
							BasicAuthPassword: "password",
						},
						UAAAuth: &db.UAAAuth{
							ClientID:     "client-id",
							ClientSecret: "client-secret",
						},
						GitHubAuth: &db.GitHubAuth{
							ClientID:     "client-id",
							ClientSecret: "client-secret",
						},
						GenericOAuth: &db.GenericOAuth{
							ClientID:     "client-id",
							ClientSecret: "client-secret",
							DisplayName:  "custom secure auth",
						},
					},
				}

				teamDB.GetTeamReturns(savedTeam, true, nil)

				var err error
				request, err = http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/auth/methods", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets the teamDB for the right team name", func() {
				Expect(teamDBFactory.GetTeamDBCallCount()).To(Equal(1))
				Expect(teamDBFactory.GetTeamDBArgsForCall(0)).To(Equal("some-team"))
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns the configured providers", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[
					{
						"type": "oauth",
						"display_name": "GitHub",
						"auth_url": "https://oauth.example.com/auth/github?team_name=some-team"
					},
					{
						"type": "oauth",
						"display_name": "UAA",
						"auth_url": "https://oauth.example.com/auth/uaa?team_name=some-team"
					},
					{
						"type": "oauth",
						"display_name": "custom secure auth",
						"auth_url": "https://oauth.example.com/auth/oauth?team_name=some-team"
					},
					{
						"type": "basic",
						"display_name": "Basic Auth",
						"auth_url": "https://example.com/teams/some-team/login"
					}
				]`))
			})
		})

		Context("when no providers are present", func() {
			var request *http.Request
			var response *http.Response

			var savedTeam db.SavedTeam

			BeforeEach(func() {
				savedTeam = db.SavedTeam{
					ID: 0,
					Team: db.Team{
						Name: "some-team",
					},
				}

				teamDB.GetTeamReturns(savedTeam, true, nil)

				var err error
				request, err = http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/auth/methods", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns an empty set of providers", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[]`))
			})
		})

		Context("when team cannot be found", func() {
			var request *http.Request
			var response *http.Response

			BeforeEach(func() {
				teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)

				var err error
				request, err = http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/auth/methods", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns Not Found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("GET /api/v1/user", func() {
		var (
			request  *http.Request
			response *http.Response

			err       error
			savedTeam db.SavedTeam
		)

		BeforeEach(func() {
			savedTeam = db.SavedTeam{
				ID: 5,
				Team: db.Team{
					Name:  "some-team",
					Admin: true,
				},
			}

			request, err = http.NewRequest("GET", server.URL+"/api/v1/user", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("as system", func() {
				BeforeEach(func() {
					userContextReader.GetSystemReturns(true, true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type application/json", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("returns the system response", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{"system":true}`))
				})
			})

			Context("as an user in some-team", func() {
				Context("but no auth in the context", func() {
					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("auth found in the context", func() {
					BeforeEach(func() {
						userContextReader.GetTeamReturns("some-team", false, true)
					})

					Context("as an user in a some-team", func() {
						Context("and fails to retrieve team from db", func() {
							BeforeEach(func() {
								userContextReader.GetSystemReturns(false, false)
								teamDB.GetTeamReturns(db.SavedTeam{}, false, errors.New("disaster"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("and team not found in the db", func() {
							BeforeEach(func() {
								teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
							})

							It("returns empty json", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`{}`))
							})
						})

						Context("and team found in the db", func() {
							BeforeEach(func() {
								teamDB.GetTeamReturns(savedTeam, true, nil)
							})

							It("returns 200 OK", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("returns Content-Type application/json", func() {
								Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
							})

							It("returns the team", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`{"team":{"id":5,"name":"some-team"}}`))
							})
						})
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
