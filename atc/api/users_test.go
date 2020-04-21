package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users API", func() {

	var (
		response *http.Response
		query    url.Values
	)

	Context("GET /api/v1/user", func() {

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/user", nil)
			Expect(err).NotTo(HaveOccurred())

			req.URL.RawQuery = query.Encode()

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)

				fakeAccess.IsAdminReturns(true)
				fakeAccess.IsSystemReturns(false)

				fakeAccess.ClaimsReturns(accessor.Claims{
					Sub:      "some-sub",
					Name:     "some-name",
					UserID:   "some-user-id",
					UserName: "some-user-name",
					Email:    "some@email.com",
				})

				fakeAccess.TeamRolesReturns(map[string][]string{
					"some-team":       []string{"owner"},
					"some-other-team": []string{"viewer"},
				})
			})

			It("succeeds", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns Content-Type 'application/json'", func() {
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns the current user", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{
							"sub": "some-sub",
							"name": "some-name",
							"user_id": "some-user-id",
							"user_name": "some-user-name",
							"email": "some@email.com",
							"is_admin": true,
							"is_system": false,
							"teams": {
							  "some-team": ["owner"],
							  "some-other-team": ["viewer"]
							}
						}`))
			})
		})

		Context("not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

	})

	Context("GET /api/v1/users", func() {

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/users", nil)
			Expect(err).NotTo(HaveOccurred())

			req.URL.RawQuery = query.Encode()

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("not an admin", func() {

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})

			})

			Context("being an admin", func() {

				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("succeeds", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				Context("failing to retrieve users", func() {
					BeforeEach(func() {
						dbUserFactory.GetAllUsersReturns(nil, errors.New("no db connection"))
					})

					It("fails", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("having no users", func() {
					BeforeEach(func() {
						dbUserFactory.GetAllUsersReturns([]db.User{}, nil)
					})

					It("returns an empty array", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[]`))
					})
				})

				Context("having users", func() {
					var loginDate time.Time
					BeforeEach(func() {
						user1 := new(dbfakes.FakeUser)
						user1.IDReturns(6)
						user1.NameReturns("bob")
						user1.ConnectorReturns("github")
						user1.SubReturns("sub")

						loginDate = time.Unix(10, 0)
						user1.LastLoginReturns(loginDate)

						dbUserFactory.GetAllUsersReturns([]db.User{user1}, nil)
					})

					It("returns all users logged in since table creation", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[{
							"id": 6,
							"username": "bob",
							"connector": "github",
							"last_login": 10
						}]`))
					})

				})

			})

		})

		Context("not authenticated", func() {

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

		})

	})

	Context("GET /api/v1/users?since=", func() {
		var date string
		BeforeEach(func() {
			fakeAccess.IsAuthenticatedReturns(true)
			fakeAccess.IsAdminReturns(true)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/users?since="+date, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})
		Context("with correct date format", func() {
			var loginDate time.Time
			BeforeEach(func() {
				date = "1969-12-30"

				user1 := new(dbfakes.FakeUser)
				user1.IDReturns(6)
				user1.NameReturns("bob")
				user1.ConnectorReturns("github")
				user1.SubReturns("sub")
				loginDate = time.Unix(10, 0)
				user1.LastLoginReturns(loginDate)
				dbUserFactory.GetAllUsersByLoginDateReturns([]db.User{user1}, nil)
			})
			It("returns users", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[{
						"id": 6,
						"username": "bob",
						"connector": "github",
						"last_login": 10
					}]`))
			})
		})

		Context("with incorrect date format", func() {
			BeforeEach(func() {
				date = "1969-14-30"
			})
			It("returns an error message", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{"error": "wrong date format (yyyy-mm-dd)"}`))
			})

			It("returns a HTTP 400", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("no users logged in since the given date", func() {
			BeforeEach(func() {
				date = ""
			})
			It("returns an empty array", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[]`))
			})
		})
	})
})
