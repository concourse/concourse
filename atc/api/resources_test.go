package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
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
				resource1.PipelineIDReturns(1)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.TeamNameReturns("some-team")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")
				resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))

				resource2 := new(dbfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.PipelineIDReturns(1)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.TeamNameReturns("other-team")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")
				resource2.BuildSummaryReturns(&atc.BuildSummary{
					ID:                   123,
					Name:                 "123",
					Status:               atc.StatusSucceeded,
					StartTime:            456,
					EndTime:              789,
					TeamName:             "some-team",
					PipelineID:           99,
					PipelineName:         "some-pipeline",
					PipelineInstanceVars: atc.InstanceVars{"foo": 1},
				})

				resource3 := new(dbfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.TeamNameReturns("some-team")
				resource3.PipelineIDReturns(2)
				resource3.PipelineNameReturns("some-pipeline")
				resource3.PipelineInstanceVarsReturns(atc.InstanceVars{"branch": "master"})
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

			It("returns each resource, including their build", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[
						{
							"name": "resource-1",
							"pipeline_id": 1,
							"pipeline_name": "a-pipeline",
							"team_name": "some-team",
							"type": "type-1",
							"last_checked": 1513364881
						},
						{
							"name": "resource-2",
							"pipeline_id": 1,
							"pipeline_name": "a-pipeline",
							"team_name": "other-team",
							"type": "type-2",
							"build": {
                "id": 123,
								"name": "123",
                "status": "succeeded",
                "start_time": 456,
                "end_time": 789,
                "team_name": "some-team",
                "pipeline_id": 99,
                "pipeline_name": "some-pipeline",
                "pipeline_instance_vars": {
                  "foo": 1
                }
							}
						},
						{
							"name": "resource-3",
							"pipeline_id": 2,
							"pipeline_name": "some-pipeline",
							"pipeline_instance_vars": {
								"branch": "master"
							},
							"team_name": "another-team",
							"type": "type-3"
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
				resource1.TeamNameReturns("a-team")
				resource1.PipelineIDReturns(1)
				resource1.PipelineNameReturns("a-pipeline")
				resource1.NameReturns("resource-1")
				resource1.TypeReturns("type-1")
				resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))

				resource2 := new(dbfakes.FakeResource)
				resource2.IDReturns(2)
				resource2.TeamNameReturns("a-team")
				resource2.PipelineIDReturns(1)
				resource2.PipelineNameReturns("a-pipeline")
				resource2.NameReturns("resource-2")
				resource2.TypeReturns("type-2")

				resource3 := new(dbfakes.FakeResource)
				resource3.IDReturns(3)
				resource3.TeamNameReturns("a-team")
				resource3.PipelineIDReturns(2)
				resource3.PipelineNameReturns("some-pipeline")
				resource3.PipelineInstanceVarsReturns(atc.InstanceVars{"branch": "master"})
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

					It("returns each resource", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
					{
						"name": "resource-1",
						"pipeline_id": 1,
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-1",
						"last_checked": 1513364881
					},
					{
						"name": "resource-2",
						"pipeline_id": 1,
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-2"
					},
					{
						"name": "resource-3",
						"pipeline_id": 2,
						"pipeline_name": "some-pipeline",
						"pipeline_instance_vars": {
							"branch": "master"
						},
						"team_name": "a-team",
						"type": "type-3"
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

				It("returns each resource", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
						{
							"name": "resource-1",
							"pipeline_id": 1,
							"pipeline_name": "a-pipeline",
							"team_name": "a-team",
							"type": "type-1",
							"last_checked": 1513364881
						},
						{
							"name": "resource-2",
							"pipeline_id": 1,
							"pipeline_name": "a-pipeline",
							"team_name": "a-team",
							"type": "type-2"
						},
						{
							"name": "resource-3",
							"pipeline_id": 2,
							"pipeline_name": "some-pipeline",
							"pipeline_instance_vars": {
								"branch": "master"
							},
							"team_name": "a-team",
							"type": "type-3"
						}
					]`))
				})

				Context("when the pipeline has no resources", func() {
					BeforeEach(func() {
						fakePipeline.ResourcesReturns(nil, nil)
					})

					It("returns an empty list", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[]`))
					})
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
						_, actualResource, actualResourceTypes, actualFromVersion, manuallyTriggered, skipIntervalRecursively, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualResource).To(Equal(fakeResource))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
						Expect(skipIntervalRecursively).To(BeTrue())
						Expect(toDb).To(BeTrue())
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
							_, actualResource, actualResourceTypes, actualFromVersion, manuallyTriggered, skipIntervalRecursively, toDB := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(actualResource).To(Equal(fakeResource))
							Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
							Expect(actualFromVersion).To(Equal(checkRequestBody.From))
							Expect(manuallyTriggered).To(BeTrue())
							Expect(skipIntervalRecursively).To(BeTrue())
							Expect(toDB).To(BeTrue())
						})
					})

					Context("when doing a shallow check", func() {
						BeforeEach(func() {
							checkRequestBody.Shallow = true
						})

						It("does not recursively skip the check interval", func() {
							_, _, _, _, _, skipIntervalRecursively, _ := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(skipIntervalRecursively).To(BeFalse())
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
						var fakeBuild *dbfakes.FakeBuild

						BeforeEach(func() {
							fakeBuild = new(dbfakes.FakeBuild)
							fakeBuild.IDReturns(10)
							fakeBuild.NameReturns("some-name")
							fakeBuild.TeamNameReturns("some-team")
							fakeBuild.StatusReturns("started")
							fakeBuild.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeBuild.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeBuild, true, nil)
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
								"id": 10,
								"name": "some-name",
								"team_name": "some-team",
								"status": "started",
								"api_url": "/api/v1/builds/10",
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

				resourceType2 := new(dbfakes.FakeResourceType)
				resourceType2.IDReturns(2)
				resourceType2.NameReturns("resource-type-2")
				resourceType2.TypeReturns("type-2")
				resourceType2.SourceReturns(map[string]interface{}{"source-key-2": "source-value-2"})
				resourceType2.PrivilegedReturns(true)
				resourceType2.CheckEveryReturns(&atc.CheckEvery{Interval: 10 * time.Millisecond})
				resourceType2.TagsReturns([]string{"tag1", "tag2"})
				resourceType2.ParamsReturns(map[string]interface{}{"param-key-2": "param-value-2"})

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

					It("returns each resource type", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
				{
					"name": "resource-type-1",
					"type": "type-1",
					"tags": ["tag1"],
					"params": {"param-key-1": "param-value-1"},
					"source": {"source-key-1": "source-value-1"}
				},
				{
					"name": "resource-type-2",
					"type": "type-2",
					"tags": ["tag1", "tag2"],
					"privileged": true,
					"check_every": "10ms",
					"params": {"param-key-2": "param-value-2"},
					"source": {"source-key-2": "source-value-2"}
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
				"source": {"source-key-1": "source-value-1"}
			},
			{
				"name": "resource-type-2",
				"type": "type-2",
				"tags": ["tag1", "tag2"],
				"privileged": true,
				"check_every": "10ms",
				"params": {"param-key-2": "param-value-2"},
				"source": {"source-key-2": "source-value-2"}
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
					resource1.TeamNameReturns("a-team")
					resource1.PipelineIDReturns(1)
					resource1.PipelineNameReturns("a-pipeline")
					resource1.NameReturns("resource-1")
					resource1.TypeReturns("type-1")
					resource1.LastCheckEndTimeReturns(time.Unix(1513364881, 0))
					resource1.BuildSummaryReturns(&atc.BuildSummary{
						ID:                   123,
						Name:                 "123",
						Status:               atc.StatusSucceeded,
						StartTime:            456,
						EndTime:              789,
						TeamName:             "some-team",
						PipelineID:           99,
						PipelineName:         "some-pipeline",
						PipelineInstanceVars: atc.InstanceVars{"foo": 1},
					})

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

				It("returns the resource json", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`
					{
						"name": "resource-1",
						"pipeline_id": 1,
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-1",
						"last_checked": 1513364881,
						"build": {
							"id": 123,
							"name": "123",
							"status": "succeeded",
							"start_time": 456,
							"end_time": 789,
							"team_name": "some-team",
							"pipeline_id": 99,
							"pipeline_name": "some-pipeline",
							"pipeline_instance_vars": {
								"foo": 1
							}
						}
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
						resource1.TeamNameReturns("a-team")
						resource1.PipelineIDReturns(1)
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

					It("returns the resource json", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`
							{
								"name": "resource-1",
								"pipeline_id": 1,
								"pipeline_name": "a-pipeline",
								"team_name": "a-team",
								"type": "type-1",
								"last_checked": 1513364881,
								"pinned_version": {"version": "v1"},
								"pinned_in_config": true
							}`))
					})
				})

				Context("when the resource version is pinned via the API", func() {
					BeforeEach(func() {
						resource1 := new(dbfakes.FakeResource)
						resource1.TeamNameReturns("a-team")
						resource1.PipelineIDReturns(1)
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
								"pipeline_id": 1,
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
						resource1.TeamNameReturns("a-team")
						resource1.PipelineIDReturns(1)
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
								"pipeline_id": 1,
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
					resource1.TeamNameReturns("a-team")
					resource1.PipelineIDReturns(1)
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

				It("returns the resource json", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`
					{
						"name": "resource-1",
						"pipeline_id": 1,
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"type": "type-1",
						"last_checked": 1513364881
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
						_, actualResourceType, actualResourceTypes, actualFromVersion, manuallyTriggered, skipIntervalRecursively, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualResourceType).To(Equal(fakeResourceType))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
						Expect(skipIntervalRecursively).To(BeTrue())
						Expect(toDb).To(BeTrue())
					})

					Context("when doing a shallow check", func() {
						BeforeEach(func() {
							checkRequestBody.Shallow = true
						})

						It("does not recursively skip the check interval", func() {
							_, _, _, _, _, skipIntervalRecursively, _ := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(skipIntervalRecursively).To(BeFalse())
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

						It("checks with the given version specified", func() {
							Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
							_, actualResourceType, actualResourceTypes, actualFromVersion, manuallyTriggered, _, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(actualResourceType).To(Equal(fakeResourceType))
							Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
							Expect(actualFromVersion).To(Equal(checkRequestBody.From))
							Expect(manuallyTriggered).To(BeTrue())
							Expect(toDb).To(BeTrue())
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
						var fakeBuild *dbfakes.FakeBuild

						BeforeEach(func() {
							fakeBuild = new(dbfakes.FakeBuild)
							fakeBuild.IDReturns(10)
							fakeBuild.NameReturns("some-name")
							fakeBuild.TeamNameReturns("some-team")
							fakeBuild.StatusReturns("started")
							fakeBuild.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeBuild.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeBuild, true, nil)
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
                 "id": 10,
								 "name": "some-name",
								 "team_name": "some-team",
								 "status": "started",
								 "api_url": "/api/v1/builds/10",
								 "start_time": 978307200,
								 "end_time": 1009843200
							}`))
						})
					})

				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/prototypes/:prototype_name/check", func() {
		var checkRequestBody atc.CheckRequestBody
		var response *http.Response

		BeforeEach(func() {
			checkRequestBody = atc.CheckRequestBody{}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(checkRequestBody)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/prototypes/prototype-name/check", bytes.NewBuffer(reqPayload))
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
					fakePipeline.PrototypeReturns(nil, false, errors.New("nope"))
				})
				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the prototype is not found", func() {
				BeforeEach(func() {
					fakePipeline.PrototypeReturns(nil, false, nil)
				})
				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when it finds the prototype", func() {
				var fakePrototype *dbfakes.FakePrototype

				BeforeEach(func() {
					fakePrototype = new(dbfakes.FakePrototype)
					fakePrototype.IDReturns(1)
					fakePipeline.PrototypeReturns(fakePrototype, true, nil)
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
						_, actualPrototype, actualResourceTypes, actualFromVersion, manuallyTriggered, skipIntervalRecursively, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualPrototype).To(Equal(fakePrototype))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
						Expect(skipIntervalRecursively).To(BeTrue())
						Expect(toDb).To(BeTrue())
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
							_, actualPrototype, actualResourceTypes, actualFromVersion, manuallyTriggered, skipIntervalRecursively, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(actualPrototype).To(Equal(fakePrototype))
							Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
							Expect(actualFromVersion).To(Equal(checkRequestBody.From))
							Expect(manuallyTriggered).To(BeTrue())
							Expect(skipIntervalRecursively).To(BeTrue())
							Expect(toDb).To(BeTrue())
						})
					})

					Context("when doing a shallow check", func() {
						BeforeEach(func() {
							checkRequestBody.Shallow = true
						})

						It("does not recursively skip the check interval", func() {
							_, _, _, _, _, skipIntervalRecursively, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(skipIntervalRecursively).To(BeFalse())
							Expect(toDb).To(BeTrue())
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
						var fakeBuild *dbfakes.FakeBuild

						BeforeEach(func() {
							fakeBuild = new(dbfakes.FakeBuild)
							fakeBuild.IDReturns(10)
							fakeBuild.NameReturns("some-name")
							fakeBuild.TeamNameReturns("some-team")
							fakeBuild.StatusReturns("started")
							fakeBuild.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeBuild.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeBuild, true, nil)
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
                 "id": 10,
								 "name": "some-name",
								 "team_name": "some-team",
								 "status": "started",
								 "api_url": "/api/v1/builds/10",
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
						_, actualResource, actualResourceTypes, actualFromVersion, manuallyTriggered, skipIntervalRecursively, toDb := dbCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(actualResource).To(Equal(fakeResource))
						Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
						Expect(actualFromVersion).To(BeNil())
						Expect(manuallyTriggered).To(BeTrue())
						Expect(skipIntervalRecursively).To(BeFalse())
						Expect(toDb).To(BeTrue())
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
						var fakeBuild *dbfakes.FakeBuild

						BeforeEach(func() {
							fakeBuild = new(dbfakes.FakeBuild)
							fakeBuild.IDReturns(10)
							fakeBuild.NameReturns("some-name")
							fakeBuild.TeamNameReturns("some-team")
							fakeBuild.StatusReturns("started")
							fakeBuild.StartTimeReturns(time.Date(2001, 01, 01, 0, 0, 0, 0, time.UTC))
							fakeBuild.EndTimeReturns(time.Date(2002, 01, 01, 0, 0, 0, 0, time.UTC))

							dbCheckFactory.TryCreateCheckReturns(fakeBuild, true, nil)
						})

						It("returns 201", func() {
							Expect(response.StatusCode).To(Equal(http.StatusCreated))
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
                 "id": 10,
								 "name": "some-name",
								 "team_name": "some-team",
								 "status": "started",
								 "api_url": "/api/v1/builds/10",
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

	Describe("DELETE /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/cache", func() {
		var (
			versionDeleteBody atc.VersionDeleteBody
			response          *http.Response
			fakeResource      *dbfakes.FakeResource
		)

		executeConnection := func() {
			reqPayload, err := json.Marshal(versionDeleteBody)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("DELETE", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/cache", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		}

		BeforeEach(func() {
			versionDeleteBody = atc.VersionDeleteBody{}
		})

		JustBeforeEach(func() {
			executeConnection()
		})

		Context("when authenticated ", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
				})

				Context("when it tries to find a resource", func() {
					It("calls the resource", func() {
						resourceName := fakePipeline.ResourceArgsForCall(0)
						Expect(resourceName).To(Equal("resource-name"))
					})
				})

				Context("when finding the resource succeeds", func() {
					BeforeEach(func() {
						fakeResource = new(dbfakes.FakeResource)
						fakeResource.IDReturns(1)
						fakePipeline.ResourceReturns(fakeResource, true, nil)
					})

					Context("when clear cache succeeds", func() {
						BeforeEach(func() {
							fakeResource.ClearResourceCacheReturns(1, nil)
						})

						Context("when no version is passed", func() {
							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("clears the db cache entries successfully", func() {
								Expect(fakeResource.ClearResourceCacheCallCount()).To(Equal(1))
							})

							It("send an empty version", func() {
								version := fakeResource.ClearResourceCacheArgsForCall(0)
								expectedVersion := atc.VersionDeleteBody{}.Version
								Expect(version).To(Equal(expectedVersion))
							})

							It("returns Content-Type 'application/json'", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/json",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("returns the number of rows deleted", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`{"caches_removed": 1}`))
							})
						})

						Context("when a version is passed", func() {
							BeforeEach(func() {
								versionDeleteBody = atc.VersionDeleteBody{Version: atc.Version{"ref": "fake-ref"}}
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("clears the db cache entries successfully", func() {
								Expect(fakeResource.ClearResourceCacheCallCount()).To(Equal(1))
							})

							It("send a non empty version", func() {
								version := fakeResource.ClearResourceCacheArgsForCall(0)
								expectedVersion := atc.Version{"ref": "fake-ref"}
								Expect(version).To(Equal(expectedVersion))
							})

							It("returns Content-Type 'application/json'", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/json",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("returns the number of rows deleted", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`{"caches_removed": 1}`))
							})
						})
					})

					Context("when no rows were deleted", func() {
						BeforeEach(func() {
							fakeResource.ClearResourceCacheReturns(0, nil)
						})

						It("returns that 0 rows were deleted", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`{"caches_removed": 0}`))
						})
					})

					Context("when clear cache fails", func() {
						BeforeEach(func() {
							fakeResource.ClearResourceCacheReturns(0, errors.New("welp"))
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

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/shared", func() {
		var response *http.Response
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/a-team/pipelines/a-pipeline/resources/some-resource/shared")
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

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when user is admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("tries to find the resource", func() {
					Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
					Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
				})

				Context("when the resource exists", func() {
					BeforeEach(func() {
						fakeResource = new(dbfakes.FakeResource)
						fakePipeline.ResourceReturns(fakeResource, true, nil)
					})

					It("tries to find all the shared resources and types", func() {
						Expect(fakeResource.SharedResourcesAndTypesCallCount()).To(Equal(1))
					})

					Context("when finding shared resources and types succeeds", func() {
						BeforeEach(func() {
							fakeResource.SharedResourcesAndTypesReturns(atc.ResourcesAndTypes{
								Resources: atc.ResourceIdentifiers{
									{
										Name:         "some-resource",
										PipelineName: "pipeline-a",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-1",
										PipelineName: "pipeline-a",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-2",
										PipelineName: "pipeline-b",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-3",
										PipelineName: "pipeline-c",
										TeamName:     "team-b",
									},
								},
								ResourceTypes: atc.ResourceIdentifiers{
									{
										Name:         "resource-type-a",
										PipelineName: "pipeline-a",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-type-b",
										PipelineName: "pipeline-b",
										TeamName:     "team-a",
									},
								},
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

						It("returns shared resources", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`{
							"resources": [
								{
									"name": "some-resource",
									"pipeline_name": "pipeline-a",
									"team_name": "team-a"
								},
								{
									"name": "resource-1",
									"pipeline_name": "pipeline-a",
									"team_name": "team-a"
								},
								{
									"name": "resource-2",
									"pipeline_name": "pipeline-b",
									"team_name": "team-a"
								},
								{
									"name": "resource-3",
									"pipeline_name": "pipeline-c",
									"team_name": "team-b"
								}
							],
							"resource_types": [
								{
									"name": "resource-type-a",
									"pipeline_name": "pipeline-a",
									"team_name": "team-a"
								},
								{
									"name": "resource-type-b",
									"pipeline_name": "pipeline-b",
									"team_name": "team-a"
								}
							]
						}`))
						})
					})

					Context("when getting shared resources fails", func() {
						BeforeEach(func() {
							fakeResource.SharedResourcesAndTypesReturns(atc.ResourcesAndTypes{}, errors.New("some-error"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("some-error")))
						})
					})
				})

				Context("when getting the resource fails", func() {
					Context("when the resource are not found", func() {
						BeforeEach(func() {
							fakePipeline.ResourceReturns(nil, false, nil)
						})

						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("with an unknown error", func() {
						BeforeEach(func() {
							fakePipeline.ResourceReturns(nil, false, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types/:resource_type_name/shared", func() {
		var response *http.Response
		var fakeResourceType *dbfakes.FakeResourceType

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/a-team/pipelines/a-pipeline/resource-types/some-resource-type/shared")
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

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when user is admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("tries to find the resource type", func() {
					Expect(fakePipeline.ResourceTypeCallCount()).To(Equal(1))
					Expect(fakePipeline.ResourceTypeArgsForCall(0)).To(Equal("some-resource-type"))
				})

				Context("when the resource type exists", func() {
					BeforeEach(func() {
						fakeResourceType = new(dbfakes.FakeResourceType)
						fakePipeline.ResourceTypeReturns(fakeResourceType, true, nil)
					})

					It("tries to find all the shared resource and resource types", func() {
						Expect(fakeResourceType.SharedResourcesAndTypesCallCount()).To(Equal(1))
					})

					Context("when finding shared resources and resource types succeeds", func() {
						BeforeEach(func() {
							fakeResourceType.SharedResourcesAndTypesReturns(atc.ResourcesAndTypes{
								Resources: atc.ResourceIdentifiers{
									{
										Name:         "some-resource",
										PipelineName: "pipeline-a",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-1",
										PipelineName: "pipeline-b",
										TeamName:     "team-a",
									},
								},
								ResourceTypes: atc.ResourceIdentifiers{
									{
										Name:         "some-resource-type",
										PipelineName: "pipeline-a",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-type-a",
										PipelineName: "pipeline-a",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-type-b",
										PipelineName: "pipeline-b",
										TeamName:     "team-a",
									},
									{
										Name:         "resource-type-c",
										PipelineName: "pipeline-c",
										TeamName:     "team-b",
									},
								},
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

						It("returns shared resources and resource types", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`{
							"resources": [
								{
									"name": "some-resource",
									"pipeline_name": "pipeline-a",
									"team_name": "team-a"
								},
								{
									"name": "resource-1",
									"pipeline_name": "pipeline-b",
									"team_name": "team-a"
								}
							],
							"resource_types": [
								{
									"name": "some-resource-type",
									"pipeline_name": "pipeline-a",
									"team_name": "team-a"
								},
								{
									"name": "resource-type-a",
									"pipeline_name": "pipeline-a",
									"team_name": "team-a"
								},
								{
									"name": "resource-type-b",
									"pipeline_name": "pipeline-b",
									"team_name": "team-a"
								},
								{
									"name": "resource-type-c",
									"pipeline_name": "pipeline-c",
									"team_name": "team-b"
								}
							]
						}`))
						})
					})

					Context("when getting shared resources and types fails", func() {
						BeforeEach(func() {
							fakeResourceType.SharedResourcesAndTypesReturns(atc.ResourcesAndTypes{}, errors.New("some-error"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("some-error")))
						})
					})
				})

				Context("when getting the resource type fails", func() {
					Context("when the resource are not found", func() {
						BeforeEach(func() {
							fakePipeline.ResourceTypeReturns(nil, false, nil)
						})

						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("with an unknown error", func() {
						BeforeEach(func() {
							fakePipeline.ResourceTypeReturns(nil, false, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})
		})
	})
})
