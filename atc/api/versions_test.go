package api_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Versions API", func() {
	var fakePipeline *dbfakes.FakePipeline

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
		dbTeam.PipelineReturns(fakePipeline, true, nil)
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions", func() {
		var response *http.Response
		var queryParams string
		var fakeResource *dbfakes.FakeResource

		BeforeEach(func() {
			queryParams = ""
			fakeResource = new(dbfakes.FakeResource)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/some-resource/versions"+queryParams, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				Context("user is not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("user is authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
					fakePipeline.ResourceReturns(fakeResource, true, nil)

					returnedVersions := []atc.ResourceVersion{
						{
							ID:      4,
							Enabled: true,
							Version: atc.Version{
								"some": "version",
							},
							Metadata: []atc.MetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							},
						},
						{
							ID:      2,
							Enabled: false,
							Version: atc.Version{
								"some": "version",
							},
							Metadata: []atc.MetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							},
						},
					}

					fakeResource.VersionsReturns(returnedVersions, db.Pagination{}, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns content type application/json", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				Context("when resource is public", func() {
					BeforeEach(func() {
						fakeResource.PublicReturns(true)
					})

					It("returns the json", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
					{
						"id": 4,
						"enabled": true,
						"version": {"some":"version"},
						"metadata": [
							{
								"name":"some",
								"value":"metadata"
							}
						]
					},
					{
						"id":2,
						"enabled": false,
						"version": {"some":"version"},
						"metadata": [
							{
								"name":"some",
								"value":"metadata"
							}
						]
					}
				]`))
					})
				})

				Context("when resource is not public", func() {
					Context("when the user is not authenticated", func() {
						It("returns the json without version metadata", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`[
								{
									"id": 4,
									"enabled": true,
									"version": {"some":"version"}
								},
								{
									"id":2,
									"enabled": false,
									"version": {"some":"version"}
								}
							]`))
						})
					})

					Context("when the user is authenticated", func() {
						BeforeEach(func() {
							fakeAccess.IsAuthenticatedReturns(true)
						})

						It("returns the json without version metadata", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`[
								{
									"id": 4,
									"enabled": true,
									"version": {"some":"version"}
								},
								{
									"id":2,
									"enabled": false,
									"version": {"some":"version"}
								}
							]`))
						})
					})
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			It("finds the resource", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
			})

			Context("when finding the resource succeeds", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				Context("when no params are passed", func() {
					It("does not set defaults for since and until", func() {
						Expect(fakeResource.VersionsCallCount()).To(Equal(1))

						page, versionFilter := fakeResource.VersionsArgsForCall(0)
						Expect(page).To(Equal(db.Page{
							Limit: 100,
						}))
						Expect(versionFilter).To(Equal(atc.Version{}))
					})
				})

				Context("when all the params are passed", func() {
					BeforeEach(func() {
						queryParams = "?from=5&to=7&limit=8&filter=ref:foo&filter=some-ref:blah"
					})

					It("passes them through", func() {
						Expect(fakeResource.VersionsCallCount()).To(Equal(1))

						page, versionFilter := fakeResource.VersionsArgsForCall(0)
						Expect(page).To(Equal(db.Page{
							From:  db.NewIntPtr(5),
							To:    db.NewIntPtr(7),
							Limit: 8,
						}))
						Expect(versionFilter).To(Equal(atc.Version{
							"ref":      "foo",
							"some-ref": "blah",
						}))
					})
				})

				Context("when params includes version filter has special char", func() {
					Context("space char", func() {
						BeforeEach(func() {
							queryParams = "?filter=some%20ref:some%20value"
						})

						It("passes them through", func() {
							Expect(fakeResource.VersionsCallCount()).To(Equal(1))

							_, versionFilter := fakeResource.VersionsArgsForCall(0)
							Expect(versionFilter).To(Equal(atc.Version{
								"some ref": "some value",
							}))
						})
					})

					Context("% char", func() {
						BeforeEach(func() {
							queryParams = "?filter=ref:some%25value"
						})

						It("passes them through", func() {
							Expect(fakeResource.VersionsCallCount()).To(Equal(1))

							_, versionFilter := fakeResource.VersionsArgsForCall(0)
							Expect(versionFilter).To(Equal(atc.Version{
								"ref": "some%value",
							}))
						})
					})

					Context(": char", func() {
						BeforeEach(func() {
							queryParams = "?filter=key%3Awith%3Acolon:abcdef"
						})

						It("passes them through by splitting on first colon", func() {
							Expect(fakeResource.VersionsCallCount()).To(Equal(1))

							_, versionFilter := fakeResource.VersionsArgsForCall(0)
							Expect(versionFilter).To(Equal(atc.Version{
								"key": "with:colon:abcdef",
							}))
						})
					})

					Context("if there is no : ", func() {
						BeforeEach(func() {
							queryParams = "?filter=abcdef"
						})

						It("set no filter when fetching versions", func() {
							Expect(fakeResource.VersionsCallCount()).To(Equal(1))

							_, versionFilter := fakeResource.VersionsArgsForCall(0)
							Expect(versionFilter).To(BeEmpty())
						})
					})
				})

				Context("when getting the versions succeeds", func() {
					var returnedVersions []atc.ResourceVersion

					BeforeEach(func() {
						queryParams = "?since=5&limit=2"
						returnedVersions = []atc.ResourceVersion{
							{
								ID:      4,
								Enabled: true,
								Version: atc.Version{
									"some": "version",
									"ref":  "foo",
								},
								Metadata: []atc.MetadataField{
									{
										Name:  "some",
										Value: "metadata",
									},
								},
							},
							{
								ID:      2,
								Enabled: false,
								Version: atc.Version{
									"some": "version",
									"ref":  "blah",
								},
								Metadata: []atc.MetadataField{
									{
										Name:  "some",
										Value: "metadata",
									},
								},
							},
						}

						fakeResource.VersionsReturns(returnedVersions, db.Pagination{}, true, nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns content type application/json", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns the json", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
					{
						"id": 4,
						"enabled": true,
						"version": {"some":"version", "ref":"foo"},
						"metadata": [
							{
								"name":"some",
								"value":"metadata"
							}
						]
					},
					{
						"id":2,
						"enabled": false,
						"version": {"some":"version", "ref":"blah"},
						"metadata": [
							{
								"name":"some",
								"value":"metadata"
							}
						]
					}
				]`))
					})

					Context("when next/previous pages are available", func() {
						BeforeEach(func() {
							fakePipeline.IDReturns(123)
							fakeResource.VersionsReturns(returnedVersions, db.Pagination{
								Newer: &db.Page{From: db.NewIntPtr(4), Limit: 2},
								Older: &db.Page{To: db.NewIntPtr(2), Limit: 2},
							}, true, nil)
						})

						It("returns Link headers per rfc5988", func() {
							Expect(response.Header["Link"]).To(ConsistOf([]string{
								fmt.Sprintf(`<%s/api/v1/pipelines/123/resources/some-resource/versions?from=4&limit=2>; rel="previous"`, externalURL),
								fmt.Sprintf(`<%s/api/v1/pipelines/123/resources/some-resource/versions?to=2&limit=2>; rel="next"`, externalURL),
							}))
						})
					})
				})

				Context("when the versions can't be found", func() {
					BeforeEach(func() {
						fakeResource.VersionsReturns(nil, db.Pagination{}, false, nil)
					})

					It("returns 404 not found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when getting the versions fails", func() {
					BeforeEach(func() {
						fakeResource.VersionsReturns(nil, db.Pagination{}, false, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when finding the resource fails", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the resource is not found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns 404 not found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/enable", func() {
		var response *http.Response
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/versions/42/enable", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
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

					It("tries to enable the right resource config version", func() {
						resourceConfigVersionID := fakeResource.EnableVersionArgsForCall(0)
						Expect(resourceConfigVersionID).To(Equal(42))
					})

					Context("when enabling the resource succeeds", func() {
						BeforeEach(func() {
							fakeResource.EnableVersionReturns(nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when enabling the resource fails", func() {
						BeforeEach(func() {
							fakeResource.EnableVersionReturns(errors.New("welp"))
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

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/disable", func() {
		var response *http.Response
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/versions/42/disable", nil)
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

					It("tries to disable the right resource config version", func() {
						resourceConfigVersionID := fakeResource.DisableVersionArgsForCall(0)
						Expect(resourceConfigVersionID).To(Equal(42))
					})

					Context("when disabling the resource version succeeds", func() {
						BeforeEach(func() {
							fakeResource.DisableVersionReturns(nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when disabling the resource fails", func() {
						BeforeEach(func() {
							fakeResource.DisableVersionReturns(errors.New("welp"))
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

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/pin", func() {
		var response *http.Response
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/resource-name/versions/42/pin", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
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

					It("tries to pin the right resource config version", func() {
						resourceConfigVersionID := fakeResource.PinVersionArgsForCall(0)
						Expect(resourceConfigVersionID).To(Equal(42))
					})

					Context("when pinning the resource succeeds", func() {
						BeforeEach(func() {
							fakeResource.PinVersionReturns(true, nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when pinning the resource fails by resource not exist", func() {
						BeforeEach(func() {
							fakeResource.PinVersionReturns(false, nil)
						})

						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when pinning the resource fails by error", func() {
						BeforeEach(func() {
							fakeResource.PinVersionReturns(false, errors.New("welp"))
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

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/input_to", func() {
		var response *http.Response
		var stringVersionID string
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/some-resource/versions/"+stringVersionID+"/input_to", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeResource = new(dbfakes.FakeResource)
			fakeResource.IDReturns(1)
			stringVersionID = "123"
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when not finding the resource", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when failing to retrieve the resource", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, errors.New("banana"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			It("looks for the resource", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
			})

			Context("when resource retrieval succeeds", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				It("looks up the given version ID", func() {
					Expect(fakePipeline.GetBuildsWithVersionAsInputCallCount()).To(Equal(1))
					resourceID, versionID := fakePipeline.GetBuildsWithVersionAsInputArgsForCall(0)
					Expect(resourceID).To(Equal(1))
					Expect(versionID).To(Equal(123))
				})

				Context("when getting the builds succeeds", func() {
					BeforeEach(func() {
						build1 := new(dbfakes.FakeBuild)
						build1.IDReturns(1024)
						build1.NameReturns("5")
						build1.JobNameReturns("some-job")
						build1.PipelineNameReturns("a-pipeline")
						build1.TeamNameReturns("a-team")
						build1.StatusReturns(db.BuildStatusSucceeded)
						build1.StartTimeReturns(time.Unix(1, 0))
						build1.EndTimeReturns(time.Unix(100, 0))

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(1025)
						build2.NameReturns("6")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("a-pipeline")
						build2.TeamNameReturns("a-team")
						build2.StatusReturns(db.BuildStatusSucceeded)
						build2.StartTimeReturns(time.Unix(200, 0))
						build2.EndTimeReturns(time.Unix(300, 0))

						fakePipeline.GetBuildsWithVersionAsInputReturns([]db.Build{build1, build2}, nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns content type application/json", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns the json", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
					{
						"id": 1024,
						"team_name": "a-team",
						"name": "5",
						"status": "succeeded",
						"job_name": "some-job",
						"api_url": "/api/v1/builds/1024",
						"pipeline_name": "a-pipeline",
						"start_time": 1,
						"end_time": 100
					},
					{
						"id": 1025,
						"name": "6",
						"team_name": "a-team",
						"status": "succeeded",
						"job_name": "some-job",
						"api_url": "/api/v1/builds/1025",
						"pipeline_name": "a-pipeline",
						"start_time": 200,
						"end_time": 300
					}
				]`))
					})
				})

				Context("when the version ID is invalid", func() {
					BeforeEach(func() {
						stringVersionID = "hello"
					})

					It("returns an empty list", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[]`))
					})
				})

				Context("when the call to get builds returns an error", func() {
					BeforeEach(func() {
						fakePipeline.GetBuildsWithVersionAsInputReturns(nil, errors.New("NOPE"))
					})

					It("returns a 500 internal server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/output_of", func() {
		var response *http.Response
		var stringVersionID string
		var fakeResource *dbfakes.FakeResource

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/resources/some-resource/versions/"+stringVersionID+"/output_of", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			stringVersionID = "123"
			fakeResource = new(dbfakes.FakeResource)
			fakeResource.IDReturns(1)
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when not finding the resource", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when failing to retrieve the resource", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, errors.New("banana"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			It("looks for the resource", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
			})

			Context("when resource retrieval succeeds", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(fakeResource, true, nil)
				})

				It("looks up the given version ID", func() {
					Expect(fakePipeline.GetBuildsWithVersionAsOutputCallCount()).To(Equal(1))
					resourceID, versionID := fakePipeline.GetBuildsWithVersionAsOutputArgsForCall(0)
					Expect(resourceID).To(Equal(1))
					Expect(versionID).To(Equal(123))
				})

				Context("when getting the builds succeeds", func() {
					BeforeEach(func() {
						build1 := new(dbfakes.FakeBuild)
						build1.IDReturns(1024)
						build1.NameReturns("5")
						build1.JobNameReturns("some-job")
						build1.PipelineNameReturns("a-pipeline")
						build1.TeamNameReturns("a-team")
						build1.StatusReturns(db.BuildStatusSucceeded)
						build1.StartTimeReturns(time.Unix(1, 0))
						build1.EndTimeReturns(time.Unix(100, 0))

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(1025)
						build2.NameReturns("6")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("a-pipeline")
						build2.TeamNameReturns("a-team")
						build2.StatusReturns(db.BuildStatusSucceeded)
						build2.StartTimeReturns(time.Unix(200, 0))
						build2.EndTimeReturns(time.Unix(300, 0))

						fakePipeline.GetBuildsWithVersionAsOutputReturns([]db.Build{build1, build2}, nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns content type application/json", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns the json", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
					{
						"id": 1024,
						"name": "5",
						"status": "succeeded",
						"job_name": "some-job",
						"api_url": "/api/v1/builds/1024",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"start_time": 1,
						"end_time": 100
					},
					{
						"id": 1025,
						"name": "6",
						"status": "succeeded",
						"job_name": "some-job",
						"api_url": "/api/v1/builds/1025",
						"pipeline_name": "a-pipeline",
						"team_name": "a-team",
						"start_time": 200,
						"end_time": 300
					}
				]`))
					})
				})

				Context("when the version ID is invalid", func() {
					BeforeEach(func() {
						stringVersionID = "hello"
					})

					It("returns an empty list", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[]`))
					})
				})

				Context("when the call to get builds returns an error", func() {
					BeforeEach(func() {
						fakePipeline.GetBuildsWithVersionAsOutputReturns(nil, errors.New("NOPE"))
					})

					It("returns a 500 internal server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})
})
