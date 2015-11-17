package getjob_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"

	. "github.com/concourse/atc/web/getjob"
	"github.com/concourse/atc/web/getjob/fakes"
	"github.com/concourse/atc/web/pagination"

	cfakes "github.com/concourse/go-concourse/concourse/fakes"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeDB *fakes.FakeJobDB
	var fakeClient *cfakes.FakeClient
	var fakePaginator *fakes.FakeJobBuildsPaginator

	BeforeEach(func() {
		fakeDB = new(fakes.FakeJobDB)
		fakeClient = new(cfakes.FakeClient)
		fakePaginator = new(fakes.FakeJobBuildsPaginator)
	})

	Context("when the config database returns an error", func() {
		var expectedError error
		BeforeEach(func() {
			expectedError = errors.New("disaster")
			fakeClient.PipelineConfigReturns(atc.Config{}, "", false, expectedError)
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedError))
		})
	})

	Context("when the config database returns no config", func() {
		BeforeEach(func() {
			fakeClient.PipelineConfigReturns(atc.Config{}, "", false, nil)
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
			Expect(err).To(HaveOccurred())

			Expect(fakeClient.PipelineConfigCallCount()).To(Equal(1))
			pipelineName := fakeClient.PipelineConfigArgsForCall(0)
			Expect(pipelineName).To(Equal("some-pipeline"))
		})
	})

	Context("when the config database returns a config", func() {
		var job atc.JobConfig

		BeforeEach(func() {
			job = atc.JobConfig{
				Name: "job-name",
			}
			config := atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name: "group-with-job",
						Jobs: []string{"job-name"},
					},
					{
						Name: "group-without-job",
					},
				},
				Jobs: []atc.JobConfig{
					job,
				},
			}

			fakeClient.PipelineConfigReturns(config, "", true, nil)
		})

		It("returns not found if the job cannot be found in the config", func() {
			_, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "not-a-job-name", 0, false)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ErrJobConfigNotFound))
		})

		Context("when the job can be found in the config", func() {
			It("looks up the jobs builds", func() {
				_, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 398, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePaginator.PaginateJobBuildsCallCount()).To(Equal(1))

				jobName, startingJobBuildID, resultsGreaterThanStartingID := fakePaginator.PaginateJobBuildsArgsForCall(0)
				Expect(jobName).To(Equal("job-name"))
				Expect(startingJobBuildID).To(Equal(398))
				Expect(resultsGreaterThanStartingID).To(BeTrue())
			})

			Context("when the job builds lookup returns an error", func() {
				It("returns an error if the jobs's builds could not be retreived", func() {
					fakePaginator.PaginateJobBuildsReturns([]db.Build{}, pagination.PaginationData{}, errors.New("disaster"))
					_, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the job builds lookup returns a build", func() {
				var buildsWithResources []BuildWithInputsOutputs
				var builds []db.Build
				var paginationData pagination.PaginationData

				BeforeEach(func() {
					endTime := time.Now()

					builds = []db.Build{
						{
							ID:        1,
							Name:      "1",
							JobName:   "job-name",
							Status:    db.StatusSucceeded,
							StartTime: endTime.Add(-24 * time.Second),
							EndTime:   endTime,
						},
					}

					buildsWithResources = []BuildWithInputsOutputs{
						{
							Build: builds[0],
						},
					}

					paginationData = pagination.NewPaginationData(true, false, 0, 0, 0)
					fakePaginator.PaginateJobBuildsReturns(builds, paginationData, nil)
				})

				Context("when the get job lookup returns an error", func() {
					It("returns an error", func() {
						fakeDB.GetJobReturns(db.SavedJob{}, errors.New("disaster"))
						_, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
						Expect(err).To(HaveOccurred())
					})

					Context("when getting inputs and outputs for a build", func() {
						var buildResources atc.BuildInputsOutputs

						BeforeEach(func() {
							buildResources = atc.BuildInputsOutputs{
								Inputs: []atc.PublicBuildInput{
									{
										Resource: "some-input-resource",
										Version: atc.Version{
											"some": "version",
										},
									},
								},
								Outputs: []atc.VersionedResource{
									{
										Resource: "some-output-resource",
										Version: atc.Version{
											"some": "version",
										},
									},
								},
							}

							buildsWithResources = []BuildWithInputsOutputs{
								{
									Build:     builds[0],
									Resources: buildResources,
								},
							}

						})

						Context("when get build resources returns an error", func() {
							BeforeEach(func() {
								fakeClient.BuildResourcesReturns(atc.BuildInputsOutputs{}, false, errors.New("Nooooooooooooooooo"))
							})

							It("returns an error", func() {
								templateData, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
								Expect(err).To(HaveOccurred())
								Expect(templateData).To(Equal(TemplateData{}))
							})
						})

						Context("when we get inputs and outputs", func() {
							BeforeEach(func() {
								fakeClient.BuildResourcesReturns(buildResources, true, nil)
							})

							It("populates the inputs and outputs for the builds returned", func() {
								templateData, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
								Expect(err).NotTo(HaveOccurred())
								Expect(fakeClient.BuildResourcesCallCount()).To(Equal(1))

								calledBuildID := fakeClient.BuildResourcesArgsForCall(0)
								Expect(calledBuildID).To(Equal(1))

								Expect(templateData.Builds).To(Equal(buildsWithResources))
							})
						})
					})

					Context("when the get job lookup returns a job", func() {
						var groupStates []group.State
						var dbJob db.SavedJob

						BeforeEach(func() {
							groupStates = []group.State{
								{
									Name:    "group-with-job",
									Enabled: true,
								},
								{
									Name:    "group-without-job",
									Enabled: false,
								},
							}

							dbJob = db.SavedJob{
								Paused: false,
								Job: db.Job{
									Name: "some-job",
								},
							}

							fakeDB.GetJobReturns(dbJob, nil)
							fakeDB.GetPipelineNameReturns("some-pipeline")
							fakeClient.BuildResourcesReturns(atc.BuildInputsOutputs{}, true, nil)
						})

						Context("when there is no current build", func() {
							BeforeEach(func() {
								fakeDB.GetCurrentBuildReturns(db.Build{}, false, nil)
							})

							It("has the correct template data with no current build", func() {
								templateData, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
								Expect(err).NotTo(HaveOccurred())

								Expect(templateData.GroupStates).To(ConsistOf(groupStates))
								Expect(templateData.Job).To(Equal(job))
								Expect(templateData.DBJob).To(Equal(dbJob))
								Expect(templateData.Builds).To(Equal(buildsWithResources))
								Expect(templateData.CurrentBuild).To(BeNil())
								Expect(templateData.PipelineName).To(Equal("some-pipeline"))
								Expect(templateData.PaginationData.HasPagination()).To(BeTrue())
							})
						})

						Context("when the current build is found", func() {
							var currentBuild db.Build

							BeforeEach(func() {
								currentBuild = db.Build{
									ID:      1,
									Name:    "1",
									JobName: "job-name",
									Status:  db.StatusSucceeded,
								}

								fakeDB.GetCurrentBuildReturns(currentBuild, true, nil)
							})

							It("has the correct template data", func() {
								templateData, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
								Expect(err).NotTo(HaveOccurred())

								Expect(templateData.GroupStates).To(ConsistOf(groupStates))
								Expect(templateData.Job).To(Equal(job))
								Expect(templateData.DBJob).To(Equal(dbJob))
								Expect(templateData.Builds).To(Equal(buildsWithResources))
								Expect(templateData.CurrentBuild).To(Equal(&currentBuild))
							})

							Context("when the job is paused", func() {
								BeforeEach(func() {
									dbJob = db.SavedJob{
										Paused: true,
										Job: db.Job{
											Name: "some-job",
										},
									}
									fakeDB.GetJobReturns(dbJob, nil)
								})

								It("has the correct template data and sets the current build status to paused", func() {
									templateData, err := FetchTemplateData(fakeClient, "some-pipeline", fakeDB, fakePaginator, "job-name", 0, false)
									Expect(err).NotTo(HaveOccurred())

									Expect(templateData.GroupStates).To(ConsistOf(groupStates))
									Expect(templateData.Job).To(Equal(job))
									Expect(templateData.DBJob).To(Equal(dbJob))
									Expect(templateData.Builds).To(Equal(buildsWithResources))
									Expect(templateData.CurrentBuild).To(Equal(&currentBuild))
								})
							})
						})
					})
				})
			})
		})
	})
})
