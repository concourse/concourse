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
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/radar/radarfakes"
	"github.com/concourse/atc/resource"
)

var _ = Describe("Resources API", func() {
	var fakePipelineDB *dbfakes.FakePipelineDB
	var expectedSavedPipeline db.SavedPipeline

	BeforeEach(func() {
		fakePipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDBFactory.BuildReturns(fakePipelineDB)
		expectedSavedPipeline = db.SavedPipeline{}
		teamDB.GetPipelineByNameReturns(expectedSavedPipeline, true, nil)
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/a-team/pipelines/a-pipeline/resources")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the dashboard resources succeeds", func() {
			BeforeEach(func() {
				groupConfigs := []atc.GroupConfig{
					{
						Name:      "group-1",
						Resources: []string{"resource-1"},
					},
					{
						Name:      "group-2",
						Resources: []string{"resource-1", "resource-2"},
					},
				}

				dashboardResource1 := db.DashboardResource{
					Resource: db.SavedResource{
						ID:           1,
						CheckError:   nil,
						Paused:       true,
						PipelineName: "a-pipeline",
						Resource:     db.Resource{Name: "resource-1"},
					},
					ResourceConfig: atc.ResourceConfig{
						Name: "resource-1",
						Type: "type-1",
					},
				}

				dashboardResource2 := db.DashboardResource{
					Resource: db.SavedResource{
						ID:           2,
						CheckError:   errors.New("sup"),
						Paused:       false,
						PipelineName: "a-pipeline",
						Resource:     db.Resource{Name: "resource-2"},
					},
					ResourceConfig: atc.ResourceConfig{
						Name: "resource-2",
						Type: "type-2",
					},
				}

				dashboardResource3 := db.DashboardResource{
					Resource: db.SavedResource{
						ID:           3,
						CheckError:   nil,
						Paused:       true,
						PipelineName: "a-pipeline",
						Resource:     db.Resource{Name: "resource-3"},
					},
					ResourceConfig: atc.ResourceConfig{
						Name: "resource-3",
						Type: "type-3",
					},
				}

				fakePipelineDB.GetResourcesReturns([]db.DashboardResource{
					dashboardResource1, dashboardResource2, dashboardResource3,
				}, groupConfigs, true, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(false)
					userContextReader.GetTeamReturns("", 0, false, false)
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						fakePipelineDB.IsPublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						fakePipelineDB.IsPublicReturns(true)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
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
						"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-1"
					},
					{
						"name": "resource-2",
						"type": "type-2",
						"groups": ["group-2"],
						"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-2",
						"failing_to_check": true
					},
					{
						"name": "resource-3",
						"type": "type-3",
						"groups": [],
						"paused": true,
						"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-3"
					}
				]`))
					})
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(true)
					userContextReader.GetTeamReturns("a-team", 1, true, true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
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
							"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-1"
						},
						{
							"name": "resource-2",
							"type": "type-2",
							"groups": ["group-2"],
							"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-2",
							"failing_to_check": true,
							"check_error": "sup"
						},
						{
							"name": "resource-3",
							"type": "type-3",
							"groups": [],
							"paused": true,
							"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-3"
						}
					]`))
				})

				Context("when getting the resource config fails", func() {
					Context("when the pipeline is no longer configured", func() {
						BeforeEach(func() {
							fakePipelineDB.GetResourcesReturns(nil, nil, false, nil)
						})

						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("with an unknown error", func() {
						BeforeEach(func() {
							fakePipelineDB.GetResourcesReturns(nil, nil, false, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name", func() {
		var response *http.Response
		var resourceName string
		BeforeEach(func() {
			resourceName = "some-resource"
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/teams/a-team/pipelines/a-pipeline/resources/%s", server.URL, resourceName), nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
				userContextReader.GetTeamReturns("", 0, false, false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipelineDB.IsPublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipelineDB.IsPublicReturns(true)
					fakePipelineDB.GetConfigReturns(atc.Config{
						Resources: []atc.ResourceConfig{
							{Name: "resource-1", Type: "type-1"},
						}}, 1, true, nil)
					resourceName = "resource-1"
					fakePipelineDB.GetResourceReturns(db.SavedResource{
						CheckError:   errors.New("sup"),
						PipelineName: "a-pipeline",
					}, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the resource json without the check error", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`
					{
						"name": "resource-1",
						"type": "type-1",
						"groups": [],
						"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-1",
						"failing_to_check": true
					}`))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("a-team", 1, true, true)
			})

			It("calls to get the config from the pipelineDB", func() {
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
							{Name: "resource-in-config-but-not-db", Type: "type-1"},
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

					Context("when the resource cannot be found in the database", func() {
						BeforeEach(func() {
							resourceName = "resource-in-config-but-not-db"
						})

						It("returns a 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when the call to the db returns an error", func() {
						BeforeEach(func() {
							fakePipelineDB.GetResourceReturns(db.SavedResource{}, false, errors.New("Oh no!"))
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
							}, true, nil)
						})

						It("returns 200 ok", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns the resource json with the check error", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"url": "/teams/a-team/pipelines/a-pipeline/resources/resource-1",
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

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/pause", func() {
		var response *http.Response

		BeforeEach(func() {
			fakePipelineDB.GetResourceReturns(db.SavedResource{
				Resource: db.Resource{
					Name: "resource-name",
				},
			}, true, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("a-team", 42, true, true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
				pipelineName := teamDB.GetPipelineByNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(pipelineDBFactory.BuildCallCount()).To(Equal(1))
				actualSavedPipeline := pipelineDBFactory.BuildArgsForCall(0)
				Expect(actualSavedPipeline).To(Equal(expectedSavedPipeline))
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

			Context("when resource can not be found", func() {
				BeforeEach(func() {
					fakePipelineDB.GetResourceReturns(db.SavedResource{}, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/unpause", func() {
		var response *http.Response

		BeforeEach(func() {
			fakePipelineDB.GetResourceReturns(db.SavedResource{
				Resource: db.Resource{
					Name: "resource-name",
				},
			}, true, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("a-team", 42, true, true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
				pipelineName := teamDB.GetPipelineByNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(pipelineDBFactory.BuildCallCount()).To(Equal(1))
				actualSavedPipeline := pipelineDBFactory.BuildArgsForCall(0)
				Expect(actualSavedPipeline).To(Equal(expectedSavedPipeline))
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

			Context("when resource can not be found", func() {
				BeforeEach(func() {
					fakePipelineDB.GetResourceReturns(db.SavedResource{}, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check", func() {
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

			request, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/check", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("a-team", 42, true, true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
				pipelineName := teamDB.GetPipelineByNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(pipelineDBFactory.BuildCallCount()).To(Equal(1))
				actualSavedPipeline := pipelineDBFactory.BuildArgsForCall(0)
				Expect(actualSavedPipeline).To(Equal(expectedSavedPipeline))
			})

			It("tries to scan with no version specified", func() {
				Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
				_, actualResourceName, actualFromVersion := fakeScanner.ScanFromVersionArgsForCall(0)
				Expect(actualResourceName).To(Equal("resource-name"))
				Expect(actualFromVersion).To(BeNil())
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			Context("when checking with a version specified", func() {
				BeforeEach(func() {
					checkRequestBody = atc.CheckRequestBody{
						From: atc.Version{
							"some-version-key": "some-version-value",
						},
					}
				})

				It("tries to scan with the version specified", func() {
					Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
					_, actualResourceName, actualFromVersion := fakeScanner.ScanFromVersionArgsForCall(0)
					Expect(actualResourceName).To(Equal("resource-name"))
					Expect(actualFromVersion).To(Equal(checkRequestBody.From))
				})
			})

			Context("when the resource already has versions", func() {
				BeforeEach(func() {
					returnedVersion := db.SavedVersionedResource{
						ID:      4,
						Enabled: true,
						VersionedResource: db.VersionedResource{
							Resource: "some-resource",
							Type:     "some-type",
							Version: db.Version{
								"some": "version",
							},
							Metadata: []db.MetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							},
							PipelineID: 42,
						},
					}
					fakePipelineDB.GetLatestVersionedResourceReturns(returnedVersion, true, nil)
				})

				It("tries to scan with the latest version when no version is passed", func() {
					Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
					_, actualResourceName, actualFromVersion := fakeScanner.ScanFromVersionArgsForCall(0)
					Expect(actualResourceName).To(Equal("resource-name"))
					Expect(actualFromVersion).To(Equal(atc.Version{"some": "version"}))
				})
			})

			Context("when failing to get latest version for resource", func() {
				BeforeEach(func() {
					fakePipelineDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{}, false, errors.New("disaster"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("does not scan from version", func() {
					Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(0))
				})
			})

			Context("when checking fails with ResourceNotFoundError", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(db.ResourceNotFoundError{})
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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
