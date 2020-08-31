package api_test

import (
	"bytes"
	"encoding/json"
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

var _ = Describe("Builds API", func() {

	Describe("POST /api/v1/builds", func() {
		var plan atc.Plan
		var response *http.Response

		BeforeEach(func() {
			plan = atc.Plan{
				Task: &atc.TaskPlan{
					Config: &atc.TaskConfig{
						Run: atc.TaskRunConfig{
							Path: "ls",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(plan)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/builds", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not trigger a build", func() {
				Expect(dbTeam.CreateStartedBuildCallCount()).To(BeZero())
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
				})

				Context("when creating a started build fails", func() {
					BeforeEach(func() {
						dbTeam.CreateStartedBuildReturns(nil, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when creating a started build succeeds", func() {
					var fakeBuild *dbfakes.FakeBuild

					BeforeEach(func() {
						fakeBuild = new(dbfakes.FakeBuild)
						fakeBuild.IDReturns(42)
						fakeBuild.NameReturns("1")
						fakeBuild.TeamNameReturns("some-team")
						fakeBuild.StatusReturns("started")
						fakeBuild.StartTimeReturns(time.Unix(1, 0))
						fakeBuild.EndTimeReturns(time.Unix(100, 0))
						fakeBuild.ReapTimeReturns(time.Unix(200, 0))

						dbTeam.CreateStartedBuildReturns(fakeBuild, nil)
					})

					It("returns 201 Created", func() {
						Expect(response.StatusCode).To(Equal(http.StatusCreated))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("creates a started build", func() {
						Expect(dbTeam.CreateStartedBuildCallCount()).To(Equal(1))
						Expect(dbTeam.CreateStartedBuildArgsForCall(0)).To(Equal(plan))
					})

					It("returns the created build", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"id": 42,
							"name": "1",
							"team_name": "some-team",
							"status": "started",
							"api_url": "/api/v1/builds/42",
							"start_time": 1,
							"end_time": 100,
							"reap_time": 200
						}`))
					})

				})
			})
		})
	})

	Describe("GET /api/v1/builds", func() {
		var response *http.Response
		var queryParams string
		var returnedBuilds []db.Build

		BeforeEach(func() {
			queryParams = ""
			build1 := new(dbfakes.FakeBuild)
			build1.IDReturns(4)
			build1.NameReturns("2")
			build1.JobNameReturns("job2")
			build1.PipelineNameReturns("pipeline2")
			build1.TeamNameReturns("some-team")
			build1.StatusReturns(db.BuildStatusStarted)
			build1.StartTimeReturns(time.Unix(1, 0))
			build1.EndTimeReturns(time.Unix(100, 0))
			build1.ReapTimeReturns(time.Unix(300, 0))

			build2 := new(dbfakes.FakeBuild)
			build2.IDReturns(3)
			build2.NameReturns("1")
			build2.JobNameReturns("job1")
			build2.PipelineNameReturns("pipeline1")
			build2.TeamNameReturns("some-team")
			build2.StatusReturns(db.BuildStatusSucceeded)
			build2.StartTimeReturns(time.Unix(101, 0))
			build2.EndTimeReturns(time.Unix(200, 0))
			build2.ReapTimeReturns(time.Unix(400, 0))

			returnedBuilds = []db.Build{build1, build2}
			fakeAccess.TeamNamesReturns([]string{"some-team"})
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			Context("when no params are passed", func() {
				BeforeEach(func() {
					queryParams = ""
				})

				It("does not set defaults for since and until", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					teamName, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						From:  0,
						To:    0,
						Limit: 100,
					}))
					Expect(teamName).To(ConsistOf("some-team"))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?from=2&to=3&limit=8"
				})

				It("passes them through", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						From:  2,
						To:    3,
						Limit: 8,
					}))
				})

				Context("timestamp is provided", func() {
					BeforeEach(func() {
						queryParams = "?timestamps=true"
					})

					It("calls AllBuilds", func() {
						_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
						Expect(page.UseDate).To(Equal(true))
					})
				})
			})

			Context("when getting the builds succeeds", func() {
				BeforeEach(func() {
					dbBuildFactory.VisibleBuildsReturns(returnedBuilds, db.Pagination{}, nil)
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

				It("returns all builds", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
						{
							"id": 4,
							"name": "2",
							"job_name": "job2",
							"pipeline_name": "pipeline2",
							"team_name": "some-team",
							"status": "started",
							"api_url": "/api/v1/builds/4",
							"start_time": 1,
							"end_time": 100,
							"reap_time": 300
						},
						{
							"id": 3,
							"name": "1",
							"job_name": "job1",
							"pipeline_name": "pipeline1",
							"team_name": "some-team",
							"status": "succeeded",
							"api_url": "/api/v1/builds/3",
							"start_time": 101,
							"end_time": 200,
							"reap_time": 400
						}
					]`))
				})
			})

			Context("when next/previous pages are available", func() {
				BeforeEach(func() {
					dbBuildFactory.VisibleBuildsReturns(returnedBuilds, db.Pagination{
						Newer: &db.Page{From: 4, Limit: 2},
						Older: &db.Page{To: 3, Limit: 2},
					}, nil)
				})

				It("returns Link headers per rfc5988", func() {
					Expect(response.Header["Link"]).To(ConsistOf([]string{
						fmt.Sprintf(`<%s/api/v1/builds?from=4&limit=2>; rel="previous"`, externalURL),
						fmt.Sprintf(`<%s/api/v1/builds?to=3&limit=2>; rel="next"`, externalURL),
					}))
				})
			})

			Context("when getting all builds fails", func() {
				BeforeEach(func() {
					dbBuildFactory.VisibleBuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when user has the admin privilege", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("calls AllBuilds", func() {
					Expect(dbBuildFactory.AllBuildsCallCount()).To(Equal(1))
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(0))
				})

			})

			Context("when no params are passed", func() {
				BeforeEach(func() {
					queryParams = ""
				})

				It("does not set defaults for since and until", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						From:  0,
						To:    0,
						Limit: 100,
					}))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?from=2&to=3&limit=8"
				})

				It("passes them through", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						From:  2,
						To:    3,
						Limit: 8,
					}))
				})
			})

			Context("when getting the builds succeeds", func() {
				BeforeEach(func() {
					dbBuildFactory.VisibleBuildsReturns(returnedBuilds, db.Pagination{}, nil)
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

				It("returns all builds", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
						{
							"id": 4,
							"name": "2",
							"job_name": "job2",
							"pipeline_name": "pipeline2",
							"team_name": "some-team",
							"status": "started",
							"api_url": "/api/v1/builds/4",
							"start_time": 1,
							"end_time": 100,
							"reap_time": 300
						},
						{
							"id": 3,
							"name": "1",
							"job_name": "job1",
							"pipeline_name": "pipeline1",
							"team_name": "some-team",
							"status": "succeeded",
							"api_url": "/api/v1/builds/3",
							"start_time": 101,
							"end_time": 200,
							"reap_time": 400
						}
					]`))
				})

				It("returns builds for teams from the token", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))
					teamName, _ := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(teamName).To(ConsistOf("some-team"))
				})
			})

			Context("when next/previous pages are available", func() {
				BeforeEach(func() {
					dbBuildFactory.VisibleBuildsReturns(returnedBuilds, db.Pagination{
						Newer: &db.Page{From: 4, Limit: 2},
						Older: &db.Page{To: 3, Limit: 2},
					}, nil)
				})

				It("returns Link headers per rfc5988", func() {
					Expect(response.Header["Link"]).To(ConsistOf([]string{
						fmt.Sprintf(`<%s/api/v1/builds?from=4&limit=2>; rel="previous"`, externalURL),
						fmt.Sprintf(`<%s/api/v1/builds?to=3&limit=2>; rel="next"`, externalURL),
					}))
				})
			})

			Context("when getting all builds fails", func() {
				BeforeEach(func() {
					dbBuildFactory.VisibleBuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id", func() {
		var response *http.Response

		Context("when parsing the build_id fails", func() {
			BeforeEach(func() {
				var err error

				response, err = client.Get(server.URL + "/api/v1/builds/nope")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns Bad Request", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when parsing the build_id succeeds", func() {
			JustBeforeEach(func() {
				var err error

				response, err = client.Get(server.URL + "/api/v1/builds/1")
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when calling the database fails", func() {
				BeforeEach(func() {
					dbBuildFactory.BuildReturns(nil, false, errors.New("disaster"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the build cannot be found", func() {
				BeforeEach(func() {
					dbBuildFactory.BuildReturns(nil, false, nil)
				})

				It("returns Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the build can be found", func() {
				BeforeEach(func() {
					build.IDReturns(1)
					build.NameReturns("1")
					build.JobNameReturns("job1")
					build.PipelineNameReturns("pipeline1")
					build.TeamNameReturns("some-team")
					build.StatusReturns(db.BuildStatusSucceeded)
					build.StartTimeReturns(time.Unix(1, 0))
					build.EndTimeReturns(time.Unix(100, 0))
					build.ReapTimeReturns(time.Unix(200, 0))
					dbBuildFactory.BuildReturns(build, true, nil)
					build.PipelineReturns(fakePipeline, true, nil)
					fakePipeline.PublicReturns(true)
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
						fakeAccess.IsAuthorizedReturns(false)
					})

					Context("and build is one off", func() {
						BeforeEach(func() {
							build.PipelineReturns(nil, false, nil)
						})

						It("returns 401", func() {
							Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
						})
					})

					Context("and the pipeline is private", func() {
						BeforeEach(func() {
							fakePipeline.PublicReturns(false)
							build.PipelineReturns(fakePipeline, true, nil)
						})

						It("returns 401", func() {
							Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
						})
					})

					Context("and the pipeline is public", func() {
						BeforeEach(func() {
							fakePipeline.PublicReturns(true)
							build.PipelineReturns(fakePipeline, true, nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
					})

					Context("when user is not authorized", func() {
						BeforeEach(func() {
							fakeAccess.IsAuthorizedReturns(false)

						})
						It("returns 200 OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when user is authorized", func() {
						BeforeEach(func() {
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

						It("returns the build with the given build_id", func() {
							Expect(dbBuildFactory.BuildCallCount()).To(Equal(1))
							buildID := dbBuildFactory.BuildArgsForCall(0)
							Expect(buildID).To(Equal(1))

							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`{
						"id": 1,
						"name": "1",
						"status": "succeeded",
						"job_name": "job1",
						"pipeline_name": "pipeline1",
						"team_name": "some-team",
						"api_url": "/api/v1/builds/1",
						"start_time": 1,
						"end_time": 100,
						"reap_time": 200
					}`))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/resources", func() {
		var response *http.Response

		Context("when the build is found", func() {
			BeforeEach(func() {
				build.JobNameReturns("job1")
				build.TeamNameReturns("some-team")
				build.PipelineReturns(fakePipeline, true, nil)
				build.PipelineIDReturns(42)
				dbBuildFactory.BuildReturns(build, true, nil)
			})

			JustBeforeEach(func() {
				var err error

				response, err = client.Get(server.URL + "/api/v1/builds/3/resources")
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
				})

				Context("and build is one off", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						fakePipeline.PublicReturns(false)
						build.PipelineReturns(fakePipeline, true, nil)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						fakePipeline.PublicReturns(true)
						build.PipelineReturns(fakePipeline, true, nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})
			})

			Context("when authenticated, but not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when authenticated and authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the build inputs/outputs are not empty", func() {
					BeforeEach(func() {
						build.ResourcesReturns([]db.BuildInput{
							{
								Name:            "input1",
								Version:         atc.Version{"version": "value1"},
								ResourceID:      1,
								FirstOccurrence: true,
							},
							{
								Name:            "input2",
								Version:         atc.Version{"version": "value2"},
								ResourceID:      2,
								FirstOccurrence: false,
							},
						},
							[]db.BuildOutput{
								{
									Name:    "myresource3",
									Version: atc.Version{"version": "value3"},
								},
								{
									Name:    "myresource4",
									Version: atc.Version{"version": "value4"},
								},
							}, nil)
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns the build with it's input and output versioned resources", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"inputs": [
								{
									"name": "input1",
									"version": {"version": "value1"},
									"pipeline_id": 42,
									"first_occurrence": true
								},
								{
									"name": "input2",
									"version": {"version": "value2"},
									"pipeline_id": 42,
									"first_occurrence": false
								}
							],
							"outputs": [
								{
									"name": "myresource3",
									"version": {"version": "value3"}
								},
								{
									"name": "myresource4",
									"version": {"version": "value4"}
								}
							]
						}`))
					})
				})

				Context("when the build resources error", func() {
					BeforeEach(func() {
						build.ResourcesReturns([]db.BuildInput{}, []db.BuildOutput{}, errors.New("where are my feedback?"))
					})

					It("returns internal server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("with an invalid build", func() {
					Context("when the lookup errors", func() {
						BeforeEach(func() {
							dbBuildFactory.BuildReturns(build, false, errors.New("Freakin' out man, I'm freakin' out!"))
						})

						It("returns internal server error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when the build does not exist", func() {
						BeforeEach(func() {
							dbBuildFactory.BuildReturns(nil, false, nil)
						})

						It("returns internal server error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})
				})
			})
		})

		Context("with an invalid build_id", func() {
			JustBeforeEach(func() {
				var err error

				response, err = client.Get(server.URL + "/api/v1/builds/nope/resources")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns internal server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/events", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("GET", server.URL+"/api/v1/builds/128/events", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the build can be found", func() {
			BeforeEach(func() {
				build.JobNameReturns("some-job")
				build.TeamNameReturns("some-team")
				build.PipelineReturns(fakePipeline, true, nil)
				dbBuildFactory.BuildReturns(build, true, nil)
			})

			Context("when authenticated, but not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(200))
				})

				It("serves the request via the event handler", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(body)).To(Equal("fake event handler factory was here"))

					Expect(constructedEventHandler.build).To(Equal(build))
					Expect(dbBuildFactory.BuildCallCount()).To(Equal(1))
					buildID := dbBuildFactory.BuildArgsForCall(0)
					Expect(buildID).To(Equal(128))
				})
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						build.PipelineReturns(fakePipeline, true, nil)
						fakePipeline.PublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						build.PipelineReturns(fakePipeline, true, nil)
						fakePipeline.PublicReturns(true)
					})

					Context("when the job is found", func() {
						var fakeJob *dbfakes.FakeJob

						BeforeEach(func() {
							fakeJob = new(dbfakes.FakeJob)
							fakePipeline.JobReturns(fakeJob, true, nil)
						})

						Context("and the job is private", func() {
							BeforeEach(func() {
								fakeJob.PublicReturns(false)
							})

							It("returns 401", func() {
								Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
							})
						})

						Context("and the job is public", func() {
							BeforeEach(func() {
								fakeJob.PublicReturns(true)
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(200))
							})

							It("serves the request via the event handler", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(string(body)).To(Equal("fake event handler factory was here"))

								Expect(constructedEventHandler.build).To(Equal(build))
								Expect(dbBuildFactory.BuildCallCount()).To(Equal(1))
								buildID := dbBuildFactory.BuildArgsForCall(0)
								Expect(buildID).To(Equal(128))
							})
						})
					})

					Context("when finding the job fails", func() {
						BeforeEach(func() {
							fakePipeline.JobReturns(nil, false, errors.New("nope"))
						})

						It("returns Internal Server Error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when the job cannot be found", func() {
						BeforeEach(func() {
							fakePipeline.JobReturns(nil, false, nil)
						})

						It("returns Not Found", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})
				})

				Context("when the build can not be found", func() {
					BeforeEach(func() {
						dbBuildFactory.BuildReturns(nil, false, nil)
					})

					It("returns Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when calling the database fails", func() {
					BeforeEach(func() {
						dbBuildFactory.BuildReturns(nil, false, errors.New("nope"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when calling the database fails", func() {
			BeforeEach(func() {
				dbBuildFactory.BuildReturns(nil, false, errors.New("nope"))
			})

			It("returns Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("PUT /api/v1/builds/:build_id/abort", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("PUT", server.URL+"/api/v1/builds/128/abort", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when looking up the build fails", func() {
				BeforeEach(func() {
					dbBuildFactory.BuildReturns(nil, false, errors.New("nope"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the build can not be found", func() {
				BeforeEach(func() {
					dbBuildFactory.BuildReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the build is found", func() {
				BeforeEach(func() {
					build.TeamNameReturns("some-team")
					dbBuildFactory.BuildReturns(build, true, nil)
				})

				Context("when not authorized", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthorizedReturns(false)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("when authorized", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthorizedReturns(true)
					})

					Context("when aborting the build fails", func() {
						BeforeEach(func() {
							build.MarkAsAbortedReturns(errors.New("nope"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when aborting succeeds", func() {
						BeforeEach(func() {
							build.MarkAsAbortedReturns(nil)
						})

						It("returns 204", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNoContent))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/preparation", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error
			response, err = http.Get(server.URL + "/api/v1/builds/42/preparation")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the build is found", func() {
			var buildPrep db.BuildPreparation

			BeforeEach(func() {
				buildPrep = db.BuildPreparation{
					BuildID:          42,
					PausedPipeline:   db.BuildPreparationStatusNotBlocking,
					PausedJob:        db.BuildPreparationStatusNotBlocking,
					MaxRunningBuilds: db.BuildPreparationStatusBlocking,
					Inputs: map[string]db.BuildPreparationStatus{
						"foo": db.BuildPreparationStatusNotBlocking,
						"bar": db.BuildPreparationStatusBlocking,
					},
					InputsSatisfied:     db.BuildPreparationStatusBlocking,
					MissingInputReasons: db.MissingInputReasons{"some-input": "some-reason"},
				}
				dbBuildFactory.BuildReturns(build, true, nil)
				build.JobNameReturns("job1")
				build.TeamNameReturns("some-team")
				build.PreparationReturns(buildPrep, true, nil)
			})

			Context("when authenticated, but not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(false)
					build.PipelineReturns(fakePipeline, true, nil)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
				})

				Context("and build is one off", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						build.PipelineReturns(fakePipeline, true, nil)
						fakePipeline.PublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						build.PipelineReturns(fakePipeline, true, nil)
						fakePipeline.PublicReturns(true)
					})

					Context("when the job is found", func() {
						var fakeJob *dbfakes.FakeJob
						BeforeEach(func() {
							fakeJob = new(dbfakes.FakeJob)
							fakePipeline.JobReturns(fakeJob, true, nil)
						})

						Context("when job is private", func() {
							BeforeEach(func() {
								fakeJob.PublicReturns(false)
							})

							It("returns 401", func() {
								Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
							})
						})

						Context("when job is public", func() {
							BeforeEach(func() {
								fakeJob.PublicReturns(true)
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})
						})
					})

					Context("when finding the job fails", func() {
						BeforeEach(func() {
							fakePipeline.JobReturns(nil, false, errors.New("nope"))
						})

						It("returns Internal Server Error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when the job cannot be found", func() {
						BeforeEach(func() {
							fakePipeline.JobReturns(nil, false, nil)
						})

						It("returns Not Found", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})
				})
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(true)
				})

				It("fetches data from the db", func() {
					Expect(build.PreparationCallCount()).To(Equal(1))
				})

				It("returns OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				It("returns the build preparation", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{
					"build_id": 42,
					"paused_pipeline": "not_blocking",
					"paused_job": "not_blocking",
					"max_running_builds": "blocking",
					"inputs": {
						"foo": "not_blocking",
						"bar": "blocking"
					},
					"inputs_satisfied": "blocking",
					"missing_input_reasons": {
						"some-input": "some-reason"
					}
				}`))
				})

				Context("when the build preparation is not found", func() {
					BeforeEach(func() {
						dbBuildFactory.BuildReturns(build, true, nil)
						build.PreparationReturns(db.BuildPreparation{}, false, nil)
					})

					It("returns Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when looking up the build preparation fails", func() {
					BeforeEach(func() {
						dbBuildFactory.BuildReturns(build, true, nil)
						build.PreparationReturns(db.BuildPreparation{}, false, errors.New("ho ho ho merry festivus"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when looking up the build fails", func() {
			BeforeEach(func() {
				dbBuildFactory.BuildReturns(nil, false, errors.New("ho ho ho merry festivus"))
			})

			It("returns 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when build is not found", func() {
			BeforeEach(func() {
				dbBuildFactory.BuildReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/plan", func() {
		var plan *json.RawMessage

		var response *http.Response

		BeforeEach(func() {
			data := []byte(`{"some":"plan"}`)
			plan = (*json.RawMessage)(&data)
		})

		JustBeforeEach(func() {
			var err error
			response, err = http.Get(server.URL + "/api/v1/builds/42/plan")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the build is found", func() {
			BeforeEach(func() {
				build.JobNameReturns("job1")
				build.TeamNameReturns("some-team")
				dbBuildFactory.BuildReturns(build, true, nil)
			})

			Context("when authenticated, but not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(false)

					build.PipelineReturns(fakePipeline, true, nil)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
				})

				Context("and build is one off", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						build.PipelineReturns(fakePipeline, true, nil)
						fakePipeline.PublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						build.PipelineReturns(fakePipeline, true, nil)
						fakePipeline.PublicReturns(true)
					})

					Context("when finding the job fails", func() {
						BeforeEach(func() {
							fakePipeline.JobReturns(nil, false, errors.New("nope"))
						})
						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when the job does not exist", func() {
						BeforeEach(func() {
							fakePipeline.JobReturns(nil, false, nil)
						})
						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when the job exists", func() {
						var fakeJob *dbfakes.FakeJob

						BeforeEach(func() {
							fakeJob = new(dbfakes.FakeJob)
							fakePipeline.JobReturns(fakeJob, true, nil)
						})

						Context("and the job is public", func() {
							BeforeEach(func() {
								fakeJob.PublicReturns(true)
							})
							Context("and the build has a plan", func() {
								BeforeEach(func() {
									build.HasPlanReturns(true)
								})
								It("returns 200", func() {
									Expect(response.StatusCode).To(Equal(http.StatusOK))
								})
							})
							Context("and the build has no plan", func() {
								BeforeEach(func() {
									build.HasPlanReturns(false)
								})
								It("returns 404", func() {
									Expect(response.StatusCode).To(Equal(http.StatusNotFound))
								})
							})
						})

						Context("and the job is private", func() {
							BeforeEach(func() {
								fakeJob.PublicReturns(false)
							})
							It("returns 401", func() {
								Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
							})
						})
					})
				})
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
					fakeAccess.IsAuthorizedReturns(true)
				})

				Context("when the build returns a plan", func() {
					BeforeEach(func() {
						build.HasPlanReturns(true)
						build.PublicPlanReturns(plan)
						build.SchemaReturns("some-schema")
					})

					It("returns OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns the plan", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
						"schema": "some-schema",
						"plan": {"some":"plan"}
					}`))
					})
				})

				Context("when the build has no plan", func() {
					BeforeEach(func() {
						build.HasPlanReturns(false)
					})

					It("returns no Content-Type header", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "",
						}
						Expect(response).ShouldNot(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns not found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})
		})

		Context("when the build is not found", func() {
			BeforeEach(func() {
				dbBuildFactory.BuildReturns(nil, false, nil)
			})

			It("returns Not Found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when looking up the build fails", func() {
			BeforeEach(func() {
				dbBuildFactory.BuildReturns(nil, false, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
