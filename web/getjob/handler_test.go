package getjob_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web/group"

	. "github.com/concourse/atc/web/getjob"

	"github.com/concourse/go-concourse/concourse"
	cfakes "github.com/concourse/go-concourse/concourse/fakes"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeClient *cfakes.FakeClient

	var templateData TemplateData
	var fetchErr error

	BeforeEach(func() {
		fakeClient = new(cfakes.FakeClient)
	})

	JustBeforeEach(func() {
		templateData, fetchErr = FetchTemplateData("some-pipeline", fakeClient, "some-job", concourse.Page{
			Since: 398,
			Until: 2,
		})
	})

	It("calls to get the pipeline config", func() {
		Expect(fakeClient.PipelineCallCount()).To(Equal(1))
		Expect(fakeClient.PipelineArgsForCall(0)).To(Equal("some-pipeline"))
	})

	Context("when getting the pipeline returns an error", func() {
		var expectedErr error

		BeforeEach(func() {
			expectedErr = errors.New("disaster")
			fakeClient.PipelineReturns(atc.Pipeline{}, false, expectedErr)
		})

		It("returns an error if the config could not be loaded", func() {
			Expect(fetchErr).To(Equal(expectedErr))
		})
	})

	Context("when the pipeline is not found", func() {
		BeforeEach(func() {
			fakeClient.PipelineReturns(atc.Pipeline{}, false, nil)
		})

		It("returns an error if the config could not be loaded", func() {
			Expect(fetchErr).To(Equal(ErrConfigNotFound))
		})
	})

	Context("when the api returns the pipeline", func() {
		BeforeEach(func() {
			fakeClient.PipelineReturns(atc.Pipeline{
				Groups: atc.GroupConfigs{
					{
						Name: "group-with-job",
						Jobs: []string{"some-job"},
					},
					{
						Name: "group-without-job",
						Jobs: []string{"some-other-job"},
					},
				},
			}, true, nil)
		})

		It("calls to get the job from the client", func() {
			actualPipelineName, actualJobName := fakeClient.JobArgsForCall(0)
			Expect(actualPipelineName).To(Equal("some-pipeline"))
			Expect(actualJobName).To(Equal("some-job"))
		})

		Context("when the client returns an error", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("nope")
				fakeClient.JobReturns(atc.Job{}, false, expectedErr)
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(expectedErr))
			})
		})

		Context("when the job could not be found", func() {
			BeforeEach(func() {
				fakeClient.JobReturns(atc.Job{}, false, nil)
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(ErrJobConfigNotFound))
			})
		})

		Context("when the job is found", func() {
			BeforeEach(func() {
				fakeClient.JobReturns(atc.Job{
					Name: "some-job",
				}, true, nil)
			})

			It("looks up the jobs builds", func() {
				Expect(fakeClient.JobBuildsCallCount()).To(Equal(1))
				pipelineName, jobName, page := fakeClient.JobBuildsArgsForCall(0)

				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(jobName).To(Equal("some-job"))
				Expect(page).To(Equal(concourse.Page{
					Since: 398,
					Until: 2,
				}))
			})

			Context("when the job builds lookup returns an error", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeClient.JobBuildsReturns(nil, concourse.Pagination{}, false, disaster)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(Equal(disaster))
				})
			})

			Context("when the job builds lookup returns a build", func() {
				var buildsWithResources []BuildWithInputsOutputs
				var builds []atc.Build
				var pagination concourse.Pagination

				BeforeEach(func() {
					endTime := time.Now()

					builds = []atc.Build{
						{
							ID:        1,
							Name:      "1",
							JobName:   "some-job",
							Status:    string(atc.StatusSucceeded),
							StartTime: endTime.Add(-24 * time.Second).Unix(),
							EndTime:   endTime.Unix(),
						},
					}

					buildsWithResources = []BuildWithInputsOutputs{
						{
							Build: builds[0],
						},
					}

					pagination = concourse.Pagination{
						Previous: &concourse.Page{
							Until: 42,
							Limit: 100,
						},
						Next: &concourse.Page{
							Since: 43,
							Limit: 100,
						},
					}

					fakeClient.JobBuildsReturns(builds, pagination, true, nil)
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
						disaster := errors.New("Nooooooooooooooooo")

						BeforeEach(func() {
							fakeClient.BuildResourcesReturns(atc.BuildInputsOutputs{}, false, disaster)
						})

						It("returns an error", func() {
							Expect(fetchErr).To(Equal(disaster))
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
								Name: "some-job",
							}

							fakeClient.BuildResourcesReturns(buildResources, true, nil)
						})

						It("populates the inputs and outputs for the builds returned", func() {
							Expect(fakeClient.BuildResourcesCallCount()).To(Equal(1))

							calledBuildID := fakeClient.BuildResourcesArgsForCall(0)
							Expect(calledBuildID).To(Equal(1))

							Expect(templateData.Builds).To(Equal(buildsWithResources))
						})

						It("includes the pagination info in the template", func() {
							Expect(templateData.Pagination).To(Equal(pagination))
						})

						Context("when there is no finished build", func() {
							BeforeEach(func() {
								fakeClient.JobReturns(job, true, nil)
							})

							It("has the correct template data", func() {
								Expect(templateData.GroupStates).To(ConsistOf(groupStates))
								Expect(templateData.Job).To(Equal(job))
								Expect(templateData.Builds).To(Equal(buildsWithResources))
								Expect(templateData.CurrentBuild).To(BeNil())
								Expect(templateData.PipelineName).To(Equal("some-pipeline"))
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
