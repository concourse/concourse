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
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/radar/radarfakes"
	"github.com/concourse/atc/resource"
)

var _ = Describe("Resources API", func() {
	var (
		fakePipeline *dbngfakes.FakePipeline
		resource1    *dbngfakes.FakeResource
	)

	BeforeEach(func() {
		fakePipeline = new(dbngfakes.FakePipeline)
		dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
		dbTeam.PipelineReturns(fakePipeline, true, nil)
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
				resource1 = new(dbngfakes.FakeResource)
				resource1.IDReturns(1)
				resource1.CheckErrorReturns(nil)
				resource1.PausedReturns(true)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")

				resource2 := new(dbngfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.CheckErrorReturns(errors.New("sup"))
				resource2.FailingToCheckReturns(true)
				resource2.PausedReturns(false)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")

				resource3 := new(dbngfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.CheckErrorReturns(nil)
				resource3.PausedReturns(true)
				resource3.PipelineNameReturns("a-pipeline")
				resource3.NameReturns("resource-3")
				resource3.TypeReturns("type-3")

				fakePipeline.ResourcesReturns([]dbng.Resource{
					resource1, resource2, resource3,
				}, nil)

				fakePipeline.ConfigReturns(atc.Config{
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
				}, "", 0, nil)

			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(false)
					userContextReader.GetTeamReturns("", false, false)
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						fakePipeline.PublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						fakePipeline.PublicReturns(true)
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
					userContextReader.GetTeamReturns("a-team", true, true)
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
					Context("when the resources are not found", func() {
						BeforeEach(func() {
							fakePipeline.ResourcesReturns(nil, nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("with an unknown error", func() {
						BeforeEach(func() {
							fakePipeline.ResourcesReturns(nil, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when getting the pipeline config fails", func() {
					BeforeEach(func() {
						fakePipeline.ConfigReturns(atc.Config{}, "", 0, errors.New("fail"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
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
				userContextReader.GetTeamReturns("", false, false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
					resourceName = "resource-1"

					resource1 := new(dbngfakes.FakeResource)
					resource1.CheckErrorReturns(errors.New("sup"))
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.FailingToCheckReturns(true)
					resource1.TypeReturns("type-1")

					fakePipeline.ResourceReturns(resource1, true, nil)
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
				userContextReader.GetTeamReturns("a-team", true, true)
			})

			It("looks it up in the database", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
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
					fakePipeline.ResourceReturns(nil, false, errors.New("Oh no!"))
				})

				It("returns a 500 error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the call to get a resource succeeds", func() {
				BeforeEach(func() {
					resource1 := new(dbngfakes.FakeResource)
					resource1.CheckErrorReturns(errors.New("sup"))
					resource1.PausedReturns(true)
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.FailingToCheckReturns(true)
					resource1.TypeReturns("type-1")

					fakePipeline.ResourceReturns(resource1, true, nil)
					fakePipeline.ConfigReturns(atc.Config{
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
					}, "", 0, nil)
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
				Context("when getting the pipeline config fails", func() {
					BeforeEach(func() {
						fakePipeline.ConfigReturns(atc.Config{}, "", 0, errors.New("fail"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/pause", func() {
		var (
			response     *http.Response
			fakeResource *dbngfakes.FakeResource
		)

		BeforeEach(func() {
			fakeResource = new(dbngfakes.FakeResource)
			fakeResource.NameReturns("resource-name")

			fakePipeline.ResourceReturns(fakeResource, true, nil)
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
				userContextReader.GetTeamReturns("a-team", true, true)
			})

			It("injects the proper pipelineDB", func() {
				pipelineName := dbTeam.PipelineArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					fakeResource.PauseReturns(nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when resource can not be found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when pausing the resource fails", func() {
				BeforeEach(func() {
					fakeResource.PauseReturns(errors.New("welp"))
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
		var (
			response     *http.Response
			fakeResource *dbngfakes.FakeResource
		)

		BeforeEach(func() {
			fakeResource = new(dbngfakes.FakeResource)
			fakeResource.NameReturns("resource-name")

			fakePipeline.ResourceReturns(fakeResource, true, nil)
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
				userContextReader.GetTeamReturns("a-team", true, true)
			})

			It("injects the proper pipelineDB", func() {
				pipelineName := dbTeam.PipelineArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
			})

			Context("when unpausing the resource succeeds", func() {
				BeforeEach(func() {
					fakeResource.UnpauseReturns(nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when resource can not be found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when unpausing the resource fails", func() {
				BeforeEach(func() {
					fakeResource.UnpauseReturns(errors.New("welp"))
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
				userContextReader.GetTeamReturns("a-team", true, true)
			})

			It("injects the proper pipelineDB", func() {
				pipelineName := dbTeam.PipelineArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
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
					returnedVersion := dbng.SavedVersionedResource{
						ID:      4,
						Enabled: true,
						VersionedResource: dbng.VersionedResource{
							Resource: "some-resource",
							Type:     "some-type",
							Version: dbng.ResourceVersion{
								"some": "version",
							},
							Metadata: []dbng.ResourceMetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							},
						},
					}
					fakePipeline.GetLatestVersionedResourceReturns(returnedVersion, true, nil)
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
					fakePipeline.GetLatestVersionedResourceReturns(dbng.SavedVersionedResource{}, false, errors.New("disaster"))
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
					fakeScanner.ScanFromVersionReturns(dbng.ResourceNotFoundError{})
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
