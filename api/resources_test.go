package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
)

var _ = Describe("Resources API", func() {
	Describe("GET /api/v1/resources", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/resources")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the resource config succeeds", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{
					Groups: []atc.GroupConfig{
						{
							Name:      "group-1",
							Resources: []string{"resource-1"},
						},
						{
							Name:      "group-2",
							Resources: []string{"resource-1", "resource-2"},
						},
					},

					Resources: []atc.ResourceConfig{
						{Name: "resource-1", Type: "type-1"},
						{Name: "resource-2", Type: "type-2"},
						{Name: "resource-3", Type: "type-3"},
					},
				}, 1, nil)
			})

			Context("when getting the check error succeeds", func() {
				BeforeEach(func() {
					resourceDB.GetResourceCheckErrorStub = func(name string) (error, error) {
						if name == "resource-2" {
							return errors.New("sup"), nil
						} else {
							return nil, nil
						}
					}
				})

				It("returns 200 OK", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("returns each resource", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`[
									{
										"name": "resource-1",
										"type": "type-1",
										"groups": ["group-1", "group-2"],
										"url": "/resources/resource-1"
									},
									{
										"name": "resource-2",
										"type": "type-2",
										"groups": ["group-2"],
										"url": "/resources/resource-2",
										"check_error": "sup"
									},
									{
										"name": "resource-3",
										"type": "type-3",
										"groups": [],
										"url": "/resources/resource-3"
									}
								]`))
				})
			})

			Context("when getting the resource check error", func() {
				BeforeEach(func() {
					resourceDB.GetResourceCheckErrorStub = func(name string) (error, error) {
						return nil, errors.New("oh no!")
					}
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when getting the resource config fails", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("PUT /api/v1/resources/:resource_id/enable", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/resources/42/enable", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when enabling the resource succeeds", func() {
				BeforeEach(func() {
					resourceDB.EnableVersionedResourceReturns(nil)
				})

				It("enabled the right versioned resource", func() {
					Ω(resourceDB.EnableVersionedResourceArgsForCall(0)).Should(Equal(42))
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when enabling the resource fails", func() {
				BeforeEach(func() {
					resourceDB.EnableVersionedResourceReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/resources/:resource_id/disable", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/resources/42/disable", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when enabling the resource succeeds", func() {
				BeforeEach(func() {
					resourceDB.DisableVersionedResourceReturns(nil)
				})

				It("disabled the right versioned resource", func() {
					Ω(resourceDB.DisableVersionedResourceArgsForCall(0)).Should(Equal(42))
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when enabling the resource fails", func() {
				BeforeEach(func() {
					resourceDB.DisableVersionedResourceReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})
})
