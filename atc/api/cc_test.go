package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cc.xml", func() {
	var (
		requestGenerator *rata.RequestGenerator
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(server.URL, atc.Routes)
	})

	Describe("GET /api/v1/teams/:team_name/cc.xml", func() {
		var response *http.Response

		JustBeforeEach(func() {
			req, err := requestGenerator.CreateRequest(atc.GetCC, rata.Params{
				"team_name": "a-team",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when the team is found", func() {
				var fakeTeam *dbfakes.FakeTeam
				BeforeEach(func() {
					fakeTeam = new(dbfakes.FakeTeam)
					fakeTeam.NameReturns("a-team")
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				Context("when a pipeline is found", func() {
					var fakePipeline *dbfakes.FakePipeline
					BeforeEach(func() {
						fakePipeline = new(dbfakes.FakePipeline)
						fakeTeam.PipelinesReturns([]db.Pipeline{
							fakePipeline,
						}, nil)
					})

					Context("when a job is found", func() {
						var endTime time.Time
						BeforeEach(func() {
							fakePipeline.DashboardReturns(atc.Dashboard{
								{
									Name:         "some-job",
									PipelineName: "something-else",
									TeamName:     "a-team",
								},
							}, nil)

							endTime, _ = time.Parse(time.RFC3339, "2018-11-04T21:26:38Z")
						})

						Context("when the last build is successful", func() {
							BeforeEach(func() {
								fakePipeline.DashboardReturns(atc.Dashboard{
									{
										Name:         "some-job",
										PipelineName: "something-else",
										TeamName:     "a-team",
										FinishedBuild: &atc.DashboardBuild{
											Name:    "42",
											Status:  "succeeded",
											EndTime: endTime,
										},
									},
								}, nil)
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("returns Content-Type 'application/xml'", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/xml",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Success" lastBuildTime="2018-11-04T21:26:38Z" name="something-else/some-job" webUrl="https://example.com/teams/a-team/pipelines/something-else/jobs/some-job"/>
</Projects>
`))
							})
						})

						Context("when the last build is aborted", func() {
							BeforeEach(func() {
								fakePipeline.DashboardReturns(atc.Dashboard{
									{
										Name:         "some-job",
										PipelineName: "something-else",
										TeamName:     "a-team",
										FinishedBuild: &atc.DashboardBuild{
											Name:    "42",
											Status:  "aborted",
											EndTime: endTime,
										},
									},
								}, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Exception" lastBuildTime="2018-11-04T21:26:38Z" name="something-else/some-job" webUrl="https://example.com/teams/a-team/pipelines/something-else/jobs/some-job"/>
</Projects>
`))
							})
						})

						Context("when the last build is errored", func() {
							BeforeEach(func() {
								fakePipeline.DashboardReturns(atc.Dashboard{
									{
										Name:         "some-job",
										PipelineName: "something-else",
										TeamName:     "a-team",
										FinishedBuild: &atc.DashboardBuild{
											Name:    "42",
											Status:  "errored",
											EndTime: endTime,
										},
									},
								}, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Exception" lastBuildTime="2018-11-04T21:26:38Z" name="something-else/some-job" webUrl="https://example.com/teams/a-team/pipelines/something-else/jobs/some-job"/>
</Projects>
`))
							})
						})

						Context("when the last build is failed", func() {
							BeforeEach(func() {
								fakePipeline.DashboardReturns(atc.Dashboard{
									{
										Name:         "some-job",
										PipelineName: "something-else",
										TeamName:     "a-team",
										FinishedBuild: &atc.DashboardBuild{
											Name:    "42",
											Status:  "failed",
											EndTime: endTime,
										},
									},
								}, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Failure" lastBuildTime="2018-11-04T21:26:38Z" name="something-else/some-job" webUrl="https://example.com/teams/a-team/pipelines/something-else/jobs/some-job"/>
</Projects>
`))
							})
						})

						Context("when a next build exists", func() {
							BeforeEach(func() {
								fakePipeline.DashboardReturns(atc.Dashboard{
									{
										Name:         "some-job",
										PipelineName: "something-else",
										TeamName:     "a-team",
										FinishedBuild: &atc.DashboardBuild{
											Name:    "42",
											Status:  "succeeded",
											EndTime: endTime,
										},
										NextBuild: &atc.DashboardBuild{},
									},
								}, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Building" lastBuildLabel="42" lastBuildStatus="Success" lastBuildTime="2018-11-04T21:26:38Z" name="something-else/some-job" webUrl="https://example.com/teams/a-team/pipelines/something-else/jobs/some-job"/>
</Projects>
`))
							})
						})

						Context("when no last build exists", func() {
							It("returns the CC.xml without the job", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML("<Projects></Projects>"))
							})
						})
					})

					Context("when no job is found", func() {
						BeforeEach(func() {
							fakePipeline.DashboardReturns(atc.Dashboard{}, nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns the CC.xml", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchXML("<Projects></Projects>"))
						})
					})

					Context("when finding the jobs fails", func() {
						BeforeEach(func() {
							fakePipeline.DashboardReturns(nil, errors.New("failed"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when no pipeline is found", func() {
					BeforeEach(func() {
						fakeTeam.PipelinesReturns([]db.Pipeline{}, nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns the CC.xml", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchXML("<Projects></Projects>"))
					})
				})

				Context("when getting the pipelines fails", func() {
					BeforeEach(func() {
						fakeTeam.PipelinesReturns(nil, errors.New("failed"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when the team is not found", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when finding the team fails", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, errors.New("failed"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
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
})
