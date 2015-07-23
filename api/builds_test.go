package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	enginefakes "github.com/concourse/atc/engine/fakes"
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
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when creating a one-off build succeeds", func() {
				BeforeEach(func() {
					buildsDB.CreateOneOffBuildReturns(db.Build{
						ID:           42,
						Name:         "1",
						JobName:      "job1",
						PipelineName: "some-pipeline",
						Status:       db.StatusStarted,
					}, nil)
				})

				Context("and building succeeds", func() {
					var fakeBuild *enginefakes.FakeBuild
					var blockForever *sync.WaitGroup

					BeforeEach(func() {
						fakeBuild = new(enginefakes.FakeBuild)

						blockForever = new(sync.WaitGroup)

						forever := blockForever
						forever.Add(1)

						fakeBuild.ResumeStub = func(lager.Logger) {
							forever.Wait()
						}

						fakeEngine.CreateBuildReturns(fakeBuild, nil)
					})

					AfterEach(func() {
						blockForever.Done()
					})

					It("returns 201 Created", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusCreated))
					})

					It("returns the build", func() {
						body, err := ioutil.ReadAll(response.Body)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(body).Should(MatchJSON(`{
							"id": 42,
							"name": "1",
							"job_name": "job1",
							"status": "started",
							"url": "/pipelines/some-pipeline/jobs/job1/builds/1"
						}`))
					})

					It("creates a one-off build and runs it asynchronously", func() {
						Ω(buildsDB.CreateOneOffBuildCallCount()).Should(Equal(1))

						Ω(fakeEngine.CreateBuildCallCount()).Should(Equal(1))
						oneOff, builtPlan := fakeEngine.CreateBuildArgsForCall(0)
						Ω(oneOff).Should(Equal(db.Build{
							ID:           42,
							Name:         "1",
							JobName:      "job1",
							PipelineName: "some-pipeline",
							Status:       db.StatusStarted,
						}))
						Ω(builtPlan).Should(Equal(plan))

						Ω(fakeBuild.ResumeCallCount()).Should(Equal(1))
					})
				})

				Context("and building fails", func() {
					BeforeEach(func() {
						fakeEngine.CreateBuildReturns(nil, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when creating a one-off build fails", func() {
				BeforeEach(func() {
					buildsDB.CreateOneOffBuildReturns(db.Build{}, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not trigger a build", func() {
				Ω(buildsDB.CreateOneOffBuildCallCount()).Should(BeZero())
				Ω(fakeEngine.CreateBuildCallCount()).Should(BeZero())
			})
		})
	})

	Describe("GET /api/v1/builds", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/builds")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting all builds succeeds", func() {
			BeforeEach(func() {
				buildsDB.GetAllBuildsReturns([]db.Build{
					{
						ID:           3,
						Name:         "2",
						JobName:      "job2",
						PipelineName: "some-pipeline",
						Status:       db.StatusStarted,
					},
					{
						ID:           1,
						Name:         "1",
						JobName:      "job1",
						PipelineName: "some-pipeline",
						Status:       db.StatusSucceeded,
					},
				}, nil)
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns all builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`[
					{
						"id": 3,
						"name": "2",
						"job_name": "job2",
						"status": "started",
						"url": "/pipelines/some-pipeline/jobs/job2/builds/2"
					},
					{
						"id": 1,
						"name": "1",
						"job_name": "job1",
						"status": "succeeded",
						"url": "/pipelines/some-pipeline/jobs/job1/builds/1"
					}
				]`))
			})
		})

		Context("when getting all builds fails", func() {
			BeforeEach(func() {
				buildsDB.GetAllBuildsReturns(nil, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
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
			buildsDB.GetBuildReturns(db.Build{
				ID:      128,
				JobName: "some-job",
			}, nil)

			request, err = http.NewRequest("GET", server.URL+"/api/v1/builds/128/events", nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("returns 200", func() {
				Ω(response.StatusCode).Should(Equal(200))
			})

			It("serves the request via the event handler", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(string(body)).Should(Equal("fake event handler factory was here"))

				Ω(constructedEventHandler.db).Should(Equal(buildsDB))
				Ω(constructedEventHandler.buildID).Should(Equal(128))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("looks up the config from the buildsDB", func() {
				Ω(buildsDB.GetConfigByBuildIDCallCount()).Should(Equal(1))
				buildID := buildsDB.GetConfigByBuildIDArgsForCall(0)
				Ω(buildID).Should(Equal(128))
			})

			Context("and the build is private", func() {
				BeforeEach(func() {
					buildsDB.GetConfigByBuildIDReturns(atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "some-job", Public: false},
						},
					}, 1, nil)
				})

				It("returns 401", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the build is public", func() {
				BeforeEach(func() {
					buildsDB.GetConfigByBuildIDReturns(atc.Config{
						Jobs: atc.JobConfigs{
							{Name: "some-job", Public: true},
						},
					}, 1, nil)
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(200))
				})

				It("serves the request via the event handler", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(string(body)).Should(Equal("fake event handler factory was here"))

					Ω(constructedEventHandler.db).Should(Equal(buildsDB))
					Ω(constructedEventHandler.buildID).Should(Equal(128))
				})
			})
		})
	})

	Describe("POST /api/v1/builds/:build_id/abort", func() {
		var (
			abortTarget *ghttp.Server

			response *http.Response
		)

		BeforeEach(func() {
			abortTarget = ghttp.NewServer()

			abortTarget.AppendHandlers(
				ghttp.VerifyRequest("POST", "/builds/some-guid/abort"),
			)

			buildsDB.GetBuildReturns(db.Build{
				ID:     128,
				Status: db.StatusStarted,
			}, nil)
		})

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds/128/abort", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			abortTarget.Close()
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("and the engine returns a build", func() {
				var fakeBuild *enginefakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(enginefakes.FakeBuild)
					fakeEngine.LookupBuildReturns(fakeBuild, nil)
				})

				It("aborts the build", func() {
					Ω(fakeBuild.AbortCallCount()).Should(Equal(1))
				})

				Context("and aborting succeeds", func() {
					BeforeEach(func() {
						fakeBuild.AbortReturns(nil)
					})

					It("returns 204", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusNoContent))
					})
				})

				Context("and aborting fails", func() {
					BeforeEach(func() {
						fakeBuild.AbortReturns(errors.New("oh no!"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("and the engine returns no build", func() {
				BeforeEach(func() {
					fakeEngine.LookupBuildReturns(nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not abort the build", func() {
				Ω(abortTarget.ReceivedRequests()).Should(BeEmpty())
			})
		})
	})
})
