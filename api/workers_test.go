package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Workers API", func() {
	var (
		fakeaccess *accessorfakes.FakeAccess
	)
	BeforeEach(func() {
		fakeaccess = new(accessorfakes.FakeAccess)
	})
	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
	})

	Describe("GET /api/v1/workers", func() {
		var response *http.Response

		JustBeforeEach(func() {

			req, err := http.NewRequest("GET", server.URL+"/api/v1/workers", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
				fakeaccess.TeamNamesReturns([]string{"some-team"})
				dbWorkerFactory.VisibleWorkersReturns(nil, nil)
			})

			It("fetches workers by team name from worker user context", func() {
				Expect(dbWorkerFactory.VisibleWorkersCallCount()).To(Equal(1))

				teamNames := dbWorkerFactory.VisibleWorkersArgsForCall(0)
				Expect(teamNames).To(ConsistOf("some-team"))
			})

			Context("when the workers can be listed", func() {
				var (
					teamWorker1 *dbfakes.FakeWorker
					teamWorker2 *dbfakes.FakeWorker
				)

				BeforeEach(func() {

					teamWorker1 = new(dbfakes.FakeWorker)
					gardenAddr1 := "1.2.3.4:7777"
					teamWorker1.GardenAddrReturns(&gardenAddr1)
					bcURL1 := "1.2.3.4:8888"
					teamWorker1.BaggageclaimURLReturns(&bcURL1)

					teamWorker2 = new(dbfakes.FakeWorker)
					gardenAddr2 := "5.6.7.8:7777"
					teamWorker2.GardenAddrReturns(&gardenAddr2)
					bcURL2 := "5.6.7.8:8888"
					teamWorker2.BaggageclaimURLReturns(&bcURL2)
					dbWorkerFactory.VisibleWorkersReturns([]db.Worker{
						teamWorker1,
						teamWorker2,
					}, nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
					dbWorkerFactory.VisibleWorkersReturns(nil, errors.New("error!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("POST /api/v1/workers", func() {
		var (
			worker    atc.Worker
			ttl       string
			certsPath string

			response         *http.Response
			fakeGardenWorker *workerfakes.FakeWorker
		)

		BeforeEach(func() {
			certsPath = "/some/certs/path"
			worker = atc.Worker{
				Name:             "worker-name",
				GardenAddr:       "1.2.3.4:7777",
				BaggageclaimURL:  "5.6.7.8:7788",
				CertsPath:        &certsPath,
				HTTPProxyURL:     "http://example.com",
				HTTPSProxyURL:    "https://example.com",
				NoProxy:          "example.com,127.0.0.1,localhost",
				ActiveContainers: 2,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource", Image: "some-resource-image"},
				},
				Platform: "haiku",
				Tags:     []string{"not", "a", "limerick"},
				Version:  "1.2.3",
			}

			ttl = "30s"
			fakeaccess.IsAuthorizedReturns(true)
			fakeaccess.IsSystemReturns(true)

			fakeGardenWorker = new(workerfakes.FakeWorker)
			fakeWorkerProvider.NewGardenWorkerReturns(fakeGardenWorker)
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
				fakeaccess.IsAuthenticatedReturns(true)
			})

			It("tries to save the worker", func() {
				Expect(dbWorkerFactory.SaveWorkerCallCount()).To(Equal(1))
				savedWorker, savedTTL := dbWorkerFactory.SaveWorkerArgsForCall(0)
				Expect(savedWorker).To(Equal(atc.Worker{
					GardenAddr:       "1.2.3.4:7777",
					Name:             "worker-name",
					BaggageclaimURL:  "5.6.7.8:7788",
					CertsPath:        &certsPath,
					HTTPProxyURL:     "http://example.com",
					HTTPSProxyURL:    "https://example.com",
					NoProxy:          "example.com,127.0.0.1,localhost",
					ActiveContainers: 2,
					ResourceTypes: []atc.WorkerResourceType{
						{Type: "some-resource", Image: "some-resource-image"},
					},
					Platform: "haiku",
					Tags:     []string{"not", "a", "limerick"},
					Version:  "1.2.3",
				}))

				Expect(savedTTL.String()).To(Equal(ttl))
			})

			Context("when request is not from tsa", func() {
				Context("when system claim is not present", func() {
					BeforeEach(func() {
						fakeaccess.IsSystemReturns(false)
					})

					It("return 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("when system claim is false", func() {
					BeforeEach(func() {
						fakeaccess.IsSystemReturns(false)
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
					var foundTeam *dbfakes.FakeTeam

					BeforeEach(func() {
						foundTeam = new(dbfakes.FakeTeam)
						dbTeamFactory.FindTeamReturns(foundTeam, true, nil)
					})

					It("saves team name in db", func() {
						Expect(foundTeam.SaveWorkerCallCount()).To(Equal(1))
					})

					Context("when saving the worker succeeds", func() {
						BeforeEach(func() {
							foundTeam.SaveWorkerReturns(new(dbfakes.FakeWorker), nil)
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
						CertsPath:        &certsPath,
						HTTPProxyURL:     "http://example.com",
						HTTPSProxyURL:    "https://example.com",
						NoProxy:          "example.com,127.0.0.1,localhost",
						ActiveContainers: 2,
						ResourceTypes: []atc.WorkerResourceType{
							{Type: "some-resource", Image: "some-resource-image"},
						},
						Platform: "haiku",
						Tags:     []string{"not", "a", "limerick"},
						Version:  "1.2.3",
					}))

					Expect(savedTTL.String()).To(Equal(ttl))
				})
			})

			Context("when the certs path is null", func() {
				BeforeEach(func() {
					worker.CertsPath = nil
				})

				It("saves the worker with a null certs path", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(Equal(1))

					savedInfo, savedTTL := dbWorkerFactory.SaveWorkerArgsForCall(0)
					Expect(savedInfo).To(Equal(atc.Worker{
						GardenAddr:       "1.2.3.4:7777",
						Name:             "worker-name",
						BaggageclaimURL:  "5.6.7.8:7788",
						CertsPath:        nil,
						HTTPProxyURL:     "http://example.com",
						HTTPSProxyURL:    "https://example.com",
						NoProxy:          "example.com,127.0.0.1,localhost",
						ActiveContainers: 2,
						ResourceTypes: []atc.WorkerResourceType{
							{Type: "some-resource", Image: "some-resource-image"},
						},
						Platform: "haiku",
						Tags:     []string{"not", "a", "limerick"},
						Version:  "1.2.3",
					}))

					Expect(savedTTL.String()).To(Equal(ttl))
				})
			})

			Context("when the certs path is an empty string", func() {
				BeforeEach(func() {
					emptyString := ""
					worker.CertsPath = &emptyString
				})

				It("saves the worker with a null certs path", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(Equal(1))

					savedInfo, savedTTL := dbWorkerFactory.SaveWorkerArgsForCall(0)
					Expect(savedInfo).To(Equal(atc.Worker{
						GardenAddr:       "1.2.3.4:7777",
						Name:             "worker-name",
						BaggageclaimURL:  "5.6.7.8:7788",
						CertsPath:        nil,
						HTTPProxyURL:     "http://example.com",
						HTTPSProxyURL:    "https://example.com",
						NoProxy:          "example.com,127.0.0.1,localhost",
						ActiveContainers: 2,
						ResourceTypes: []atc.WorkerResourceType{
							{Type: "some-resource", Image: "some-resource-image"},
						},
						Platform: "haiku",
						Tags:     []string{"not", "a", "limerick"},
						Version:  "1.2.3",
					}))

					Expect(savedTTL.String()).To(Equal(ttl))
				})
			})

			Context("when saving the worker succeeds", func() {
				var fakeWorker *dbfakes.FakeWorker
				BeforeEach(func() {
					fakeWorker = new(dbfakes.FakeWorker)
					dbWorkerFactory.SaveWorkerReturns(fakeWorker, nil)
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
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("missing garden address")))
				})

				It("does not save it", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(BeZero())
				})
			})

			Context("when worker version is invalid", func() {
				BeforeEach(func() {
					worker.Version = "invalid"
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("returns the validation error in the response body", func() {
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("invalid worker version, only numeric characters are allowed")))
				})

				It("does not save it", func() {
					Expect(dbWorkerFactory.SaveWorkerCallCount()).To(BeZero())
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
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
			fakeWorker *dbfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/land", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)
			fakeWorker.TeamNameReturns("some-team")
			fakeWorker.LandReturns(nil)

			fakeaccess.IsAuthenticatedReturns(true)
			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
		})

		Context("when the request is authenticated as system", func() {
			BeforeEach(func() {
				fakeaccess.IsSystemReturns(true)
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

		Context("when the request is authorized as the worker's owner", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(true)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when the request is authorized as the wrong team", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(false)
			})

			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
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
			fakeWorker *dbfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/retire", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)
			fakeWorker.TeamNameReturns("some-team")
			fakeaccess.IsAuthenticatedReturns(true)

			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
			fakeWorker.RetireReturns(nil)
		})

		Context("when autheticated as system", func() {
			BeforeEach(func() {
				fakeaccess.IsSystemReturns(true)
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

		Context("when authorized as as the worker's owner", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(true)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when authorized as some other team", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(false)
			})

			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
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
			fakeWorker *dbfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/workers/"+workerName+"/prune", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)
			fakeWorker.TeamNameReturns("some-team")

			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
			fakeaccess.IsAuthenticatedReturns(true)
			fakeaccess.IsAuthorizedReturns(true)
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
				fakeWorker.PruneReturns(db.ErrCannotPruneRunningWorker)
			})

			It("returns 400", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"stderr":"cannot prune running worker"}`))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
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
			fakeWorker *dbfakes.FakeWorker
		)

		BeforeEach(func() {
			fakeWorker = new(dbfakes.FakeWorker)
			workerName = "some-name"
			fakeWorker.NameReturns(workerName)
			fakeWorker.ActiveContainersReturns(2)
			fakeWorker.PlatformReturns("penguin")
			fakeWorker.TagsReturns([]string{"some-tag"})
			fakeWorker.StateReturns(db.WorkerStateRunning)
			fakeWorker.TeamNameReturns("some-team")

			ttlStr = "30s"
			ttl, err = time.ParseDuration(ttlStr)
			Expect(err).NotTo(HaveOccurred())

			worker = atc.Worker{
				Name:             workerName,
				ActiveContainers: 2,
			}
			fakeaccess.IsAuthenticatedReturns(true)
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

		It("returns Content-Type 'application/json'", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
				"active_volumes": 0,
				"resource_types": null,
				"platform": "penguin",
				"tags": ["some-tag"],
				"team": "some-team",
				"start_time": 0,
				"version": ""
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
				dbWorkerFactory.HeartbeatWorkerReturns(nil, db.ErrWorkerNotPresent)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
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
			fakeWorker *dbfakes.FakeWorker
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("DELETE", server.URL+"/api/v1/workers/"+workerName, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeWorker = new(dbfakes.FakeWorker)
			workerName = "some-worker"
			fakeWorker.NameReturns(workerName)

			fakeaccess.IsAuthenticatedReturns(true)
			fakeWorker.DeleteReturns(nil)
			dbWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
		})

		Context("when user is system user", func() {
			BeforeEach(func() {
				fakeaccess.IsSystemReturns(true)
			})
			It("deletes the worker from the DB", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(Equal(1))
				Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))

				Expect(fakeWorker.DeleteCallCount()).To(Equal(1))
			})
			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
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
		})

		Context("when user is admin user", func() {
			BeforeEach(func() {
				fakeaccess.IsAdminReturns(true)
			})
			It("deletes the worker from the DB", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(Equal(1))
				Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))

				Expect(fakeWorker.DeleteCallCount()).To(Equal(1))
			})
			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when user is authorized for team", func() {
			BeforeEach(func() {
				fakeWorker.TeamNameReturns("some-team")
				fakeaccess.IsAuthorizedReturns(true)
			})
			It("deletes the worker from the DB", func() {
				Expect(dbWorkerFactory.GetWorkerCallCount()).To(Equal(1))
				Expect(dbWorkerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))

				Expect(fakeWorker.DeleteCallCount()).To(Equal(1))
			})
			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
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
