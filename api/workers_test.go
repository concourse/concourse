package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Workers API", func() {
	Describe("GET /api/v1/workers", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/workers", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when the workers can be listed", func() {
				BeforeEach(func() {
					workerDB.WorkersReturns([]db.WorkerInfo{
						{
							GardenAddr:       "1.2.3.4:7777",
							BaggageclaimURL:  "5.6.7.8:7788",
							ActiveContainers: 1,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "freebsd",
							Tags:     []string{"demon"},
						},
						{
							GardenAddr:       "1.2.3.4:8888",
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "beos",
							Tags:     []string{"best", "os", "ever", "rip"},
						},
					}, nil)
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("returns the workers", func() {
					var returnedWorkers []atc.Worker
					err := json.NewDecoder(response.Body).Decode(&returnedWorkers)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returnedWorkers).Should(Equal([]atc.Worker{
						{
							GardenAddr:       "1.2.3.4:7777",
							BaggageclaimURL:  "5.6.7.8:7788",
							ActiveContainers: 1,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "freebsd",
							Tags:     []string{"demon"},
						},
						{
							GardenAddr:       "1.2.3.4:8888",
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "beos",
							Tags:     []string{"best", "os", "ever", "rip"},
						},
					}))
				})
			})

			Context("when getting the workers fails", func() {
				BeforeEach(func() {
					workerDB.WorkersReturns(nil, errors.New("oh no!"))
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

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("POST /api/v1/workers", func() {
		var (
			worker atc.Worker
			ttl    string

			response *http.Response
		)

		BeforeEach(func() {
			worker = atc.Worker{
				GardenAddr:       "1.2.3.4:7777",
				BaggageclaimURL:  "5.6.7.8:7788",
				ActiveContainers: 2,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource", Image: "some-resource-image"},
				},
				Platform: "haiku",
				Tags:     []string{"not", "a", "limerick"},
			}

			ttl = "30s"
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(worker)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/workers?ttl="+ttl, ioutil.NopCloser(bytes.NewBuffer(payload)))
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when the worker is valid", func() {
				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				Context("when the name is not provided", func() {
					It("saves it", func() {
						Ω(workerDB.SaveWorkerCallCount()).Should(Equal(1))

						savedInfo, savedTTL := workerDB.SaveWorkerArgsForCall(0)
						Ω(savedInfo).Should(Equal(db.WorkerInfo{
							GardenAddr:       "1.2.3.4:7777",
							Name:             "1.2.3.4:7777",
							BaggageclaimURL:  "5.6.7.8:7788",
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "haiku",
							Tags:     []string{"not", "a", "limerick"},
						}))
						Ω(savedTTL.String()).Should(Equal(ttl))
					})
				})

				Context("when the name is provided", func() {
					BeforeEach(func() {
						worker = atc.Worker{
							GardenAddr:       "1.2.3.4:7777",
							BaggageclaimURL:  "5.6.7.8:7788",
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "haiku",
							Tags:     []string{"not", "a", "limerick"},
							Name:     "poem",
						}
					})

					It("saves it", func() {
						Ω(workerDB.SaveWorkerCallCount()).Should(Equal(1))

						savedInfo, savedTTL := workerDB.SaveWorkerArgsForCall(0)
						Ω(savedInfo).Should(Equal(db.WorkerInfo{
							GardenAddr:       "1.2.3.4:7777",
							Name:             "poem",
							BaggageclaimURL:  "5.6.7.8:7788",
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource", Image: "some-resource-image"},
							},
							Platform: "haiku",
							Tags:     []string{"not", "a", "limerick"},
						}))
						Ω(savedTTL.String()).Should(Equal(ttl))
					})
				})

				Context("and saving it fails", func() {
					BeforeEach(func() {
						workerDB.SaveWorkerReturns(errors.New("oh no!"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when the TTL is invalid", func() {
				BeforeEach(func() {
					ttl = "invalid-duration"
				})

				It("returns 400", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("returns the validation error in the response body", func() {
					Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("malformed ttl")))
				})

				It("does not save it", func() {
					Ω(workerDB.SaveWorkerCallCount()).Should(BeZero())
				})
			})

			Context("when the worker has no address", func() {
				BeforeEach(func() {
					worker.GardenAddr = ""
				})

				It("returns 400", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("returns the validation error in the response body", func() {
					Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("missing address")))
				})

				It("does not save it", func() {
					Ω(workerDB.SaveWorkerCallCount()).Should(BeZero())
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not save the config", func() {
				Ω(workerDB.SaveWorkerCallCount()).Should(BeZero())
			})
		})
	})
})
