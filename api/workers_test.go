package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
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
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				userContextReader.GetTeamReturns("some-team", false, true)
				authValidator.IsAuthenticatedReturns(true)
			})

			It("fetches workers by team name from user context", func() {
				Expect(dbTeam.WorkersCallCount()).To(Equal(1))

				Expect(teamDBFactory.GetTeamDBCallCount()).To(Equal(1))
				teamName := teamDBFactory.GetTeamDBArgsForCall(0)
				Expect(teamName).To(Equal("some-team"))
			})

			Context("when the workers can be listed", func() {
				var (
					teamWorker1 *dbngfakes.FakeWorker
					teamWorker2 *dbngfakes.FakeWorker
				)

				BeforeEach(func() {

					teamWorker1 = new(dbngfakes.FakeWorker)
					gardenAddr1 := "1.2.3.4:7777"
					teamWorker1.GardenAddrReturns(&gardenAddr1)
					bcURL1 := "1.2.3.4:8888"
					teamWorker1.BaggageclaimURLReturns(&bcURL1)

					teamWorker2 = new(dbngfakes.FakeWorker)
					gardenAddr2 := "5.6.7.8:7777"
					teamWorker2.GardenAddrReturns(&gardenAddr2)
					bcURL2 := "5.6.7.8:8888"
					teamWorker2.BaggageclaimURLReturns(&bcURL2)

					dbTeam.WorkersReturns([]dbng.Worker{
						teamWorker1,
						teamWorker2,
					}, nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the workers", func() {
					var returnedWorkers []atc.Worker
					err := json.NewDecoder(response.Body).Decode(&returnedWorkers)
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedWorkers).To(Equal([]atc.Worker{
						{
							GardenAddr:      "1.2.3.4:7777",
							BaggageclaimURL: "1.2.3.4:8888",
						},
						{
							GardenAddr:      "5.6.7.8:7777",
							BaggageclaimURL: "5.6.7.8:8888",
						},
					}))

				})
			})

			Context("when getting the workers fails", func() {
				BeforeEach(func() {
					dbTeam.WorkersReturns(nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
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
				Name:             "worker-name",
				GardenAddr:       "1.2.3.4:7777",
				BaggageclaimURL:  "5.6.7.8:7788",
				HTTPProxyURL:     "http://example.com",
				HTTPSProxyURL:    "https://example.com",
				NoProxy:          "example.com,127.0.0.1,localhost",
				ActiveContainers: 2,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource", Image: "some-resource-image"},
				},
				Platform: "haiku",
				Tags:     []string{"not", "a", "limerick"},
			}

			ttl = "30s"
			userContextReader.GetTeamReturns("some-team", true, true)
			userContextReader.GetSystemReturns(true, true)
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(worker)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/workers?ttl="+ttl, ioutil.NopCloser(bytes.NewBuffer(payload)))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("tries to save the worker", func() {
				Expect(dbWorkerFactory.SaveWorkerCallCount()).To(Equal(1))
				savedWorker, savedTTL := dbWorkerFactory.SaveWorkerArgsForCall(0)
				Expect(savedWorker).To(Equal(atc.Worker{
					GardenAddr:       "1.2.3.4:7777",
					Name:             "worker-name",
					BaggageclaimURL:  "5.6.7.8:7788",
					HTTPProxyURL:     "http://example.com",
					HTTPSProxyURL:    "https://example.com",
					NoProxy:          "example.com,127.0.0.1,localhost",
					ActiveContainers: 2,
					ResourceTypes: []atc.WorkerResourceType{
						{Type: "some-resource", Image: "some-resource-image"},
					},
					Platform: "haiku",
					Tags:     []string{"not", "a", "limerick"},
				}))

				Expect(savedTTL.String()).To(Equal(ttl))
			})

			Context("when request is not from tsa", func() {
				Context("when system claim is not present", func() {
					BeforeEach(func() {
						userContextReader.GetSystemReturns(false, false)
					})

					It("return 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("when system claim is false", func() {
					BeforeEach(func() {
						userContextReader.GetSystemReturns(false, true)
					})

					It("return 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})
			})

			Context("when payload contains team name", func() {
				BeforeEach(func() {
					worker.Team = "some-team"
				})

				Context("when specified team exists", func() {
					var foundTeam *dbngfakes.FakeTeam

					BeforeEach(func() {
						foundTeam = new(dbngfakes.FakeTeam)
						dbTeamFactory.FindTeamReturns(foundTeam, true, nil)
					})

					It("saves team name in db", func() {
						Expect(foundTeam.SaveWorkerCallCount()).To(Equal(1))
					})

					Context("when saving the worker succeeds", func() {
						BeforeEach(func() {
							foundTeam.SaveWorkerReturns(new(dbngfakes.FakeWorker), nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when saving the worker fails", func() {
						BeforeEach(func() {
							foundTeam.SaveWorkerReturns(nil, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when specified team does not exist", func() {
					BeforeEach(func() {
						dbTeamFactory.FindTeamReturns(nil, false, nil)
					})

					It("returns 400", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})
			})

			Context("when the worker has no name", func() {
				BeforeEach(func() {
					worker.Name = ""
				})

				It("tries to save the worker with the garden address as the name", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(Equal(1))

					savedInfo, savedTTL := dbWorkerFactory.SaveWorkerArgsForCall(0)
					Expect(savedInfo).To(Equal(atc.Worker{
						GardenAddr:       "1.2.3.4:7777",
						Name:             "1.2.3.4:7777",
						BaggageclaimURL:  "5.6.7.8:7788",
						HTTPProxyURL:     "http://example.com",
						HTTPSProxyURL:    "https://example.com",
						NoProxy:          "example.com,127.0.0.1,localhost",
						ActiveContainers: 2,
						ResourceTypes: []atc.WorkerResourceType{
							{Type: "some-resource", Image: "some-resource-image"},
						},
						Platform: "haiku",
						Tags:     []string{"not", "a", "limerick"},
					}))

					Expect(savedTTL.String()).To(Equal(ttl))
				})
			})

			Context("when saving the worker succeeds", func() {
				BeforeEach(func() {
					dbWorkerFactory.SaveWorkerReturns(new(dbngfakes.FakeWorker), nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when saving the worker fails", func() {
				BeforeEach(func() {
					dbWorkerFactory.SaveWorkerReturns(nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the TTL is invalid", func() {
				BeforeEach(func() {
					ttl = "invalid-duration"
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("returns the validation error in the response body", func() {
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("malformed ttl")))
				})

				It("does not save it", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(BeZero())
				})
			})

			Context("when the worker has no address", func() {
				BeforeEach(func() {
					worker.GardenAddr = ""
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("returns the validation error in the response body", func() {
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("missing address")))
				})

				It("does not save it", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(BeZero())
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not save the config", func() {
				Expect(dbWorkerFactory.SaveWorkerCallCount()).To(BeZero())
			})
		})
	})

	Describe("PUT /api/v1/workers/:worker_name/land", func() {
		var (
			response   *http.Response
			workerName string
			fakeWorker *dbngfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/land", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbngfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)
			fakeWorker.TeamNameReturns("some-team")
			fakeWorker.LandReturns(nil)

			authValidator.IsAuthenticatedReturns(true)
			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
		})

		Context("when the request is authenticated as system", func() {
			BeforeEach(func() {
				userContextReader.GetSystemReturns(true, true)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("sees if the worker exists and attempts to land it", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(Equal(1))
				Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))
				Expect(fakeWorker.LandCallCount()).To(Equal(1))
			})

			Context("when landing the worker fails", func() {
				var returnedErr error

				BeforeEach(func() {
					returnedErr = errors.New("some-error")
					fakeWorker.LandReturns(returnedErr)
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the worker does not exist", func() {
				BeforeEach(func() {
					dbWorkerFactory.GetWorkerReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when the request is authenticated as the worker's owner", func() {
			BeforeEach(func() {
				userContextReader.GetTeamReturns("some-team", false, true)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when the request is authenticated as the wrong team", func() {
			BeforeEach(func() {
				userContextReader.GetTeamReturns("some-other-team", false, true)
			})

			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not attempt to find the worker", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(BeZero())
			})
		})
	})

	Describe("PUT /api/v1/workers/:worker_name/retire", func() {
		var (
			response   *http.Response
			workerName string
			fakeWorker *dbngfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/retire", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbngfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)
			fakeWorker.TeamNameReturns("some-team")

			authValidator.IsAuthenticatedReturns(true)

			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
			fakeWorker.RetireReturns(nil)
		})

		Context("when autheticated as system", func() {
			BeforeEach(func() {
				userContextReader.GetSystemReturns(true, true)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("sees if the worker exists and attempts to retire it", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(Equal(1))
				Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))

				Expect(fakeWorker.RetireCallCount()).To(Equal(1))
			})

			Context("when retiring the worker fails", func() {
				var returnedErr error

				BeforeEach(func() {
					returnedErr = errors.New("some-error")
					fakeWorker.RetireReturns(returnedErr)
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the worker does not exist", func() {
				BeforeEach(func() {
					dbWorkerFactory.GetWorkerReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when autheticated as as the worker's owner", func() {
			BeforeEach(func() {
				userContextReader.GetTeamReturns("some-team", false, true)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when autheticated as some other team", func() {
			BeforeEach(func() {
				userContextReader.GetTeamReturns("some-other-team", false, true)
			})

			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not attempt to find the worker", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(BeZero())
			})
		})
	})

	Describe("PUT /api/v1/workers/:worker_name/prune", func() {
		var (
			response   *http.Response
			workerName string
			fakeWorker *dbngfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/prune", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbngfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)
			fakeWorker.TeamNameReturns("some-team")

			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
			authValidator.IsAuthenticatedReturns(true)
			userContextReader.GetTeamReturns("some-team", false, true)
			fakeWorker.PruneReturns(nil)
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("sees if the worker exists and attempts to prune it", func() {
			Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))
			Expect(fakeWorker.PruneCallCount()).To(Equal(1))
		})

		Context("when pruning the worker fails", func() {
			var returnedErr error

			BeforeEach(func() {
				returnedErr = errors.New("some-error")
				fakeWorker.PruneReturns(returnedErr)
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the worker does not exist", func() {
			BeforeEach(func() {
				dbWorkerFactory.GetWorkerReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when the worker is running", func() {
			BeforeEach(func() {
				fakeWorker.PruneReturns(dbng.ErrCannotPruneRunningWorker)
			})

			It("returns 400", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"stderr":"cannot prune running worker"}`))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not attempt to find the worker", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(BeZero())
			})
		})
	})

	Describe("PUT /api/v1/workers/:worker_name/heartbeat", func() {
		var (
			response   *http.Response
			workerName string
			ttlStr     string
			ttl        time.Duration
			err        error

			worker     atc.Worker
			fakeWorker *dbngfakes.FakeWorker
		)

		BeforeEach(func() {
			fakeWorker = new(dbngfakes.FakeWorker)
			workerName = "some-name"
			fakeWorker.NameReturns(workerName)
			fakeWorker.ActiveContainersReturns(2)
			fakeWorker.PlatformReturns("penguin")
			fakeWorker.TagsReturns([]string{"some-tag"})
			fakeWorker.StateReturns(dbng.WorkerStateRunning)
			fakeWorker.TeamNameReturns("some-team")

			ttlStr = "30s"
			ttl, err = time.ParseDuration(ttlStr)
			Expect(err).NotTo(HaveOccurred())

			worker = atc.Worker{
				Name:             workerName,
				ActiveContainers: 2,
			}

			authValidator.IsAuthenticatedReturns(true)
			dbWorkerFactory.HeartbeatWorkerReturns(fakeWorker, nil)
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(worker)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/heartbeat?ttl="+ttlStr, ioutil.NopCloser(bytes.NewBuffer(payload)))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns saved worker", func() {
			contents, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(contents).To(MatchJSON(`{
				"name": "some-name",
				"state": "running",
				"addr": "",
				"baggageclaim_url": "",
				"active_containers": 2,
				"resource_types": null,
				"platform": "penguin",
				"tags": ["some-tag"],
				"team": "some-team",
				"name": "some-name",
				"start_time": 0
			}`))
		})

		It("sees if the worker exists and attempts to heartbeat with provided ttl", func() {
			Expect(dbWorkerFactory.HeartbeatWorkerCallCount()).To(Equal(1))

			w, t := dbWorkerFactory.HeartbeatWorkerArgsForCall(0)
			Expect(w).To(Equal(worker))
			Expect(t).To(Equal(ttl))
		})

		Context("when the TTL is invalid", func() {
			BeforeEach(func() {
				ttlStr = "invalid-duration"
			})

			It("returns 400", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("returns the validation error in the response body", func() {
				Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("malformed ttl")))
			})

			It("does not heartbeat worker", func() {
				Expect(dbWorkerFactory.HeartbeatWorkerCallCount()).To(BeZero())
			})
		})

		Context("when heartbeating the worker fails", func() {
			var returnedErr error

			BeforeEach(func() {
				returnedErr = errors.New("some-error")
				dbWorkerFactory.HeartbeatWorkerReturns(nil, returnedErr)
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the worker does not exist", func() {
			BeforeEach(func() {
				dbWorkerFactory.HeartbeatWorkerReturns(nil, dbng.ErrWorkerNotPresent)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not heartbeat the worker", func() {
				Expect(dbWorkerFactory.HeartbeatWorkerCallCount()).To(BeZero())
			})
		})
	})

	Describe("DELETE /api/v1/workers/:worker_name", func() {
		var (
			response   *http.Response
			workerName string
			fakeWorker *dbngfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("DELETE", server.URL+"/api/v1/workers/"+workerName, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbngfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)

			authValidator.IsAuthenticatedReturns(true)
			fakeWorker.DeleteReturns(nil)
			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("deletes the worker from the DB", func() {
			Expect(dbWorkerFactory.GetWorkerCallCount()).To(Equal(1))
			Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))

			Expect(fakeWorker.DeleteCallCount()).To(Equal(1))
		})

		Context("when deleting the worker fails", func() {
			var returnedErr error

			BeforeEach(func() {
				returnedErr = errors.New("some-error")
				fakeWorker.DeleteReturns(returnedErr)
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not attempt to find the worker", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(BeZero())
			})
		})
	})
})
