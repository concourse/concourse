package skymarshal_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/provider/providerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth API", func() {
	Describe("GET /auth/basic/token?team_name=some-team", func() {
		var (
			request  *http.Request
			response *http.Response

			fakeTeam *dbfakes.FakeTeam
		)
		BeforeEach(func() {
			fakeTeam = new(dbfakes.FakeTeam)
			fakeTeam.IDReturns(0)
			fakeTeam.NameReturns("some-team")
			fakeTeam.AdminReturns(true)

			dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)

			var err error
			request, err = http.NewRequest("GET", server.URL+"/auth/basic/token?team_name=some-team", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the request's authorization is basic auth", func() {
			BeforeEach(func() {
				fakeBasicAuthValidator.IsAuthenticatedReturns(true)
				request.Header.Add("Authorization", "Basic grylls")
			})

			Context("when generating the token succeeds", func() {
				BeforeEach(func() {
					fakeAuthTokenGenerator.GenerateTokenReturns("some type", "some value", nil)
					fakeCSRFTokenGenerator.GenerateTokenReturns("some-csrf-token", nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns application/json", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("returns CSRF token", func() {
					Expect(response.Header.Get("X-Csrf-Token")).To(Equal("some-csrf-token"))
				})

				It("returns a token valid for 1 day", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{"type":"some type","value":"some value"}`))

					expiration, teamName, isAdmin, csrfToken := fakeAuthTokenGenerator.GenerateTokenArgsForCall(0)
					Expect(expiration).To(BeTemporally("~", time.Now().Add(24*time.Hour), time.Minute))
					Expect(teamName).To(Equal("some-team"))
					Expect(isAdmin).To(Equal(true))
					Expect(csrfToken).To(Equal("some-csrf-token"))
				})
			})

			Context("when generating the token fails", func() {
				BeforeEach(func() {
					fakeAuthTokenGenerator.GenerateTokenReturns("", "", errors.New("nope"))
				})

				It("returns Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when the request's authorization is bearer token", func() {
			BeforeEach(func() {
				fakeBasicAuthValidator.IsAuthenticatedReturns(false)
				request.Header.Add("Authorization", "Bearer grylls")
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not generate a token", func() {
				Expect(fakeAuthTokenGenerator.GenerateTokenCallCount()).To(Equal(0))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeTokenValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not generate a token", func() {
				Expect(fakeAuthTokenGenerator.GenerateTokenCallCount()).To(Equal(0))
			})
		})
	})

	Describe("GET /auth/list_methods?team_name=some-team", func() {
		Context("when providers are present", func() {
			var (
				request  *http.Request
				response *http.Response

				fakeTeam            *dbfakes.FakeTeam
				fakeProviderFactory *providerfakes.FakeTeamProvider
				fakeProviderName    = "FakeProvider"
				fakeAuthConfig      *providerfakes.FakeAuthConfig
			)
			BeforeEach(func() {
				fakeTeam = new(dbfakes.FakeTeam)
				fakeProviderFactory = new(providerfakes.FakeTeamProvider)
				fakeAuthConfig = new(providerfakes.FakeAuthConfig)

				provider.Register(fakeProviderName, fakeProviderFactory)

				data := []byte(`{"mcdonalds": "fries"}`)
				fakeTeam.IDReturns(0)
				fakeTeam.NameReturns("some-team")
				fakeTeam.BasicAuthReturns(&atc.BasicAuth{
					BasicAuthUsername: "user",
					BasicAuthPassword: "password",
				})
				fakeTeam.AuthReturns(map[string]*json.RawMessage{
					fakeProviderName: (*json.RawMessage)(&data),
				})

				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				fakeProviderFactory.UnmarshalConfigReturns(fakeAuthConfig, nil)
				fakeAuthConfig.AuthMethodReturns(provider.AuthMethod{
					Type:        provider.AuthTypeOAuth,
					DisplayName: "fake display",
					AuthURL:     "https://example.com/some-auth-url",
				})

				var err error
				request, err = http.NewRequest("GET", server.URL+"/auth/list_methods?team_name=some-team", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets the team for the right team name", func() {
				Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
				Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("some-team"))
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
						"display_name": "fake display",
						"auth_url": "https://example.com/some-auth-url"
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

			var fakeTeam *dbfakes.FakeTeam

			BeforeEach(func() {
				fakeTeam = new(dbfakes.FakeTeam)
				fakeTeam.IDReturns(0)
				fakeTeam.NameReturns("some-team")

				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)

				var err error
				request, err = http.NewRequest("GET", server.URL+"/auth/list_methods?team_name=some-team", nil)
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
			var fakeTeam *dbfakes.FakeTeam

			BeforeEach(func() {
				fakeTeam = new(dbfakes.FakeTeam)
				dbTeamFactory.FindTeamReturns(fakeTeam, false, nil)

				var err error
				request, err = http.NewRequest("GET", server.URL+"/auth/list_methods?team_name=some-team", nil)
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

			It("returns a JSONAPI error for the team not being found", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{"errors": [{"title": "Team Not Found Error", "detail": "Team with name 'some-team' not found.", "status": "404"}]}`))
			})
		})
	})

	Describe("GET /auth/userinfo", func() {
		var (
			request  *http.Request
			response *http.Response

			err      error
			fakeTeam *dbfakes.FakeTeam
		)

		BeforeEach(func() {
			fakeTeam = new(dbfakes.FakeTeam)
			fakeTeam.IDReturns(5)
			fakeTeam.NameReturns("some-team")
			fakeTeam.AdminReturns(true)

			request, err = http.NewRequest("GET", server.URL+"/auth/userinfo", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeTokenValidator.IsAuthenticatedReturns(true)
			})

			Context("as system", func() {
				BeforeEach(func() {
					fakeTokenReader.GetSystemReturns(true, true)
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
						fakeTokenReader.GetTeamReturns("some-team", false, true)
					})

					Context("as an user in a some-team", func() {
						Context("and fails to retrieve team from db", func() {
							BeforeEach(func() {
								fakeTokenReader.GetSystemReturns(false, false)
								dbTeamFactory.FindTeamReturns(nil, false, errors.New("disaster"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("and team found in the db", func() {
							BeforeEach(func() {
								dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
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
				fakeTokenValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
