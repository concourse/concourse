package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

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
						{Addr: "1.2.3.4:7777", ActiveContainers: 1},
						{Addr: "1.2.3.4:8888", ActiveContainers: 2},
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
						{Addr: "1.2.3.4:7777", ActiveContainers: 1},
						{Addr: "1.2.3.4:8888", ActiveContainers: 2},
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
			ttl    time.Duration

			response *http.Response
		)

		BeforeEach(func() {
			worker = atc.Worker{
				Addr:             "1.2.3.4:7777",
				ActiveContainers: 2,
			}

			ttl = 30 * time.Second
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(worker)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/workers?ttl="+ttl.String(), ioutil.NopCloser(bytes.NewBuffer(payload)))
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

				It("saves it", func() {
					Ω(workerDB.SaveWorkerCallCount()).Should(Equal(1))

					savedInfo, savedTTL := workerDB.SaveWorkerArgsForCall(0)
					Ω(savedInfo).Should(Equal(db.WorkerInfo{
						Addr:             "1.2.3.4:7777",
						ActiveContainers: 2,
					}))
					Ω(savedTTL).Should(Equal(ttl))
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

			Context("when the worker has no address", func() {
				BeforeEach(func() {
					worker.Addr = ""
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
