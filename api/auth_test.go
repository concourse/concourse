package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/provider/providerfakes"
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

			for _, b := range []string{"bearer", "BEARER", "Bearer"} {
				bearer := b

				Context("when the request's authorization is already a "+bearer+" token", func() {
					BeforeEach(func() {
						request.Header.Add("Authorization", bearer+" grylls")
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns application/json", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns the existing token without generating a new one", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{"type":"` + bearer + `","value":"grylls"}`))

						Expect(fakeTokenGenerator.GenerateTokenCallCount()).To(Equal(0))
					})
				})
			}

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

						expiration, teamName, teamID, isAdmin := fakeTokenGenerator.GenerateTokenArgsForCall(0)
						Expect(expiration).To(BeTemporally("~", time.Now().Add(24*time.Hour), time.Minute))
						Expect(teamName).To(Equal(savedTeam.Name))
						Expect(teamID).To(Equal(savedTeam.ID))
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
				authProvider1 := new(providerfakes.FakeProvider)
				authProvider1.DisplayNameReturns("OAuth Provider 1")

				authProvider2 := new(providerfakes.FakeProvider)
				authProvider2.DisplayNameReturns("OAuth Provider 2")
				providerFactory.GetProvidersReturns(
					provider.Providers{
						"oauth-provider-1": authProvider1,
						"oauth-provider-2": authProvider2,
					},
					nil,
				)

				savedTeam = db.SavedTeam{
					ID: 0,
					Team: db.Team{
						Name: "some-team",
						BasicAuth: &db.BasicAuth{
							BasicAuthUsername: "user",
							BasicAuthPassword: "password",
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
					"display_name": "OAuth Provider 1",
					"auth_url": "https://oauth.example.com/auth/oauth-provider-1?team_name=some-team"
				},
				{
					"type": "oauth",
					"display_name": "OAuth Provider 2",
					"auth_url": "https://oauth.example.com/auth/oauth-provider-2?team_name=some-team"
				},
				{
					"type": "basic",
					"display_name": "Basic Auth",
					"auth_url": "https://example.com/teams/some-team/login/basic"
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
})
