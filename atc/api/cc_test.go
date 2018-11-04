package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cc.xml", func() {
	var (
		requestGenerator *rata.RequestGenerator
		fakeaccess       *accessorfakes.FakeAccess
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(server.URL, atc.Routes)

		fakeaccess = new(accessorfakes.FakeAccess)
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
	})

	Describe("GET /api/v1/teams/:team_name/cc.xml", func() {
		var response *http.Response

		JustBeforeEach(func() {
			req, err := requestGenerator.CreateRequest(atc.GetCC, rata.Params{
				"team_name":     "a-team",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
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
						fakePipeline.NameReturns("something-else")
						fakeTeam.PipelinesReturns([]db.Pipeline{
							fakePipeline,
						}, nil)
					})

					Context("when a job is found", func() {
						var fakeJob *dbfakes.FakeJob
						var endTime time.Time
						BeforeEach(func() {
							fakeJob = new(dbfakes.FakeJob)
							fakeJob.NameReturns("some-job")

							fakePipeline.JobsReturns(db.Jobs{fakeJob}, nil)

							endTime, _ = time.Parse(time.RFC3339, "2018-11-04T21:26:38Z")
						})

						Context("when the last build is successful", func() {
							BeforeEach(func() {
								succeededBuild := new(dbfakes.FakeBuild)
								succeededBuild.StatusReturns(db.BuildStatusSucceeded)
								succeededBuild.IDReturns(42)
								succeededBuild.EndTimeReturns(endTime)
								fakeJob.FinishedAndNextBuildReturns(succeededBuild, nil, nil)
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("returns Content-Type 'application/xml'", func() {
								Expect(response.Header.Get("Content-Type")).To(Equal("application/xml"))
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Success" lastBuildTime="2018-11-04T21:26:38Z" name="something-else :: some-job"/>
</Projects>
`))
							})
						})

						Context("when the last build is aborted", func() {
							BeforeEach(func() {
								abortedBuild := new(dbfakes.FakeBuild)
								abortedBuild.StatusReturns(db.BuildStatusAborted)
								abortedBuild.IDReturns(42)
								abortedBuild.EndTimeReturns(endTime)
								fakeJob.FinishedAndNextBuildReturns(abortedBuild, nil, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Unknown" lastBuildTime="2018-11-04T21:26:38Z" name="something-else :: some-job"/>
</Projects>
`))
							})
						})

						Context("when the last build is errored", func() {
							BeforeEach(func() {
								erroredBuild := new(dbfakes.FakeBuild)
								erroredBuild.StatusReturns(db.BuildStatusErrored)
								erroredBuild.IDReturns(42)
								erroredBuild.EndTimeReturns(endTime)
								fakeJob.FinishedAndNextBuildReturns(erroredBuild, nil, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Exception" lastBuildTime="2018-11-04T21:26:38Z" name="something-else :: some-job"/>
</Projects>
`))
							})
						})

						Context("when the last build is failed", func() {
							BeforeEach(func() {
								failedBuild := new(dbfakes.FakeBuild)
								failedBuild.StatusReturns(db.BuildStatusFailed)
								failedBuild.IDReturns(42)
								failedBuild.EndTimeReturns(endTime)
								fakeJob.FinishedAndNextBuildReturns(failedBuild, nil, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Sleeping" lastBuildLabel="42" lastBuildStatus="Failure" lastBuildTime="2018-11-04T21:26:38Z" name="something-else :: some-job"/>
</Projects>
`))
							})
						})

						Context("when a next build exists", func() {
							BeforeEach(func() {
								finishedBuild := new(dbfakes.FakeBuild)
								finishedBuild.StatusReturns(db.BuildStatusSucceeded)
								finishedBuild.IDReturns(42)
								finishedBuild.EndTimeReturns(endTime)

								nextBuild := new(dbfakes.FakeBuild)

								fakeJob.FinishedAndNextBuildReturns(finishedBuild, nextBuild, nil)
							})

							It("returns the CC.xml", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML(`
<Projects>
  <Project activity="Building" lastBuildLabel="42" lastBuildStatus="Success" lastBuildTime="2018-11-04T21:26:38Z" name="something-else :: some-job"/>
</Projects>
`))
							})
						})

						Context("when no last build exists", func() {
							BeforeEach(func() {
								fakeJob.FinishedAndNextBuildReturns(nil, nil, nil)
							})

							It("returns the CC.xml without the job", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchXML("<Projects></Projects>"))
							})
						})

						Context("when finding the last build fails", func() {
							BeforeEach(func() {
								fakeJob.FinishedAndNextBuildReturns(nil, nil, errors.New("failed"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when no job is found", func() {
						BeforeEach(func() {
							fakePipeline.JobsReturns(db.Jobs{}, nil)
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
							fakePipeline.JobsReturns(nil, errors.New("failed"))
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
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
