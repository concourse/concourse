package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/jsonapi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/radar/radarfakes"
	"github.com/concourse/concourse/atc/resource"
)

var _ = Describe("Resources API", func() {
	var (
		fakePipeline *dbfakes.FakePipeline
		resource1    *dbfakes.FakeResource
		fakeaccess   = new(accessorfakes.FakeAccess)
		variables    creds.Variables
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
		dbTeam.PipelineReturns(fakePipeline, true, nil)
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
	})

	Describe("GET /api/v1/resources", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/resources")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the dashboard resources succeeds", func() {
			BeforeEach(func() {
				resource1 = new(dbfakes.FakeResource)
				resource1.IDReturns(1)
				resource1.CheckErrorReturns(nil)
				resource1.ResourceConfigCheckErrorReturns(nil)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.TeamNameReturns("some-team")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")
				resource1.LastCheckedReturns(time.Unix(1513364881, 0))

				resource2 := new(dbfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.ResourceConfigCheckErrorReturns(errors.New("sup"))
				resource2.CheckErrorReturns(nil)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.TeamNameReturns("other-team")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")

				resource3 := new(dbfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.CheckErrorReturns(errors.New("sup"))
				resource3.ResourceConfigCheckErrorReturns(nil)
				resource3.PipelineNameReturns("a-pipeline")
				resource3.TeamNameReturns("another-team")
				resource3.NameReturns("resource-3")
				resource3.TypeReturns("type-3")

				dbResourceFactory.VisibleResourcesReturns([]db.Resource{
					resource1, resource2, resource3,
				}, nil)
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns Content-Type 'application/json'", func() {
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns each resource, including their check failure", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[
						{
							"name": "resource-1",
							"pipeline_name": "a-pipeline",
							"team_name": "some-team",
							"type": "type-1",
							"last_checked": 1513364881
						},
						{
							"name": "resource-2",
							"pipeline_name": "a-pipeline",
							"team_name": "other-team",
							"type": "type-2",
							"failing_to_check": true,
							"check_error": "sup"
						},
						{
							"name": "resource-3",
							"pipeline_name": "a-pipeline",
							"team_name": "another-team",
							"type": "type-3",
							"failing_to_check": true,
							"check_setup_error": "sup"
						}
					]`))
			})

			Context("when getting the resource config fails", func() {
				BeforeEach(func() {
					dbResourceFactory.VisibleResourcesReturns(nil, errors.New("nope"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when there are no visible resources", func() {
				BeforeEach(func() {
					dbResourceFactory.VisibleResourcesReturns(nil, nil)
				})

				It("returns empty array", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[]`))
				})
			})

			Context("when not authenticated", func() {
				It("populates resource factory with no team names", func() {
					Expect(dbResourceFactory.VisibleResourcesCallCount()).To(Equal(1))
					Expect(dbResourceFactory.VisibleResourcesArgsForCall(0)).To(BeEmpty())
				})
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeaccess.TeamNamesReturns([]string{"some-team"})
				})

				It("constructs job factory with provided team names", func() {
					Expect(dbResourceFactory.VisibleResourcesCallCount()).To(Equal(1))
					Expect(dbResourceFactory.VisibleResourcesArgsForCall(0)).To(ContainElement("some-team"))
				})
			})
		})
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
				resource1 = new(dbfakes.FakeResource)
				resource1.IDReturns(1)
				resource1.CheckErrorReturns(nil)
				resource1.ResourceConfigCheckErrorReturns(nil)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")
				resource1.LastCheckedReturns(time.Unix(1513364881, 0))

				resource2 := new(dbfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.ResourceConfigCheckErrorReturns(errors.New("sup"))
				resource2.CheckErrorReturns(nil)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")

				resource3 := new(dbfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.ResourceConfigCheckErrorReturns(nil)
				resource3.CheckErrorReturns(errors.New("sup"))
				resource3.PipelineNameReturns("a-pipeline")
				resource3.NameReturns("resource-3")
				resource3.TypeReturns("type-3")

				fakePipeline.ResourcesReturns([]db.Resource{
					resource1, resource2, resource3,
				}, nil)
			})

			Context("when not authenticated and not authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(false)
					fakeaccess.IsAuthorizedReturns(false)
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

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns each resource, excluding their check failure", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
					{
						"name": "resource-1",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-1",
						"last_checked": 1513364881
					},
					{
						"name": "resource-2",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-2",
						"failing_to_check": true
					},
					{
						"name": "resource-3",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-3",
						"failing_to_check": true
					}
				]`))
					})
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("returns each resource, including their check failure", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
						{
							"name": "resource-1",
							"pipeline_name": "a-pipeline",
							"team_name": "a-team",
							"type": "type-1",
							"last_checked": 1513364881
						},
						{
							"name": "resource-2",
							"pipeline_name": "a-pipeline",
							"team_name": "a-team",
							"type": "type-2",
							"failing_to_check": true,
							"check_error": "sup"
						},
						{
							"name": "resource-3",
							"pipeline_name": "a-pipeline",
							"team_name": "a-team",
							"type": "type-3",
							"check_setup_error": "sup",
							"failing_to_check": true
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
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check", func() {
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(dbTeam.PipelineCallCount()).To(Equal(1))
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

			Context("when checking fails with ResourceNotFoundError", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(db.ResourceNotFoundError{})
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when checking the resource fails with ResourceTypeNotFoundError", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(db.ResourceTypeNotFoundError{Name: "missing-type"})
				})

				It("returns jsonapi 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(response.Header.Get("Content-Type")).To(Equal(jsonapi.MediaType))
				})
			})

			Context("when checking the resource fails internally", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					buf := new(bytes.Buffer)
					_, err := buf.ReadFrom(response.Body)
					Expect(err).ToNot(HaveOccurred())
					body := buf.String()
					Expect(body).To(Equal("welp"))
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
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/a-team/pipelines/a-pipeline/resource-types")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the resource types succeeds", func() {
			BeforeEach(func() {
				resourceType1 := new(dbfakes.FakeResourceType)
				resourceType1.IDReturns(1)
				resourceType1.NameReturns("resource-type-1")
				resourceType1.TypeReturns("type-1")
				resourceType1.SourceReturns(map[string]interface{}{"source-key-1": "source-value-1"})
				resourceType1.PrivilegedReturns(false)
				resourceType1.TagsReturns([]string{"tag1"})
				resourceType1.ParamsReturns(map[string]interface{}{"param-key-1": "param-value-1"})
				resourceType1.VersionReturns(map[string]string{
					"version-key-1": "version-value-1",
					"version-key-2": "version-value-2",
				})
				resourceType1.ResourceConfigCheckErrorReturns(nil)
				resourceType1.CheckErrorReturns(nil)

				resourceType2 := new(dbfakes.FakeResourceType)
				resourceType2.IDReturns(2)
				resourceType2.NameReturns("resource-type-2")
				resourceType2.TypeReturns("type-2")
				resourceType2.SourceReturns(map[string]interface{}{"source-key-2": "source-value-2"})
				resourceType2.PrivilegedReturns(true)
				resourceType2.CheckEveryReturns("10ms")
				resourceType2.TagsReturns([]string{"tag1", "tag2"})
				resourceType2.ParamsReturns(map[string]interface{}{"param-key-2": "param-value-2"})
				resourceType2.VersionReturns(map[string]string{
					"version-key-2": "version-value-2",
				})
				resourceType2.ResourceConfigCheckErrorReturns(errors.New("sup"))
				resourceType2.CheckErrorReturns(errors.New("sup"))

				fakePipeline.ResourceTypesReturns(db.ResourceTypes{
					resourceType1, resourceType2,
				}, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(false)
					fakeaccess.IsAuthorizedReturns(false)
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

					It("returns application/json", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns each resource type, excluding the check errors", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
				{
					"name": "resource-type-1",
					"type": "type-1",
					"tags": ["tag1"],
					"privileged": false,
					"params": {"param-key-1": "param-value-1"},
					"source": {"source-key-1": "source-value-1"},
					"version": {
						"version-key-1": "version-value-1",
						"version-key-2": "version-value-2"
					}
				},
				{
					"name": "resource-type-2",
					"type": "type-2",
					"tags": ["tag1", "tag2"],
					"privileged": true,
					"check_every": "10ms",
					"params": {"param-key-2": "param-value-2"},
					"source": {"source-key-2": "source-value-2"},
					"version": {
						"version-key-2": "version-value-2"
					}
				}
			]`))
					})
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns application/json", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("returns each resource type", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
			{
				"name": "resource-type-1",
				"type": "type-1",
				"tags": ["tag1"],
				"privileged": false,
				"params": {"param-key-1": "param-value-1"},
				"source": {"source-key-1": "source-value-1"},
				"version": {
					"version-key-1": "version-value-1",
					"version-key-2": "version-value-2"
				}
			},
			{
				"name": "resource-type-2",
				"type": "type-2",
				"tags": ["tag1", "tag2"],
				"privileged": true,
				"check_every": "10ms",
				"params": {"param-key-2": "param-value-2"},
				"source": {"source-key-2": "source-value-2"},
				"version": {
					"version-key-2": "version-value-2"
				},
				"check_setup_error": "sup",
				"check_error": "sup"
			}
		]`))
				})

				Context("when getting the resource type fails", func() {
					Context("when the resource type are not found", func() {
						BeforeEach(func() {
							fakePipeline.ResourceTypesReturns(nil, nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns application/json", func() {
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
						})
					})

					Context("with an unknown error", func() {
						BeforeEach(func() {
							fakePipeline.ResourceTypesReturns(nil, errors.New("oh no!"))
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

		Context("when not authenticated and not authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
				fakeaccess.IsAuthorizedReturns(false)
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

					resource1 := new(dbfakes.FakeResource)
					resource1.CheckErrorReturns(errors.New("sup"))
					resource1.ResourceConfigCheckErrorReturns(errors.New("sup"))
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.TypeReturns("type-1")
					resource1.LastCheckedReturns(time.Unix(1513364881, 0))

					fakePipeline.ResourceReturns(resource1, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("returns the resource json without the check error", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`
					{
						"name": "resource-1",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-1",
						"last_checked": 1513364881,
						"failing_to_check": true
					}`))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
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
				Context("when the resource version is pinned via pipeline config", func() {
					BeforeEach(func() {
						resource1 := new(dbfakes.FakeResource)
						resource1.CheckErrorReturns(errors.New("sup"))
						resource1.ResourceConfigCheckErrorReturns(errors.New("sup"))
						resource1.PipelineNameReturns("a-pipeline")
						resource1.NameReturns("resource-1")
						resource1.TypeReturns("type-1")
						resource1.LastCheckedReturns(time.Unix(1513364881, 0))
						resource1.ConfigPinnedVersionReturns(atc.Version{"version": "v1"})

						fakePipeline.ResourceReturns(resource1, true, nil)
					})

					It("returns 200 ok", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns the resource json with the check error", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"pipeline_name": "a-pipeline",
								"team_name": "a-team",
								"type": "type-1",
								"last_checked": 1513364881,
								"failing_to_check": true,
								"check_setup_error": "sup",
								"check_error": "sup",
								"pinned_version": {"version": "v1"},
								"pinned_in_config": true
							}`))
					})
				})
				Context("when the resource version is pinned via the API", func() {
					BeforeEach(func() {
						resource1 := new(dbfakes.FakeResource)
						resource1.PipelineNameReturns("a-pipeline")
						resource1.NameReturns("resource-1")
						resource1.TypeReturns("type-1")
						resource1.LastCheckedReturns(time.Unix(1513364881, 0))
						resource1.APIPinnedVersionReturns(atc.Version{"version": "v1"})

						fakePipeline.ResourceReturns(resource1, true, nil)
					})

					It("returns 200 ok", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns the resource json describing the pinned version", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"pipeline_name": "a-pipeline",
								"team_name": "a-team",
								"type": "type-1",
								"last_checked": 1513364881,
								"pinned_version": {"version": "v1"}
							}`))
					})
				})
			})
		})

		Context("when authenticated but not authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
					resourceName = "resource-1"

					resource1 := new(dbfakes.FakeResource)
					resource1.CheckErrorReturns(errors.New("sup"))
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.TypeReturns("type-1")
					resource1.LastCheckedReturns(time.Unix(1513364881, 0))

					fakePipeline.ResourceReturns(resource1, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
				})

				It("returns the resource json without the check error", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`
					{
						"name": "resource-1",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-1",
						"last_checked": 1513364881,
						"failing_to_check": true
					}`))
				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types/:resource_type_name/check", func() {
		var checkRequestBody atc.CheckRequestBody
		var response *http.Response

		BeforeEach(func() {
			checkRequestBody = atc.CheckRequestBody{}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(checkRequestBody)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resource-types/resource-type-name/check", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(false)
			})

			It("returns Forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when authenticated and authorized", func() {
			var fakeScanner *radarfakes.FakeScanner

			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)

				fakeScanner = new(radarfakes.FakeScanner)
				fakeScannerFactory.NewResourceTypeScannerReturns(fakeScanner)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("calls Scan", func() {
				Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
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
					Expect(actualResourceName).To(Equal("resource-type-name"))
					Expect(actualFromVersion).To(Equal(checkRequestBody.From))
				})
			})

			Context("when resource type checking fails with ResourceNotFoundError", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(db.ResourceTypeNotFoundError{})
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when resource type fails with unexpected error", func() {
				BeforeEach(func() {
					err := errors.New("some-error")
					fakeScanner.ScanFromVersionReturns(err)
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check/webhook", func() {
		var (
			fakeScanner               *radarfakes.FakeScanner
			checkRequestBody          atc.CheckRequestBody
			response                  *http.Response
			fakeResource              *dbfakes.FakeResource
			fakeResourceConfig        *dbfakes.FakeResourceConfig
			fakeResourceConfigVersion *dbfakes.FakeResourceConfigVersion
		)

		ExpectResourceWebhookScanErrorsToBeHandled := func() {
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

			Context("when checking the resource fails with err", func() {
				BeforeEach(func() {
					fakeScanner.ScanFromVersionReturns(errors.New("error"))
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		}

		BeforeEach(func() {
			fakeScanner = new(radarfakes.FakeScanner)
			fakeScannerFactory.NewResourceScannerReturns(fakeScanner)
			checkRequestBody = atc.CheckRequestBody{}

			fakeResource = new(dbfakes.FakeResource)
			fakeResource.NameReturns("resource-name")
			fakeResourceConfig = new(dbfakes.FakeResourceConfig)
			fakeResourceConfigVersion = new(dbfakes.FakeResourceConfigVersion)
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(checkRequestBody)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/check/webhook?webhook_token=fake-token", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				variables = template.StaticVariables{
					"webhook-token": "fake-token",
				}
				token, err := creds.NewString(variables, "((webhook-token))").Evaluate()
				Expect(err).NotTo(HaveOccurred())
				fakeResource.WebhookTokenReturns(token)
				fakePipeline.ResourceReturns(fakeResource, true, nil)
				fakeResource.ResourceConfigIDReturns(1)
			})

			It("injects the proper pipelineDB", func() {
				Expect(dbTeam.PipelineCallCount()).To(Equal(1))
				resourceName := fakePipeline.ResourceArgsForCall(0)
				Expect(resourceName).To(Equal("resource-name"))
			})

			It("tries to find the resource", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("resource-name"))
			})

			Context("when finding the resource succeeds", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				It("tries to find the resource config using the resource config id", func() {
					Expect(dbResourceConfigFactory.FindResourceConfigByIDCallCount()).To(Equal(1))
					Expect(dbResourceConfigFactory.FindResourceConfigByIDArgsForCall(0)).To(Equal(1))
				})

				Context("when finding the resource config succeeds", func() {
					BeforeEach(func() {
						dbResourceConfigFactory.FindResourceConfigByIDReturns(fakeResourceConfig, true, nil)
					})

					Context("when the latest version is found", func() {
						BeforeEach(func() {
							fakeResourceConfigVersion.IDReturns(4)
							fakeResourceConfigVersion.VersionReturns(db.Version{"some": "version"})
							fakeResourceConfigVersion.MetadataReturns([]db.ResourceConfigMetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							})

							fakeResourceConfig.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
						})

						It("tries to scan with the latest version", func() {
							Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
							_, actualResourceName, actualFromVersion := fakeScanner.ScanFromVersionArgsForCall(0)
							Expect(actualResourceName).To(Equal("resource-name"))
							Expect(actualFromVersion).To(Equal(atc.Version{"some": "version"}))
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						ExpectResourceWebhookScanErrorsToBeHandled()
					})

					Context("when the latest version is not found", func() {
						BeforeEach(func() {
							fakeResourceConfig.LatestVersionReturns(nil, false, nil)
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

						ExpectResourceWebhookScanErrorsToBeHandled()
					})

					Context("when failing to get latest version for resource", func() {
						BeforeEach(func() {
							fakeResourceConfig.LatestVersionReturns(nil, false, errors.New("disaster"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})

						It("does not scan from version", func() {
							Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(0))
						})
					})
				})

				Context("when finding the resource config fails", func() {
					BeforeEach(func() {
						dbResourceConfigFactory.FindResourceConfigByIDReturns(nil, false, errors.New("oops"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the resource config is not found", func() {
					BeforeEach(func() {
						dbResourceConfigFactory.FindResourceConfigByIDReturns(nil, false, nil)
					})

					It("tries to scan", func() {
						Expect(fakeScanner.ScanFromVersionCallCount()).To(Equal(1))
					})
				})
			})

			Context("when finding the resource fails", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, errors.New("oops"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the resource is not found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

		})

		Context("when unauthorized", func() {
			BeforeEach(func() {
				fakeResource.WebhookTokenReturns("wrong-token")
				fakePipeline.ResourceReturns(fakeResource, true, nil)
			})
			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
