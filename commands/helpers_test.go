package commands_test

import (
	"fmt"

	"github.com/concourse/atc"
	fakes "github.com/concourse/go-concourse/concourse/fakes"
	. "github.com/concourse/fly/commands"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper Functions", func() {
	Describe("#GetBuild", func() {
		var client *fakes.FakeClient

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
			client = new(fakes.FakeClient)
		})

		Context("when passed a build id", func() {
			Context("when build exists", func() {
				BeforeEach(func() {
					client.BuildReturns(expectedBuild, true, nil)
				})

				It("returns the build", func() {
					build, err := GetBuild(client, "", expectedBuildID, "")
					Expect(err).NotTo(HaveOccurred())
					Expect(build).To(Equal(expectedBuild))
					Expect(client.BuildCallCount()).To(Equal(1))
					Expect(client.BuildArgsForCall(0)).To(Equal(expectedBuildID))
				})
			})

			Context("when a build does not exist", func() {
				BeforeEach(func() {
					client.BuildReturns(atc.Build{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(client, "", expectedBuildID, "")
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
						client.JobReturns(job, true, nil)
					})

					It("returns the next build for that job", func() {
						build, err := GetBuild(client, expectedJobName, "", expectedPipelineName)
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(expectedBuild))
						Expect(client.JobCallCount()).To(Equal(1))
						pipelineName, jobName := client.JobArgsForCall(0)
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
						client.JobReturns(job, true, nil)
					})

					It("returns the finished build for that job", func() {
						build, err := GetBuild(client, expectedJobName, "", expectedPipelineName)
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(expectedBuild))
						Expect(client.JobCallCount()).To(Equal(1))
						pipelineName, jobName := client.JobArgsForCall(0)
						Expect(pipelineName).To(Equal(expectedPipelineName))
						Expect(jobName).To(Equal(expectedJobName))
					})
				})

				Context("when no builds exist", func() {
					BeforeEach(func() {
						job := atc.Job{
							Name: expectedJobName,
						}
						client.JobReturns(job, true, nil)
					})

					It("returns an error", func() {
						_, err := GetBuild(client, expectedJobName, "", expectedPipelineName)
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when job does not exists", func() {
				BeforeEach(func() {
					client.JobReturns(atc.Job{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(client, expectedJobName, "", expectedPipelineName)
					Expect(err).To(MatchError("job not found"))
				})
			})
		})

		Context("when passed pipeline, job, and build names", func() {
			Context("when the build exists", func() {
				BeforeEach(func() {
					client.JobBuildReturns(expectedBuild, true, nil)
				})

				It("returns the build", func() {
					build, err := GetBuild(client, expectedJobName, expectedBuildName, expectedPipelineName)
					Expect(err).NotTo(HaveOccurred())
					Expect(build).To(Equal(expectedBuild))
					Expect(client.JobBuildCallCount()).To(Equal(1))
					pipelineName, jobName, buildName := client.JobBuildArgsForCall(0)
					Expect(pipelineName).To(Equal(expectedPipelineName))
					Expect(buildName).To(Equal(expectedBuildName))
					Expect(jobName).To(Equal(expectedJobName))
				})
			})

			Context("when the build does not exist", func() {
				BeforeEach(func() {
					client.JobBuildReturns(atc.Build{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(client, expectedJobName, expectedBuildName, expectedPipelineName)
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
				client.AllBuildsReturns([]atc.Build{expectedBuild, expectedOneOffBuild}, nil)
			})

			It("returns latest one off build", func() {
				build, err := GetBuild(client, "", "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(build).To(Equal(expectedOneOffBuild))
				Expect(client.AllBuildsCallCount()).To(Equal(1))
			})
		})
	})
})
