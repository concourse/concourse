package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	radarfakes "github.com/concourse/atc/radar/fakes"
	"github.com/concourse/atc/resource"
)

var _ = Describe("Resources API", func() {
	var fakePipelineDB *dbfakes.FakePipelineDB

	BeforeEach(func() {
		fakePipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDBFactory.BuildWithTeamNameAndNameReturns(fakePipelineDB, nil)
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/resources", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/a-pipeline/resources")
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
			teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
			Expect(pipelineName).To(Equal("a-pipeline"))
			Expect(teamName).To(Equal(atc.DefaultTeamName))
		})

		Context("when getting the resource config succeeds", func() {
			BeforeEach(func() {
				fakePipelineDB.GetConfigReturns(atc.Config{
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
				}, 1, true, nil)
			})

			Context("when getting the check error succeeds", func() {
				BeforeEach(func() {
					fakePipelineDB.GetResourceStub = func(name string) (db.SavedResource, error) {
						if name == "resource-2" {
							return db.SavedResource{
								ID:           1,
								CheckError:   errors.New("sup"),
								PipelineName: "a-pipeline",
								Resource: db.Resource{
									Name: name,
								},
							}, nil
						} else {
							return db.SavedResource{
								ID:           2,
								Paused:       true,
								PipelineName: "a-pipeline",
								Resource: db.Resource{
									Name: name,
								},
							}, nil
						}
					}
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						authValidator.IsAuthenticatedReturns(true)
					})

					It("returns each resource, including their check failure", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-1"
							},
							{
								"name": "resource-2",
								"type": "type-2",
								"groups": ["group-2"],
								"url": "/pipelines/a-pipeline/resources/resource-2",
								"failing_to_check": true,
								"check_error": "sup"
							},
							{
								"name": "resource-3",
								"type": "type-3",
								"groups": [],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-3"
							}
						]`))
					})
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						authValidator.IsAuthenticatedReturns(false)
					})

					It("returns each resource, excluding their check failure", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-1"
							},
							{
								"name": "resource-2",
								"type": "type-2",
								"groups": ["group-2"],
								"url": "/pipelines/a-pipeline/resources/resource-2",
								"failing_to_check": true
							},
							{
								"name": "resource-3",
								"type": "type-3",
								"groups": [],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-3"
							}
						]`))

					})
				})
			})

			Context("when getting the resource check error", func() {
				BeforeEach(func() {
					fakePipelineDB.GetResourceStub = func(name string) (db.SavedResource, error) {
						return db.SavedResource{}, errors.New("oh no!")
					}
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when getting the resource config fails", func() {
			Context("when the pipeline is no longer configured", func() {
				BeforeEach(func() {
					fakePipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("with an unknown error", func() {
				BeforeEach(func() {
					fakePipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/resources/:resource_name", func() {
		var response *http.Response
		var resourceName string
		BeforeEach(func() {
			resourceName = "some-resource"
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/pipelines/a-pipeline/resources/%s", server.URL, resourceName), nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls to get the config from the fakePipelineDB", func() {
			Expect(fakePipelineDB.GetConfigCallCount()).To(Equal(1))
		})

		Context("when the call to get config returns an error", func() {
			BeforeEach(func() {
				fakePipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("disaster"))
			})

			It("returns a 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the config in the database can't be found", func() {
			BeforeEach(func() {
				fakePipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
			})

			It("returns a 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when getting the config is successful", func() {
			BeforeEach(func() {
				fakePipelineDB.GetConfigReturns(atc.Config{
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
				}, 1, true, nil)
			})

			Context("when the resource cannot be found in the config", func() {
				It("returns a 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the resource is found in the config", func() {
				BeforeEach(func() {
					resourceName = "resource-1"
				})

				It("looks it up in the database", func() {
					Expect(fakePipelineDB.GetResourceCallCount()).To(Equal(1))
					Expect(fakePipelineDB.GetResourceArgsForCall(0)).To(Equal("resource-1"))
				})

				Context("when the call to the db returns an error", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceReturns(db.SavedResource{}, errors.New("Oh no!"))
					})

					It("returns a 500 error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the call to get a resource succeeds", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceReturns(db.SavedResource{
							ID:           1,
							CheckError:   errors.New("sup"),
							Paused:       true,
							PipelineName: "a-pipeline",
							Resource: db.Resource{
								Name: "resource-1",
							},
						}, nil)
					})

					It("returns 200 ok", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					Context("when not authenticated", func() {
						It("returns the resource json without the check error", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"url": "/pipelines/a-pipeline/resources/resource-1",
								"paused": true,
								"failing_to_check": true
							}`))
						})
					})

					Context("when authenticated", func() {
						BeforeEach(func() {
							authValidator.IsAuthenticatedReturns(true)
						})

						It("returns the resource json with the check error", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"url": "/pipelines/a-pipeline/resources/resource-1",
								"paused": true,
								"failing_to_check": true,
								"check_error": "sup"
							}`))
						})
					})
				})
			})
		})

	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/resources/:resource_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/a-pipeline/resources/resource-name/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(teamName).To(Equal(atc.DefaultTeamName))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					fakePipelineDB.PauseResourceReturns(nil)
				})

				It("paused the right resource", func() {
					Expect(fakePipelineDB.PauseResourceArgsForCall(0)).To(Equal("resource-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the resource fails", func() {
				BeforeEach(func() {
					fakePipelineDB.PauseResourceReturns(errors.New("welp"))
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

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/resources/:resource_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/a-pipeline/resources/resource-name/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(teamName).To(Equal(atc.DefaultTeamName))
			})

			Context("when unpausing the resource succeeds", func() {
				BeforeEach(func() {
					fakePipelineDB.UnpauseResourceReturns(nil)
				})

				It("unpaused the right resource", func() {
					Expect(fakePipelineDB.UnpauseResourceArgsForCall(0)).To(Equal("resource-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when unpausing the resource fails", func() {
				BeforeEach(func() {
					fakePipelineDB.UnpauseResourceReturns(errors.New("welp"))
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

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/resources/:resource_name/check", func() {
		var fakeScanner *radarfakes.FakeScanner
		var checkRequestBody atc.CheckRequestBody
		var response *http.Response

		BeforeEach(func() {
			fakeScanner = new(radarfakes.FakeScanner)
			fakeScannerFactory.NewResourceScannerReturns(fakeScanner)

			checkRequestBody = atc.CheckRequestBody{}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(checkRequestBody)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", server.URL+"/api/v1/pipelines/a-pipeline/resources/resource-name/check", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())

			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(teamName).To(Equal(atc.DefaultTeamName))
			})

			Context("when checking succeeds", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(nil)
				})

				Context("when checking no version specified", func() {
					It("called Scan with no version specified", func() {
						Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
						_, actualResourceName, actualFromVersion := fakeScanner.ScanFromVersionArgsForCall(0)
						Expect(actualResourceName).To(Equal("resource-name"))
						Expect(actualFromVersion).To(BeNil())
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})

				Context("when checking with a version specified", func() {
					BeforeEach(func() {
						checkRequestBody = atc.CheckRequestBody{
							From: atc.Version{
								"some-version-key": "some-version-value",
							},
						}
					})

					It("called Scan with a fromVersion specified", func() {
						Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
						_, actualResourceName, actualFromVersion := fakeScanner.ScanFromVersionArgsForCall(0)
						Expect(actualResourceName).To(Equal("resource-name"))
						Expect(actualFromVersion).To(Equal(checkRequestBody.From))
					})
				})
			})

			Context("when checking the resource fails internally", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when checking the resource fails with ErrResourceScriptFailed", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(
						resource.ErrResourceScriptFailed{
							ExitStatus: 42,
							Stderr:     "my tooth",
						},
					)
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("returns the script's exit status and stderr", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{
						"exit_status": 42,
						"stderr": "my tooth"
					}`))
				})

				It("returns application/json", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
		})
	})
})
