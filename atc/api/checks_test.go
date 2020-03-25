package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Checks API", func() {
	Describe("GET /api/v1/checks/:check_id", func() {
		var err error
		var path string
		var response *http.Response

		BeforeEach(func() {
			path = "/api/v1/checks/10"
		})

		JustBeforeEach(func() {
			response, err = client.Get(server.URL + path)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.HasTokenReturns(true)
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when parsing the check_id fails", func() {
				BeforeEach(func() {
					path = "/api/v1/checks/nope"
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("when parsing the check_id succeeds", func() {
				BeforeEach(func() {
					path = "/api/v1/checks/10"
				})

				Context("when calling the database fails", func() {
					BeforeEach(func() {
						dbCheckFactory.CheckReturns(nil, false, errors.New("disaster"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the check cannot be found", func() {
					BeforeEach(func() {
						dbCheckFactory.CheckReturns(nil, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when the check can be found", func() {
					var fakeCheck *dbfakes.FakeCheck

					BeforeEach(func() {
						fakeCheck = new(dbfakes.FakeCheck)
						fakeCheck.IDReturns(10)
						fakeCheck.StatusReturns("errored")
						fakeCheck.CreateTimeReturns(time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC))
						fakeCheck.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
						fakeCheck.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))
						fakeCheck.CheckErrorReturns(errors.New("nope"))

						dbCheckFactory.CheckReturns(fakeCheck, true, nil)
					})

					Context("when fetching checkables errors", func() {
						BeforeEach(func() {
							fakeCheck.AllCheckablesReturns(nil, errors.New("nope"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when fetching checkables returns no results", func() {
						BeforeEach(func() {
							fakeCheck.AllCheckablesReturns([]db.Checkable{}, nil)
						})

						It("returns 403", func() {
							Expect(response.StatusCode).To(Equal(http.StatusForbidden))
						})
					})

					Context("when fetching checkables returns results", func() {
						var fakeResource1 *dbfakes.FakeResource
						var fakeResource2 *dbfakes.FakeResource

						BeforeEach(func() {
							fakeResource1 = new(dbfakes.FakeResource)
							fakeResource2 = new(dbfakes.FakeResource)

							fakeCheck.AllCheckablesReturns([]db.Checkable{fakeResource1, fakeResource2}, nil)
						})

						Context("when not authorized for either team", func() {
							BeforeEach(func() {
								fakeAccess.IsAuthorizedReturns(false)
							})

							It("returns 403", func() {
								Expect(response.StatusCode).To(Equal(http.StatusForbidden))
							})
						})

						Context("when authorized for any team", func() {
							BeforeEach(func() {
								fakeAccess.IsAuthorizedReturns(true)
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("returns application/json", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/json",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("returns the check", func() {
								Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
									 "id": 10,
									 "status": "errored",
									 "create_time": 946684800,
									 "start_time": 978307200,
									 "end_time": 1009843200,
									 "check_error": "nope"
								}`))
							})
						})
					})
				})
			})
		})
	})
})
