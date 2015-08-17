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
)

var _ = Describe("FetchTemplateData", func() {
	var fakeDB *fakes.FakeJobDB
	var fakePaginator *fakes.FakeJobBuildsPaginator

	BeforeEach(func() {
		fakeDB = new(fakes.FakeJobDB)
		fakePaginator = new(fakes.FakeJobBuildsPaginator)
	})

	Context("when the config database returns an error", func() {
		BeforeEach(func() {
			fakeDB.GetConfigReturns(atc.Config{}, db.ConfigVersion(1), errors.New("disaster"))
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
			Ω(err).Should(HaveOccurred())
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

			fakeDB.GetConfigReturns(config, db.ConfigVersion(1), nil)
		})

		It("returns not found if the job cannot be found in the config", func() {
			_, err := FetchTemplateData(fakeDB, fakePaginator, "not-a-job-name", 0, false)
			Ω(err).Should(HaveOccurred())
			Ω(err).Should(MatchError(ErrJobConfigNotFound))
		})

		Context("when the job can be found in the config", func() {
			It("looks up the jobs builds", func() {
				_, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 398, true)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakePaginator.PaginateJobBuildsCallCount()).Should(Equal(1))

				jobName, startingJobBuildID, resultsGreaterThanStartingID := fakePaginator.PaginateJobBuildsArgsForCall(0)
				Ω(jobName).Should(Equal("job-name"))
				Ω(startingJobBuildID).Should(Equal(398))
				Ω(resultsGreaterThanStartingID).Should(BeTrue())
			})

			Context("when the job builds lookup returns an error", func() {
				It("returns an error if the jobs's builds could not be retreived", func() {
					fakePaginator.PaginateJobBuildsReturns([]db.Build{}, pagination.PaginationData{}, errors.New("disaster"))
					_, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
					Ω(err).Should(HaveOccurred())
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
						_, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
						Ω(err).Should(HaveOccurred())
					})

					Context("when getting inputs and outputs for a build", func() {
						var inputs []db.BuildInput
						var outputs []db.BuildOutput

						BeforeEach(func() {
							inputs = []db.BuildInput{
								{
									Name: "input1",
								},
							}
							outputs = []db.BuildOutput{
								{
									db.VersionedResource{
										Resource: "some-resource",
									},
								},
							}

							buildsWithResources = []BuildWithInputsOutputs{
								{
									Build:   builds[0],
									Inputs:  inputs,
									Outputs: outputs,
								},
							}

						})

						Context("when get build resources returns an error", func() {
							BeforeEach(func() {
								fakeDB.GetBuildResourcesReturns([]db.BuildInput{}, []db.BuildOutput{}, errors.New("some-error"))
							})

							It("returns an error", func() {
								templateData, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
								Ω(err).Should(HaveOccurred())
								Ω(templateData).Should(Equal(TemplateData{}))
							})
						})

						Context("when we get inputs and outputs", func() {
							BeforeEach(func() {
								fakeDB.GetBuildResourcesReturns(inputs, outputs, nil)
							})

							It("populates the inputs and outputs for the builds returned", func() {
								templateData, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(fakeDB.GetBuildResourcesCallCount()).Should(Equal(1))

								calledBuildID := fakeDB.GetBuildResourcesArgsForCall(0)
								Ω(calledBuildID).Should(Equal(1))

								Ω(templateData.Builds).Should(Equal(buildsWithResources))
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
						})

						Context("when the current build lookup returns an error", func() {
							It("has the correct template data and sets the current build status to pending", func() {
								fakeDB.GetCurrentBuildReturns(db.Build{}, errors.New("No current build"))

								templateData, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(templateData.GroupStates).Should(ConsistOf(groupStates))
								Ω(templateData.Job).Should(Equal(job))
								Ω(templateData.DBJob).Should(Equal(dbJob))
								Ω(templateData.Builds).Should(Equal(buildsWithResources))
								Ω(templateData.CurrentBuild).Should(Equal(db.Build{
									Status: db.StatusPending,
								}))
								Ω(templateData.PipelineName).Should(Equal("some-pipeline"))
								Ω(templateData.PaginationData.HasPagination()).Should(BeTrue())
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

								fakeDB.GetCurrentBuildReturns(currentBuild, nil)
							})

							It("has the correct template data", func() {
								templateData, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(templateData.GroupStates).Should(ConsistOf(groupStates))
								Ω(templateData.Job).Should(Equal(job))
								Ω(templateData.DBJob).Should(Equal(dbJob))
								Ω(templateData.Builds).Should(Equal(buildsWithResources))
								Ω(templateData.CurrentBuild).Should(Equal(currentBuild))
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
									templateData, err := FetchTemplateData(fakeDB, fakePaginator, "job-name", 0, false)
									Ω(err).ShouldNot(HaveOccurred())

									Ω(templateData.GroupStates).Should(ConsistOf(groupStates))
									Ω(templateData.Job).Should(Equal(job))
									Ω(templateData.DBJob).Should(Equal(dbJob))
									Ω(templateData.Builds).Should(Equal(buildsWithResources))
									Ω(templateData.CurrentBuild).Should(Equal(currentBuild))
								})
							})
						})
					})
				})

			})
		})

	})
})
