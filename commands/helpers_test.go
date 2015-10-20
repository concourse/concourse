package commands_test

import (
	"fmt"

	"github.com/concourse/atc"
	fakes "github.com/concourse/fly/atcclient/fakes"
	. "github.com/concourse/fly/commands"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper Functions", func() {
	Describe("#GetBuild", func() {
		var handler *fakes.FakeHandler

		expectedBuildID := "123"
		expectedBuildName := "5"
		expectedJobName := "myjob"
		expectedPipelineName := "mypipeline"
		expectedBuild := atc.Build{
			ID:      123,
			Name:    expectedBuildName,
			Status:  "Great Success",
			JobName: expectedJobName,
			URL:     fmt.Sprintf("/pipelines/%s/jobs/%s/builds/%s", expectedPipelineName, expectedJobName, expectedBuildName),
			ApiUrl:  fmt.Sprintf("api/v1/builds/%s", expectedBuildID),
		}

		BeforeEach(func() {
			handler = new(fakes.FakeHandler)
		})

		Context("when passed a build id", func() {
			Context("when build exists", func() {
				BeforeEach(func() {
					handler.BuildReturns(expectedBuild, true, nil)
				})

				It("returns the build", func() {
					build, err := GetBuild(handler, "", expectedBuildID, "")
					Expect(err).NotTo(HaveOccurred())
					Expect(build).To(Equal(expectedBuild))
					Expect(handler.BuildCallCount()).To(Equal(1))
					Expect(handler.BuildArgsForCall(0)).To(Equal(expectedBuildID))
				})
			})

			Context("when a build does not exist", func() {
				BeforeEach(func() {
					handler.BuildReturns(atc.Build{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(handler, "", expectedBuildID, "")
					Expect(err).To(MatchError("build not found"))
				})
			})
		})

		Context("when passed a pipeline and job name", func() {
			Context("when job exists", func() {
				Context("when the next build exists", func() {
					BeforeEach(func() {
						job := atc.Job{
							Name:      expectedJobName,
							NextBuild: &expectedBuild,
						}
						handler.JobReturns(job, true, nil)
					})

					It("returns the next build for that job", func() {
						build, err := GetBuild(handler, expectedJobName, "", expectedPipelineName)
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(expectedBuild))
						Expect(handler.JobCallCount()).To(Equal(1))
						pipelineName, jobName := handler.JobArgsForCall(0)
						Expect(pipelineName).To(Equal(expectedPipelineName))
						Expect(jobName).To(Equal(expectedJobName))
					})
				})

				Context("when the only the finished build exists", func() {
					BeforeEach(func() {
						job := atc.Job{
							Name:          expectedJobName,
							FinishedBuild: &expectedBuild,
						}
						handler.JobReturns(job, true, nil)
					})

					It("returns the finished build for that job", func() {
						build, err := GetBuild(handler, expectedJobName, "", expectedPipelineName)
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(expectedBuild))
						Expect(handler.JobCallCount()).To(Equal(1))
						pipelineName, jobName := handler.JobArgsForCall(0)
						Expect(pipelineName).To(Equal(expectedPipelineName))
						Expect(jobName).To(Equal(expectedJobName))
					})
				})

				Context("when no builds exist", func() {
					BeforeEach(func() {
						job := atc.Job{
							Name: expectedJobName,
						}
						handler.JobReturns(job, true, nil)
					})

					It("returns an error", func() {
						_, err := GetBuild(handler, expectedJobName, "", expectedPipelineName)
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when job does not exists", func() {
				BeforeEach(func() {
					handler.JobReturns(atc.Job{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(handler, expectedJobName, "", expectedPipelineName)
					Expect(err).To(MatchError("job not found"))
				})
			})
		})

		Context("when passed pipeline, job, and build names", func() {
			Context("when the build exists", func() {
				BeforeEach(func() {
					handler.JobBuildReturns(expectedBuild, true, nil)
				})

				It("returns the build", func() {
					build, err := GetBuild(handler, expectedJobName, expectedBuildName, expectedPipelineName)
					Expect(err).NotTo(HaveOccurred())
					Expect(build).To(Equal(expectedBuild))
					Expect(handler.JobBuildCallCount()).To(Equal(1))
					pipelineName, jobName, buildName := handler.JobBuildArgsForCall(0)
					Expect(pipelineName).To(Equal(expectedPipelineName))
					Expect(buildName).To(Equal(expectedBuildName))
					Expect(jobName).To(Equal(expectedJobName))
				})
			})

			Context("when the build does not exist", func() {
				BeforeEach(func() {
					handler.JobBuildReturns(atc.Build{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(handler, expectedJobName, expectedBuildName, expectedPipelineName)
					Expect(err).To(MatchError("build not found"))
				})
			})
		})

		Context("when nothing is passed", func() {
			expectedOneOffBuild := atc.Build{
				ID:      123,
				Name:    expectedBuildName,
				Status:  "Great Success",
				JobName: "",
				URL:     fmt.Sprintf("/builds/%s", expectedBuildID),
				ApiUrl:  fmt.Sprintf("api/v1/builds/%s", expectedBuildID),
			}

			BeforeEach(func() {
				handler.AllBuildsReturns([]atc.Build{expectedBuild, expectedOneOffBuild}, nil)
			})

			It("returns latest one off build", func() {
				build, err := GetBuild(handler, "", "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(build).To(Equal(expectedOneOffBuild))
				Expect(handler.AllBuildsCallCount()).To(Equal(1))
			})
		})
	})
})
