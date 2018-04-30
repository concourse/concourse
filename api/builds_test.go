package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine/enginefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Builds API", func() {
	var fakeaccess *accessorfakes.FakeAccess

	BeforeEach(func() {
		fakeaccess = new(accessorfakes.FakeAccess)
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
	})

	Describe("POST /api/v1/builds", func() {
		var plan atc.Plan
		var response *http.Response

		BeforeEach(func() {
			fakeaccess = new(accessorfakes.FakeAccess)
			plan = atc.Plan{
				Task: &atc.TaskPlan{
					Config: &atc.TaskConfig{
						Run: atc.TaskRunConfig{
							Path: "ls",
						},
					},
				},
			}
			fakeaccess.IsAuthenticatedReturns(true)
		})

		JustBeforeEach(func() {
			fakeAccessor.CreateReturns(fakeaccess)

			reqPayload, err := json.Marshal(plan)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/builds", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(true)
			})

			Context("when creating a one-off build succeeds", func() {
				BeforeEach(func() {
					dbTeam.CreateOneOffBuildStub = func() (db.Build, error) {
						Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
						teamName := dbTeamFactory.FindTeamArgsForCall(0)
						build.IDReturns(42)
						build.NameReturns("1")
						build.TeamNameReturns(teamName)
						build.StatusReturns(db.BuildStatusStarted)
						build.StartTimeReturns(time.Unix(1, 0))
						build.EndTimeReturns(time.Unix(100, 0))
						build.ReapTimeReturns(time.Unix(200, 0))
						return build, nil
					}
				})

				Context("and building succeeds", func() {
					var fakeEngineBuild *enginefakes.FakeBuild
					var resumed <-chan struct{}
					var blockForever *sync.WaitGroup

					BeforeEach(func() {
						fakeEngineBuild = new(enginefakes.FakeBuild)

						blockForever = new(sync.WaitGroup)

						forever := blockForever
						forever.Add(1)

						r := make(chan struct{})
						resumed = r
						fakeEngineBuild.ResumeStub = func(lager.Logger) {
							close(r)
							forever.Wait()
						}

						fakeEngine.CreateBuildReturns(fakeEngineBuild, nil)
					})

					AfterEach(func() {
						blockForever.Done()
					})

					It("returns 201 Created", func() {
						Expect(response.StatusCode).To(Equal(http.StatusCreated))
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("creates build for specified team", func() {
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

					It("creates a one-off build and runs it asynchronously", func() {
						Expect(dbTeam.CreateOneOffBuildCallCount()).To(Equal(1))

						Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))
						_, oneOffBuild, builtPlan := fakeEngine.CreateBuildArgsForCall(0)
						Expect(oneOffBuild).To(Equal(build))

						Expect(builtPlan).To(Equal(plan))

						<-resumed
					})
				})

				Context("and building fails", func() {
					BeforeEach(func() {
						fakeEngine.CreateBuildReturns(nil, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when creating a one-off build fails", func() {
				BeforeEach(func() {
					dbTeam.CreateOneOffBuildReturns(nil, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
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

			It("does not trigger a build", func() {
				Expect(dbTeam.CreateOneOffBuildCallCount()).To(BeZero())
				Expect(fakeEngine.CreateBuildCallCount()).To(BeZero())
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
			fakeaccess.TeamNamesReturns([]string{"some-team"})
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			Context("when no params are passed", func() {
				BeforeEach(func() {
					queryParams = ""
				})

				It("does not set defaults for since and until", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					teamName, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						Since: 0,
						Until: 0,
						Limit: 100,
					}))
					Expect(teamName).To(ConsistOf("some-team"))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?since=2&until=3&limit=8"
				})

				It("passes them through", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						Since: 2,
						Until: 3,
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
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
						Previous: &db.Page{Until: 4, Limit: 2},
						Next:     &db.Page{Since: 3, Limit: 2},
					}, nil)
				})

				It("returns Link headers per rfc5988", func() {
					Expect(response.Header["Link"]).To(ConsistOf([]string{
						fmt.Sprintf(`<%s/api/v1/builds?until=4&limit=2>; rel="previous"`, externalURL),
						fmt.Sprintf(`<%s/api/v1/builds?since=3&limit=2>; rel="next"`, externalURL),
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
				fakeaccess.IsAuthenticatedReturns(true)
			})

			Context("when no params are passed", func() {
				BeforeEach(func() {
					queryParams = ""
				})

				It("does not set defaults for since and until", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						Since: 0,
						Until: 0,
						Limit: 100,
					}))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?since=2&until=3&limit=8"
				})

				It("passes them through", func() {
					Expect(dbBuildFactory.VisibleBuildsCallCount()).To(Equal(1))

					_, page := dbBuildFactory.VisibleBuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						Since: 2,
						Until: 3,
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
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
						Previous: &db.Page{Until: 4, Limit: 2},
						Next:     &db.Page{Since: 3, Limit: 2},
					}, nil)
				})

				It("returns Link headers per rfc5988", func() {
					Expect(response.Header["Link"]).To(ConsistOf([]string{
						fmt.Sprintf(`<%s/api/v1/builds?until=4&limit=2>; rel="previous"`, externalURL),
						fmt.Sprintf(`<%s/api/v1/builds?since=3&limit=2>; rel="next"`, externalURL),
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
						fakeaccess.IsAuthenticatedReturns(false)
						fakeaccess.IsAuthorizedReturns(false)
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
						fakeaccess.IsAuthenticatedReturns(true)
					})

					Context("when user is not authorized", func() {
						BeforeEach(func() {
							fakeaccess.IsAuthorizedReturns(false)

						})
						It("returns 200 OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when user is authorized", func() {
						BeforeEach(func() {
							fakeaccess.IsAuthorizedReturns(true)
						})

						It("returns 200 OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns Content-Type 'application/json'", func() {
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
					fakeaccess.IsAuthenticatedReturns(false)
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
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when authenticated and authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the build inputs/outputs are not empty", func() {
					BeforeEach(func() {
						build.ResourcesReturns([]db.BuildInput{
							{
								Name: "input1",
								VersionedResource: db.VersionedResource{
									Resource: "myresource1",
									Type:     "git",
									Version:  db.ResourceVersion{"version": "value1"},
									Metadata: []db.ResourceMetadataField{
										{
											Name:  "meta1",
											Value: "value1",
										},
										{
											Name:  "meta2",
											Value: "value2",
										},
									},
								},
								FirstOccurrence: true,
							},
							{
								Name: "input2",
								VersionedResource: db.VersionedResource{
									Resource: "myresource2",
									Type:     "git",
									Version:  db.ResourceVersion{"version": "value2"},
									Metadata: []db.ResourceMetadataField{},
								},
								FirstOccurrence: false,
							},
						},
							[]db.BuildOutput{
								{
									VersionedResource: db.VersionedResource{
										Resource: "myresource3",
										Version:  db.ResourceVersion{"version": "value3"},
									},
								},
								{
									VersionedResource: db.VersionedResource{
										Resource: "myresource4",
										Version:  db.ResourceVersion{"version": "value4"},
									},
								},
							}, nil)
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns the build with it's input and output versioned resources", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"inputs": [
								{
									"name": "input1",
									"resource": "myresource1",
									"type": "git",
									"version": {"version": "value1"},
									"metadata":[
										{
											"name": "meta1",
											"value": "value1"
										},
										{
											"name": "meta2",
											"value": "value2"
										}
									],
									"pipeline_id": 42,
									"first_occurrence": true
								},
								{
									"name": "input2",
									"resource": "myresource2",
									"type": "git",
									"version": {"version": "value2"},
									"metadata": [],
									"pipeline_id": 42,
									"first_occurrence": false
								}
							],
							"outputs": [
								{
									"id": 0,
									"type":"",
									"metadata":null,
									"resource": "myresource3",
									"version": {"version": "value3"},
									"enabled": false
								},
								{
									"id": 0,
									"type":"",
									"metadata":null,
									"resource": "myresource4",
									"version": {"version": "value4"},
									"enabled": false
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
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(true)
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
					fakeaccess.IsAuthenticatedReturns(false)
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
								fakeJob.ConfigReturns(atc.JobConfig{
									Name:   "some-job",
									Public: false,
								})
							})

							It("returns 401", func() {
								Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
							})
						})

						Context("and the job is public", func() {
							BeforeEach(func() {
								fakeJob.ConfigReturns(atc.JobConfig{
									Name:   "some-job",
									Public: true,
								})
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

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
			})

			Context("when the build can be found", func() {
				BeforeEach(func() {
					build.TeamNameReturns("some-team")
					dbBuildFactory.BuildReturns(build, true, nil)
				})

				Context("when accessing same team's build", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthorizedReturns(true)
					})

					Context("when the engine returns a build", func() {
						var engineBuild *enginefakes.FakeBuild

						BeforeEach(func() {
							engineBuild = new(enginefakes.FakeBuild)
							fakeEngine.LookupBuildReturns(engineBuild, nil)
						})

						It("aborts the build", func() {
							Expect(engineBuild.AbortCallCount()).To(Equal(1))
						})

						Context("when aborting succeeds", func() {
							BeforeEach(func() {
								engineBuild.AbortReturns(nil)
							})

							It("returns 204", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNoContent))
							})
						})

						Context("when aborting fails", func() {
							BeforeEach(func() {
								engineBuild.AbortReturns(errors.New("oh no!"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the engine returns no build", func() {
						BeforeEach(func() {
							fakeEngine.LookupBuildReturns(nil, errors.New("oh no!"))
						})

						It("returns Internal Server Error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when accessing other team's build", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthorizedReturns(false)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
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

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
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
						"foo": db.BuildPreparationStatusUnknown,
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
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(false)
					build.PipelineReturns(fakePipeline, true, nil)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(false)
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
								fakeJob.ConfigReturns(atc.JobConfig{
									Name:   "job1",
									Public: false,
								})
							})

							It("returns 401", func() {
								Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
							})
						})

						Context("when job is public", func() {
							BeforeEach(func() {
								fakeJob.ConfigReturns(atc.JobConfig{
									Name:   "job1",
									Public: true,
								})
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
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(true)
				})

				It("fetches data from the db", func() {
					Expect(build.PreparationCallCount()).To(Equal(1))
				})

				It("returns OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
						"foo": "unknown",
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
			var engineBuild *enginefakes.FakeBuild

			BeforeEach(func() {
				build.JobNameReturns("job1")
				build.TeamNameReturns("some-team")
				dbBuildFactory.BuildReturns(build, true, nil)

				engineBuild = new(enginefakes.FakeBuild)
				fakeEngine.LookupBuildReturns(engineBuild, nil)
			})

			Context("when authenticated, but not authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(false)

					build.PipelineReturns(fakePipeline, true, nil)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(false)
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

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)
					fakeaccess.IsAuthorizedReturns(true)
				})

				Context("when the build returns a plan", func() {
					BeforeEach(func() {
						build.PublicPlanReturns(plan)
						build.EngineReturns("some-schema")
					})

					It("returns OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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

	Describe("PUT /api/v1/builds/:build_id/plan/:plan_id/input", func() {
		var (
			otherTracker *ghttp.Server

			response *http.Response
		)

		BeforeEach(func() {
			otherTracker = ghttp.NewServer()

			otherTracker.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/builds/128/plan/some-plan/input"),
					ghttp.VerifyBody([]byte("some-payload")),
					ghttp.RespondWith(http.StatusTeapot, "im a teapot"),
				),
			)
		})

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("PUT", server.URL+"/api/v1/builds/128/plan/some-plan/input", bytes.NewBufferString("some-payload"))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			otherTracker.Close()
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
			})

			Context("when the build can be found", func() {
				BeforeEach(func() {
					build.TeamNameReturns("some-team")
					dbBuildFactory.BuildReturns(build, true, nil)
				})

				Context("when accessing same teams build", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthorizedReturns(true)
					})

					Context("when the build is tracked by the current ATC", func() {
						BeforeEach(func() {
							build.TrackerReturns("http://127.0.0.1:1234")
						})

						Context("when the engine returns a build", func() {
							var engineBuild *enginefakes.FakeBuild
							var streamedBody string

							BeforeEach(func() {
								engineBuild = new(enginefakes.FakeBuild)
								fakeEngine.LookupBuildReturns(engineBuild, nil)

								engineBuild.ReceiveInputStub = func(logger lager.Logger, id atc.PlanID, stream io.ReadCloser) {
									p, err := ioutil.ReadAll(stream)
									Expect(err).ToNot(HaveOccurred())

									streamedBody = string(p)
								}
							})

							It("sends the request body to the plan", func() {
								Expect(engineBuild.ReceiveInputCallCount()).To(Equal(1))

								_, id, _ := engineBuild.ReceiveInputArgsForCall(0)
								Expect(id).To(Equal(atc.PlanID("some-plan")))
								Expect(streamedBody).To(Equal("some-payload"))
							})

							It("returns No Content", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNoContent))
							})

							Context("when the build is initially not tracked", func() {
								BeforeEach(func() {
									build.ReloadReturns(true, nil)

									build.TrackerReturnsOnCall(0, "")
									build.TrackerReturnsOnCall(2, "http://127.0.0.1:1234")
								})

								It("waits until it is", func() {
									Expect(response.StatusCode).To(Equal(http.StatusNoContent))

									Expect(engineBuild.ReceiveInputCallCount()).To(Equal(1))

									_, id, _ := engineBuild.ReceiveInputArgsForCall(0)
									Expect(id).To(Equal(atc.PlanID("some-plan")))
									Expect(streamedBody).To(Equal("some-payload"))
								})
							})
						})

						Context("when the engine returns no build", func() {
							BeforeEach(func() {
								fakeEngine.LookupBuildReturns(nil, errors.New("oh no!"))
							})

							It("returns Internal Server Error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the build is tracked by another ATC", func() {
						BeforeEach(func() {
							build.TrackerReturns(otherTracker.URL())
						})

						It("does not bother looking up the build in the engine", func() {
							Expect(fakeEngine.LookupBuildCallCount()).To(Equal(0))
						})

						It("forwards the request to the other ATC", func() {
							Expect(otherTracker.ReceivedRequests()).To(HaveLen(1))
							Expect(response.StatusCode).To(Equal(http.StatusTeapot))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("im a teapot")))
						})

						Context("when the build is initially not tracked", func() {
							BeforeEach(func() {
								build.ReloadReturns(true, nil)

								build.TrackerReturnsOnCall(0, "")
								build.TrackerReturnsOnCall(2, otherTracker.URL())
							})

							It("waits until it is", func() {
								Expect(fakeEngine.LookupBuildCallCount()).To(Equal(0))

								Expect(otherTracker.ReceivedRequests()).To(HaveLen(1))
								Expect(response.StatusCode).To(Equal(http.StatusTeapot))
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("im a teapot")))
							})
						})
					})
				})

				Context("when accessing other teams build", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthorizedReturns(false)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
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

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not abort the build", func() {
				Expect(otherTracker.ReceivedRequests()).To(BeEmpty())
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/plan/:plan_id/output", func() {
		var (
			otherTracker *ghttp.Server

			response *http.Response
		)

		BeforeEach(func() {
			otherTracker = ghttp.NewServer()

			otherTracker.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds/128/plan/some-plan/output"),
					ghttp.RespondWith(http.StatusTeapot, "im a teapot"),
				),
			)
		})

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("GET", server.URL+"/api/v1/builds/128/plan/some-plan/output", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			otherTracker.Close()
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
			})

			Context("when the build can be found", func() {
				BeforeEach(func() {
					build.TeamNameReturns("some-team")
					dbBuildFactory.BuildReturns(build, true, nil)
				})

				Context("when accessing same team's build", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthorizedReturns(true)
					})

					Context("when the build is tracked by the current ATC", func() {
						BeforeEach(func() {
							build.TrackerReturns("http://127.0.0.1:1234")
						})

						Context("when the engine returns a build", func() {
							var engineBuild *enginefakes.FakeBuild

							BeforeEach(func() {
								engineBuild = new(enginefakes.FakeBuild)
								fakeEngine.LookupBuildReturns(engineBuild, nil)

								engineBuild.SendOutputStub = func(logger lager.Logger, id atc.PlanID, dest io.Writer) {
									fmt.Fprintln(dest, "hello from build")
								}
							})

							It("sends the plan's output to the response body", func() {
								Expect(engineBuild.SendOutputCallCount()).To(Equal(1))

								_, id, _ := engineBuild.SendOutputArgsForCall(0)
								Expect(id).To(Equal(atc.PlanID("some-plan")))
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("hello from build\n")))
							})

							It("returns OK", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							Context("when the build is initially not tracked", func() {
								BeforeEach(func() {
									build.ReloadReturns(true, nil)

									build.TrackerReturnsOnCall(0, "")
									build.TrackerReturnsOnCall(2, "http://127.0.0.1:1234")
								})

								It("waits until it is", func() {
									Expect(response.StatusCode).To(Equal(http.StatusOK))

									Expect(engineBuild.SendOutputCallCount()).To(Equal(1))

									_, id, _ := engineBuild.SendOutputArgsForCall(0)
									Expect(id).To(Equal(atc.PlanID("some-plan")))
									Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("hello from build\n")))
								})
							})
						})

						Context("when the engine returns no build", func() {
							BeforeEach(func() {
								fakeEngine.LookupBuildReturns(nil, errors.New("oh no!"))
							})

							It("returns Internal Server Error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the build is tracked by another ATC", func() {
						BeforeEach(func() {
							build.TrackerReturns(otherTracker.URL())
						})

						It("does not bother looking up the build in the engine", func() {
							Expect(fakeEngine.LookupBuildCallCount()).To(Equal(0))
						})

						It("forwards the request to the other ATC", func() {
							Expect(otherTracker.ReceivedRequests()).To(HaveLen(1))
							Expect(response.StatusCode).To(Equal(http.StatusTeapot))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("im a teapot")))
						})

						Context("when the build is initially not tracked", func() {
							BeforeEach(func() {
								build.ReloadReturns(true, nil)

								build.TrackerReturnsOnCall(0, "")
								build.TrackerReturnsOnCall(2, otherTracker.URL())
							})

							It("waits until it is", func() {
								Expect(fakeEngine.LookupBuildCallCount()).To(Equal(0))

								Expect(otherTracker.ReceivedRequests()).To(HaveLen(1))
								Expect(response.StatusCode).To(Equal(http.StatusTeapot))
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("im a teapot")))
							})
						})
					})
				})

				Context("when accessing other team's build", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthorizedReturns(false)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
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

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not abort the build", func() {
				Expect(otherTracker.ReceivedRequests()).To(BeEmpty())
			})
		})
	})
})
