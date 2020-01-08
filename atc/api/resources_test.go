package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("Resources API", func() {
	var (
		fakePipeline *dbfakes.FakePipeline
		resource1    *dbfakes.FakeResource
		variables    vars.Variables
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
		dbTeam.PipelineReturns(fakePipeline, true, nil)
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
				resource1.CheckSetupErrorReturns(nil)
				resource1.CheckErrorReturns(nil)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.TeamNameReturns("some-team")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")
				resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))

				resource2 := new(dbfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.CheckErrorReturns(errors.New("sup"))
				resource2.CheckSetupErrorReturns(nil)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.TeamNameReturns("other-team")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")

				resource3 := new(dbfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.CheckSetupErrorReturns(errors.New("sup"))
				resource3.CheckErrorReturns(nil)
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
				expectedHeaderEntries := map[string]string{
					"Content-Type": "application/json",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
					fakeAccess.TeamNamesReturns([]string{"some-team"})
				})

				It("constructs job factory with provided team names", func() {
					Expect(dbResourceFactory.VisibleResourcesCallCount()).To(Equal(1))
					Expect(dbResourceFactory.VisibleResourcesArgsForCall(0)).To(ContainElement("some-team"))
				})

				Context("when user has admin privilege", func() {
					BeforeEach(func() {
						fakeAccess.IsAdminReturns(true)
					})

					It("returns all resources", func() {
						Expect(dbResourceFactory.AllResourcesCallCount()).To(Equal(1))
					})
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
				resource1.CheckSetupErrorReturns(nil)
				resource1.CheckErrorReturns(nil)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")
				resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))

				resource2 := new(dbfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.CheckErrorReturns(errors.New("sup"))
				resource2.CheckSetupErrorReturns(nil)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")

				resource3 := new(dbfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.CheckErrorReturns(nil)
				resource3.CheckSetupErrorReturns(errors.New("sup"))
				resource3.PipelineNameReturns("a-pipeline")
				resource3.NameReturns("resource-3")
				resource3.TypeReturns("type-3")

				fakePipeline.ResourcesReturns([]db.Resource{
					resource1, resource2, resource3,
				}, nil)
			})

			Context("when not authenticated and not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
					fakeAccess.IsAuthorizedReturns(false)
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
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/unpin", func() {
		var response *http.Response
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/unpin", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated ", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("tries to find the resource", func() {
					resourceName := fakePipeline.ResourceArgsForCall(0)
					Expect(resourceName).To(Equal("resource-name"))
				})

				Context("when finding the resource succeeds", func() {
					BeforeEach(func() {
						fakeResource = new(dbfakes.FakeResource)
						fakeResource.IDReturns(1)
						fakePipeline.ResourceReturns(fakeResource, true, nil)
					})

					Context("when unpinning the resource version succeeds", func() {
						BeforeEach(func() {
							fakeResource.UnpinVersionReturns(nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when unpinning the resource fails", func() {
						BeforeEach(func() {
							fakeResource.UnpinVersionReturns(errors.New("welp"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when it fails to find the resource", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, errors.New("welp"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the resource is not found", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, nil)
					})

					It("returns not found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})
			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})
		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/pin_comment", func() {
		var response *http.Response
		var pinCommentRequestBody atc.SetPinCommentRequestBody
		var fakeResource *dbfakes.FakeResource

		BeforeEach(func() {
			pinCommentRequestBody = atc.SetPinCommentRequestBody{}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(pinCommentRequestBody)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/pin_comment", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated ", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("tries to find the resource", func() {
					resourceName := fakePipeline.ResourceArgsForCall(0)
					Expect(resourceName).To(Equal("resource-name"))
				})

				Context("when finding the resource succeeds", func() {
					BeforeEach(func() {
						fakeResource = new(dbfakes.FakeResource)
						fakeResource.IDReturns(1)
						fakePipeline.ResourceReturns(fakeResource, true, nil)
						pinCommentRequestBody.PinComment = "I am a pin comment"
					})

					It("Tries to set the pin comment", func() {
						Expect(fakeResource.SetPinCommentCallCount()).To(Equal(1))
						comment := fakeResource.SetPinCommentArgsForCall(0)
						Expect(comment).To(Equal("I am a pin comment"))
					})

					Context("when setting the pin comment succeeds", func() {
						BeforeEach(func() {
							fakeResource.SetPinCommentReturns(nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when setting the pin comment fails", func() {
						BeforeEach(func() {
							fakeResource.SetPinCommentReturns(errors.New("welp"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when it fails to find the resource", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, errors.New("welp"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the resource is not found", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, nil)
					})

					It("returns not found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})
			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check", func() {
		var checkRequestBody atc.CheckRequestBody
		var response *http.Response

		BeforeEach(func() {
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
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when looking up the resource fails", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, errors.New("nope"))
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

			Context("when it finds the resource", func() {
				var fakeResource *dbfakes.FakeResource

				BeforeEach(func() {
					fakeResource = new(dbfakes.FakeResource)
					fakeResource.IDReturns(1)
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				Context("when looking up the resource types fails", func() {
					BeforeEach(func() {
						fakePipeline.ResourceTypesReturns(nil, errors.New("nope"))
					})
					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when looking up the resource types succeeds", func() {
					var fakeResourceTypes db.ResourceTypes

					BeforeEach(func() {
						fakeResourceTypes = db.ResourceTypes{}
						fakePipeline.ResourceTypesReturns(fakeResourceTypes, nil)
					})

					It("checks with no version specified", func() {
						Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
						_, actualResource, actualResourceTypes, actualFromVersion, manuallyTriggered := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualResource).To(Equal(fakeResource))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
					})

					Context("when checking with a version specified", func() {
						BeforeEach(func() {
							checkRequestBody = atc.CheckRequestBody{
								From: atc.Version{
									"some-version-key": "some-version-value",
								},
							}
						})

						It("checks with the version specified", func() {
							Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
							_, actualResource, actualResourceTypes, actualFromVersion, manuallyTriggered := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(actualResource).To(Equal(fakeResource))
							Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
							Expect(actualFromVersion).To(Equal(checkRequestBody.From))
							Expect(manuallyTriggered).To(BeTrue())
						})
					})

					Context("when checking fails", func() {
						BeforeEach(func() {
							dbCheckFactory.TryCreateCheckReturns(nil, false, errors.New("nope"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when checking does not create a new check", func() {
						BeforeEach(func() {
							dbCheckFactory.TryCreateCheckReturns(nil, false, nil)
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when checking creates a new check", func() {
						var fakeCheck *dbfakes.FakeCheck

						BeforeEach(func() {
							fakeCheck = new(dbfakes.FakeCheck)
							fakeCheck.IDReturns(10)
							fakeCheck.StatusReturns("started")
							fakeCheck.CreateTimeReturns(time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeCheck.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeCheck.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeCheck, true, nil)
						})

						It("notify checker", func() {
							Expect(dbCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
                 "id": 10,
								 "status": "started",
								 "create_time": 946684800,
								 "start_time": 978307200,
								 "end_time": 1009843200
							}`))
						})
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
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
				resourceType1.CheckErrorReturns(nil)
				resourceType1.CheckSetupErrorReturns(nil)
				resourceType1.UniqueVersionHistoryReturns(true)

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
				resourceType2.CheckErrorReturns(errors.New("sup"))
				resourceType2.CheckSetupErrorReturns(errors.New("sup"))

				fakePipeline.ResourceTypesReturns(db.ResourceTypes{
					resourceType1, resourceType2,
				}, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
					fakeAccess.IsAuthorizedReturns(false)
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
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns each resource type, excluding the check errors", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
				{
					"name": "resource-type-1",
					"type": "type-1",
					"tags": ["tag1"],
					"params": {"param-key-1": "param-value-1"},
					"source": {"source-key-1": "source-value-1"},
					"unique_version_history": true,
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
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns application/json", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				It("returns each resource type", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
			{
				"name": "resource-type-1",
				"type": "type-1",
				"tags": ["tag1"],
				"params": {"param-key-1": "param-value-1"},
				"source": {"source-key-1": "source-value-1"},
				"unique_version_history": true,
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
							expectedHeaderEntries := map[string]string{
								"Content-Type": "application/json",
							}
							Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
				fakeAccess.IsAuthenticatedReturns(false)
				fakeAccess.IsAuthorizedReturns(false)
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
					resource1.CheckSetupErrorReturns(errors.New("sup"))
					resource1.CheckErrorReturns(errors.New("sup"))
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.TypeReturns("type-1")
					resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))

					fakePipeline.ResourceReturns(resource1, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
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
						resource1.CheckSetupErrorReturns(errors.New("sup"))
						resource1.CheckErrorReturns(errors.New("sup"))
						resource1.PipelineNameReturns("a-pipeline")
						resource1.NameReturns("resource-1")
						resource1.TypeReturns("type-1")
						resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))
						resource1.ConfigPinnedVersionReturns(atc.Version{"version": "v1"})

						fakePipeline.ResourceReturns(resource1, true, nil)
					})

					It("returns 200 ok", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
						resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))
						resource1.APIPinnedVersionReturns(atc.Version{"version": "v1"})

						fakePipeline.ResourceReturns(resource1, true, nil)
					})

					It("returns 200 ok", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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

				Context("when the resource has a pin comment", func() {
					BeforeEach(func() {
						resource1 := new(dbfakes.FakeResource)
						resource1.PipelineNameReturns("a-pipeline")
						resource1.NameReturns("resource-1")
						resource1.TypeReturns("type-1")
						resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))
						resource1.APIPinnedVersionReturns(atc.Version{"version": "v1"})
						resource1.PinCommentReturns("a pin comment")
						fakePipeline.ResourceReturns(resource1, true, nil)
					})

					It("returns the pin comment in the response json", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"pipeline_name": "a-pipeline",
								"team_name": "a-team",
								"type": "type-1",
								"last_checked": 1513364881,
								"pinned_version": {"version": "v1"},
								"pin_comment": "a pin comment"
							}`))
					})
				})
			})
		})

		Context("when authenticated but not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(false)
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
					resource1.CheckSetupErrorReturns(errors.New("sup"))
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.TypeReturns("type-1")
					resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))

					fakePipeline.ResourceReturns(resource1, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
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
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(false)
			})

			It("returns Forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when authenticated and authorized", func() {

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when looking up the resource type fails", func() {
				BeforeEach(func() {
					fakePipeline.ResourceTypeReturns(nil, false, errors.New("nope"))
				})
				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the resource type is not found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceTypeReturns(nil, false, nil)
				})
				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when it finds the resource type", func() {
				var fakeResourceType *dbfakes.FakeResourceType

				BeforeEach(func() {
					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(1)
					fakePipeline.ResourceTypeReturns(fakeResourceType, true, nil)
				})

				Context("when looking up the resource types fails", func() {
					BeforeEach(func() {
						fakePipeline.ResourceTypesReturns(nil, errors.New("nope"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when looking up the resource types succeeds", func() {
					var fakeResourceTypes db.ResourceTypes

					BeforeEach(func() {
						fakeResourceTypes = db.ResourceTypes{}
						fakePipeline.ResourceTypesReturns(fakeResourceTypes, nil)
					})

					It("checks with no version specified", func() {
						Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
						_, actualResourceType, actualResourceTypes, actualFromVersion, manuallyTriggered := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualResourceType).To(Equal(fakeResourceType))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
					})

					Context("when checking with a version specified", func() {
						BeforeEach(func() {
							checkRequestBody = atc.CheckRequestBody{
								From: atc.Version{
									"some-version-key": "some-version-value",
								},
							}
						})

						It("checks with no version specified", func() {
							Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
							_, actualResourceType, actualResourceTypes, actualFromVersion, manuallyTriggered := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(actualResourceType).To(Equal(fakeResourceType))
							Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
							Expect(actualFromVersion).To(Equal(checkRequestBody.From))
							Expect(manuallyTriggered).To(BeTrue())
						})
					})

					Context("when checking fails", func() {
						BeforeEach(func() {
							dbCheckFactory.TryCreateCheckReturns(nil, false, errors.New("nope"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when checking does not create a new check", func() {
						BeforeEach(func() {
							dbCheckFactory.TryCreateCheckReturns(nil, false, nil)
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when checking creates a new check", func() {
						var fakeCheck *dbfakes.FakeCheck

						BeforeEach(func() {
							fakeCheck = new(dbfakes.FakeCheck)
							fakeCheck.IDReturns(10)
							fakeCheck.StatusReturns("started")
							fakeCheck.CreateTimeReturns(time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeCheck.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeCheck.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeCheck, true, nil)
						})

						It("notify checker", func() {
							Expect(dbCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
                 "id": 10,
								 "status": "started",
								 "create_time": 946684800,
								 "start_time": 978307200,
								 "end_time": 1009843200
							}`))
						})
					})

				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check/webhook", func() {
		var (
			checkRequestBody atc.CheckRequestBody
			response         *http.Response
			fakeResource     *dbfakes.FakeResource
		)

		BeforeEach(func() {
			checkRequestBody = atc.CheckRequestBody{}

			fakeResource = new(dbfakes.FakeResource)
			fakeResource.NameReturns("resource-name")
			fakeResource.IDReturns(10)
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
				variables = vars.StaticVariables{
					"webhook-token": "fake-token",
				}
				token, err := creds.NewString(variables, "((webhook-token))").Evaluate()
				Expect(err).NotTo(HaveOccurred())
				fakeResource.WebhookTokenReturns(token)
				fakePipeline.ResourceReturns(fakeResource, true, nil)
				fakeResource.ResourceConfigIDReturns(1)
				fakeResource.ResourceConfigScopeIDReturns(2)
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

				Context("when finding the resource types fails", func() {
					BeforeEach(func() {
						fakePipeline.ResourceTypesReturns(nil, errors.New("oops"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when finding the resource types succeeds", func() {
					var fakeResourceTypes db.ResourceTypes

					BeforeEach(func() {
						fakeResourceTypes = db.ResourceTypes{}
						fakePipeline.ResourceTypesReturns(fakeResourceTypes, nil)
					})

					It("checks with a nil version", func() {
						Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
						_, actualResource, actualResourceTypes, actualFromVersion, manuallyTriggered := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualResource).To(Equal(fakeResource))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
					})

					Context("when checking fails", func() {
						BeforeEach(func() {
							dbCheckFactory.TryCreateCheckReturns(nil, false, errors.New("nope"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when checking does not create a new check", func() {
						BeforeEach(func() {
							dbCheckFactory.TryCreateCheckReturns(nil, false, nil)
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when checking creates a new check", func() {
						var fakeCheck *dbfakes.FakeCheck

						BeforeEach(func() {
							fakeCheck = new(dbfakes.FakeCheck)
							fakeCheck.IDReturns(10)
							fakeCheck.StatusReturns("started")
							fakeCheck.CreateTimeReturns(time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeCheck.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeCheck.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeCheck, true, nil)
						})

						It("notify checker", func() {
							Expect(dbCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
                 "id": 10,
								 "status": "started",
								 "create_time": 946684800,
								 "start_time": 978307200,
								 "end_time": 1009843200
							}`))
						})
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
