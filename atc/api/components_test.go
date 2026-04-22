package api_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Components API", func() {
	var response *http.Response

	Describe("GET /api/v1/components", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/components", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
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

			Context("and is not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and is admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				Context("when getting all components succeeds", func() {
					var fakeComponent *dbfakes.FakeComponent

					BeforeEach(func() {
						fakeComponent = new(dbfakes.FakeComponent)
						fakeComponent.NameReturns("scheduler")
						fakeComponent.IntervalReturns(10 * time.Second)
						fakeComponent.LastRanReturns(time.Now())
						fakeComponent.PausedReturns(false)

						dbComponentFactory.AllReturns([]db.Component{fakeComponent}, nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns the components as JSON", func() {
						body, err := io.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						var components []atc.Component
						err = json.Unmarshal(body, &components)
						Expect(err).NotTo(HaveOccurred())

						Expect(components).To(HaveLen(1))
						Expect(components[0].Name).To(Equal("scheduler"))
						Expect(components[0].Interval).To(Equal(10 * time.Second))
						Expect(components[0].Paused).To(BeFalse())
					})
				})

				Context("when getting all components fails", func() {
					BeforeEach(func() {
						dbComponentFactory.AllReturns(nil, errors.New("disaster"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("PUT /api/v1/components/pause", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/components/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
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

			Context("and is not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and is admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				Context("when pausing all succeeds", func() {
					BeforeEach(func() {
						dbComponentFactory.PauseAllReturns(nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("calls PauseAll", func() {
						Expect(dbComponentFactory.PauseAllCallCount()).To(Equal(1))
					})
				})

				Context("when pausing all fails", func() {
					BeforeEach(func() {
						dbComponentFactory.PauseAllReturns(errors.New("disaster"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("PUT /api/v1/components/unpause", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/components/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
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

			Context("and is not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and is admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				Context("when unpausing all succeeds", func() {
					BeforeEach(func() {
						dbComponentFactory.UnpauseAllReturns(nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("calls UnpauseAll", func() {
						Expect(dbComponentFactory.UnpauseAllCallCount()).To(Equal(1))
					})
				})

				Context("when unpausing all fails", func() {
					BeforeEach(func() {
						dbComponentFactory.UnpauseAllReturns(errors.New("disaster"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("PUT /api/v1/components/:component_name/pause", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/components/some-component/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
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

			Context("and is not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and is admin", func() {
				var fakeComponent *dbfakes.FakeComponent

				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				Context("when the component is found", func() {
					BeforeEach(func() {
						fakeComponent = new(dbfakes.FakeComponent)
						dbComponentFactory.FindReturns(fakeComponent, true, nil)
					})

					Context("and pausing succeeds", func() {
						BeforeEach(func() {
							fakeComponent.PauseReturns(nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("finds the component by name", func() {
							Expect(dbComponentFactory.FindArgsForCall(0)).To(Equal("some-component"))
						})

						It("calls Pause", func() {
							Expect(fakeComponent.PauseCallCount()).To(Equal(1))
						})
					})

					Context("and pausing fails", func() {
						BeforeEach(func() {
							fakeComponent.PauseReturns(errors.New("disaster"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when the component is not found", func() {
					BeforeEach(func() {
						dbComponentFactory.FindReturns(nil, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the component fails", func() {
					BeforeEach(func() {
						dbComponentFactory.FindReturns(nil, false, errors.New("disaster"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("PUT /api/v1/components/:component_name/unpause", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/components/some-component/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
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

			Context("and is not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and is admin", func() {
				var fakeComponent *dbfakes.FakeComponent

				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				Context("when the component is found", func() {
					BeforeEach(func() {
						fakeComponent = new(dbfakes.FakeComponent)
						dbComponentFactory.FindReturns(fakeComponent, true, nil)
					})

					Context("and unpausing succeeds", func() {
						BeforeEach(func() {
							fakeComponent.UnpauseReturns(nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("finds the component by name", func() {
							Expect(dbComponentFactory.FindArgsForCall(0)).To(Equal("some-component"))
						})

						It("calls Unpause", func() {
							Expect(fakeComponent.UnpauseCallCount()).To(Equal(1))
						})
					})

					Context("and unpausing fails", func() {
						BeforeEach(func() {
							fakeComponent.UnpauseReturns(errors.New("disaster"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when the component is not found", func() {
					BeforeEach(func() {
						dbComponentFactory.FindReturns(nil, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the component fails", func() {
					BeforeEach(func() {
						dbComponentFactory.FindReturns(nil, false, errors.New("disaster"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})
})
