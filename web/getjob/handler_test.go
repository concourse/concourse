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
	var fakeClient *cfakes.FakeClient
	var fakePaginator *fakes.FakeJobBuildsPaginator

	BeforeEach(func() {
		fakeClient = new(cfakes.FakeClient)
		fakePaginator = new(fakes.FakeJobBuildsPaginator)
	})

	It("calls to get the pipeline config", func() {
		FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
		Expect(fakeClient.PipelineConfigCallCount()).To(Equal(1))
		Expect(fakeClient.PipelineConfigArgsForCall(0)).To(Equal("some-pipeline"))
	})

	Context("when the config database returns an error", func() {
		var expectedErr error
		BeforeEach(func() {
			expectedErr = errors.New("disaster")
			fakeClient.PipelineConfigReturns(atc.Config{}, "", false, expectedErr)
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
			Expect(err).To(Equal(expectedErr))
		})
	})

	Context("when the config database returns no config", func() {
		BeforeEach(func() {
			fakeClient.PipelineConfigReturns(atc.Config{}, "", false, nil)
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
			Expect(err).To(Equal(ErrConfigNotFound))
		})
	})

	Context("when the config database returns a config", func() {
		BeforeEach(func() {
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
			}

			fakeClient.PipelineConfigReturns(config, "", true, nil)
		})

		It("calls to get the job from the client", func() {
			FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "not-a-job-name", 0, false)
			Expect(fakeClient.JobCallCount()).To(Equal(1))
			actualPipelineName, actualJobName := fakeClient.JobArgsForCall(0)
			Expect(actualPipelineName).To(Equal("some-pipeline"))
			Expect(actualJobName).To(Equal("not-a-job-name"))
		})

		Context("when the client returns an error", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("nope")
				fakeClient.JobReturns(atc.Job{}, false, expectedErr)
			})

			It("returns an error", func() {
				_, actualErr := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "not-a-job-name", 0, false)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
			})
		})

		Context("when the job could not be found", func() {
			BeforeEach(func() {
				fakeClient.JobReturns(atc.Job{}, false, nil)
			})

			It("returns an error", func() {
				_, actualErr := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "not-a-job-name", 0, false)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(ErrJobConfigNotFound))
			})
		})

		Context("when the job is found", func() {
			BeforeEach(func() {
				fakeClient.JobReturns(atc.Job{
					Name: "some-job",
				}, true, nil)
			})

			It("looks up the jobs builds", func() {
				_, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 398, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePaginator.PaginateJobBuildsCallCount()).To(Equal(1))

				jobName, startingJobBuildID, resultsGreaterThanStartingID := fakePaginator.PaginateJobBuildsArgsForCall(0)
				Expect(jobName).To(Equal("some-job"))
				Expect(startingJobBuildID).To(Equal(398))
				Expect(resultsGreaterThanStartingID).To(BeTrue())
			})

			Context("when the job builds lookup returns an error", func() {
				It("returns an error if the jobs's builds could not be retreived", func() {
					fakePaginator.PaginateJobBuildsReturns([]db.Build{}, pagination.PaginationData{}, errors.New("disaster"))
					_, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
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
							templateData, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
							Expect(err).To(HaveOccurred())
							Expect(templateData).To(Equal(TemplateData{}))
						})
					})

					Context("when we get inputs and outputs", func() {
						var groupStates []group.State
						var job atc.Job
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

							job = atc.Job{
								Name: "job-name",
							}

							fakeClient.BuildResourcesReturns(buildResources, true, nil)
						})

						It("populates the inputs and outputs for the builds returned", func() {
							templateData, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
							Expect(err).NotTo(HaveOccurred())
							Expect(fakeClient.BuildResourcesCallCount()).To(Equal(1))

							calledBuildID := fakeClient.BuildResourcesArgsForCall(0)
							Expect(calledBuildID).To(Equal(1))

							Expect(templateData.Builds).To(Equal(buildsWithResources))
						})

						Context("when there is no finished build", func() {
							BeforeEach(func() {
								fakeClient.JobReturns(job, true, nil)
							})

							It("has the correct template data", func() {
								templateData, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
								Expect(err).NotTo(HaveOccurred())

								Expect(templateData.GroupStates).To(ConsistOf(groupStates))
								Expect(templateData.Job).To(Equal(job))
								Expect(templateData.Builds).To(Equal(buildsWithResources))
								Expect(templateData.CurrentBuild).To(BeNil())
								Expect(templateData.PipelineName).To(Equal("some-pipeline"))
								Expect(templateData.PaginationData.HasPagination()).To(BeTrue())
							})
						})

						Context("when we have a finished build", func() {

							BeforeEach(func() {
								job.FinishedBuild = &atc.Build{
									ID: 2,
								}
								fakeClient.JobReturns(job, true, nil)
							})

							It("has the correct template data", func() {
								templateData, err := FetchTemplateData("some-pipeline", fakeClient, fakePaginator, "job-name", 0, false)
								Expect(err).NotTo(HaveOccurred())

								Expect(templateData.GroupStates).To(ConsistOf(groupStates))
								Expect(templateData.Job).To(Equal(job))
								Expect(templateData.Builds).To(Equal(buildsWithResources))
								Expect(templateData.CurrentBuild).To(Equal(&atc.Build{
									ID: 2,
								}))
							})
						})
					})
				})
			})
		})
	})
})
