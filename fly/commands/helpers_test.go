package commands_test

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"

	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/fly/commands"
	"github.com/concourse/concourse/fly/rc/rcfakes"
	"github.com/concourse/concourse/go-concourse/concourse"
	fakes "github.com/concourse/concourse/go-concourse/concourse/concoursefakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper Functions", func() {
	Describe("#GetBuild", func() {
		var client *fakes.FakeClient
		var team *fakes.FakeTeam
		expectedBuildID := "123"
		expectedBuildName := "5"
		expectedJobName := "myjob"
		expectedPipelineName := "mypipeline"
		expectedBuild := atc.Build{
			ID:      123,
			Name:    expectedBuildName,
			Status:  "Great Success",
			JobName: expectedJobName,
			APIURL:  fmt.Sprintf("api/v1/builds/%s", expectedBuildID),
		}

		BeforeEach(func() {
			client = new(fakes.FakeClient)
			team = new(fakes.FakeTeam)
		})

		Context("when passed a build id", func() {
			Context("when no error is encountered while fetching build", func() {
				Context("when build exists", func() {
					BeforeEach(func() {
						client.BuildReturns(expectedBuild, true, nil)
					})

					It("returns the build", func() {
						build, err := GetBuild(client, nil, "", expectedBuildID, "")
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
						_, err := GetBuild(client, nil, "", expectedBuildID, "")
						Expect(err).To(HaveOccurred())
						Expect(err).To(MatchError("build not found"))
					})
				})
			})

			Context("when an error is encountered while fetching build", func() {
				BeforeEach(func() {
					client.BuildReturns(atc.Build{}, false, errors.New("some-error"))
				})

				It("return an error", func() {
					_, err := GetBuild(client, nil, "", expectedBuildID, "")
					Expect(err).To(MatchError("failed to get build some-error"))
				})
			})
		})

		Context("when passed a pipeline and job name", func() {
			Context("when no error was encountered while looking up for team job", func() {
				Context("when job exists", func() {
					Context("when the next build exists", func() {
						BeforeEach(func() {
							job := atc.Job{
								Name:      expectedJobName,
								NextBuild: &expectedBuild,
							}
							team.JobReturns(job, true, nil)
						})

						It("returns the next build for that job", func() {
							build, err := GetBuild(client, team, expectedJobName, "", expectedPipelineName)
							Expect(err).NotTo(HaveOccurred())
							Expect(build).To(Equal(expectedBuild))
							Expect(team.JobCallCount()).To(Equal(1))
							pipelineName, jobName := team.JobArgsForCall(0)
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
							team.JobReturns(job, true, nil)
						})

						It("returns the finished build for that job", func() {
							build, err := GetBuild(client, team, expectedJobName, "", expectedPipelineName)
							Expect(err).NotTo(HaveOccurred())
							Expect(build).To(Equal(expectedBuild))
							Expect(team.JobCallCount()).To(Equal(1))
							pipelineName, jobName := team.JobArgsForCall(0)
							Expect(pipelineName).To(Equal(expectedPipelineName))
							Expect(jobName).To(Equal(expectedJobName))
						})
					})

					Context("when no builds exist", func() {
						BeforeEach(func() {
							job := atc.Job{
								Name: expectedJobName,
							}
							team.JobReturns(job, true, nil)
						})

						It("returns an error", func() {
							_, err := GetBuild(client, team, expectedJobName, "", expectedPipelineName)
							Expect(err).To(HaveOccurred())
						})
					})
				})

				Context("when job does not exists", func() {
					BeforeEach(func() {
						team.JobReturns(atc.Job{}, false, nil)
					})

					It("returns an error", func() {
						_, err := GetBuild(client, team, expectedJobName, "", expectedPipelineName)
						Expect(err).To(MatchError("job not found"))
					})
				})
			})

			Context("when an error was encountered while looking up for team job", func() {
				BeforeEach(func() {
					team.JobReturns(atc.Job{}, false, errors.New("some-error"))
				})

				It("should return an error", func() {
					_, err := GetBuild(client, team, expectedJobName, "", "")
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("failed to get job some-error"))
				})
			})

		})

		Context("when passed pipeline, job, and build names", func() {
			Context("when the build exists", func() {
				BeforeEach(func() {
					team.JobBuildReturns(expectedBuild, true, nil)
				})

				It("returns the build", func() {
					build, err := GetBuild(client, team, expectedJobName, expectedBuildName, expectedPipelineName)
					Expect(err).NotTo(HaveOccurred())
					Expect(build).To(Equal(expectedBuild))
					Expect(team.JobBuildCallCount()).To(Equal(1))
					pipelineName, jobName, buildName := team.JobBuildArgsForCall(0)
					Expect(pipelineName).To(Equal(expectedPipelineName))
					Expect(buildName).To(Equal(expectedBuildName))
					Expect(jobName).To(Equal(expectedJobName))
				})
			})

			Context("when the build does not exist", func() {
				BeforeEach(func() {
					team.JobBuildReturns(atc.Build{}, false, nil)
				})

				It("returns an error", func() {
					_, err := GetBuild(client, team, expectedJobName, expectedBuildName, expectedPipelineName)
					Expect(err).To(MatchError("build not found"))
				})
			})
		})

		Context("when nothing is passed", func() {
			Context("when client.Builds does not return an error", func() {
				var allBuilds [300]atc.Build

				expectedOneOffBuild := atc.Build{
					ID:      150,
					Name:    expectedBuildName,
					Status:  "success",
					JobName: "",
					APIURL:  fmt.Sprintf("api/v1/builds/%s", expectedBuildID),
				}

				Context("when a build was found", func() {
					BeforeEach(func() {
						for i := 300 - 1; i >= 0; i-- {
							allBuilds[i] = atc.Build{
								ID:      i,
								Name:    strconv.Itoa(i),
								JobName: "some-job",
								APIURL:  fmt.Sprintf("api/v1/builds/%d", i),
							}
						}

						allBuilds[150] = expectedOneOffBuild

						client.BuildsStub = func(page concourse.Page) ([]atc.Build, concourse.Pagination, error) {
							var builds []atc.Build
							if page.To != 0 {
								builds = allBuilds[page.To : page.To+page.Limit]
							} else {
								builds = allBuilds[0:page.Limit]
							}

							pagination := concourse.Pagination{
								Previous: &concourse.Page{
									Limit: page.Limit,
									From:  builds[0].ID,
								},
								Next: &concourse.Page{
									Limit: page.Limit,
									To:    builds[len(builds)-1].ID,
								},
							}

							return builds, pagination, nil
						}
					})

					It("returns latest one off build", func() {
						build, err := GetBuild(client, nil, "", "", "")
						Expect(err).NotTo(HaveOccurred())
						Expect(build).To(Equal(expectedOneOffBuild))
						Expect(client.BuildsCallCount()).To(Equal(2))
					})
				})

				Context("when no builds were found ", func() {
					BeforeEach(func() {
						client.BuildsReturns([]atc.Build{}, concourse.Pagination{Next: nil}, nil)
					})

					It("returns an error", func() {
						_, err := GetBuild(client, nil, "", "", "")
						Expect(err).To(HaveOccurred())
						Expect(err).To(MatchError("no builds match job"))
					})
				})
			})

			Context("when client.Builds returns an error", func() {
				BeforeEach(func() {
					client.BuildsReturns(nil, concourse.Pagination{}, errors.New("some-error"))
				})

				It("should return an error", func() {
					_, err := GetBuild(client, nil, "", "", "")
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("failed to get builds some-error"))
				})
			})
		})
	})
	Describe("#GetLatestResourceVersions", func() {
		var team *fakes.FakeTeam
		var resourceVersions []atc.ResourceVersion

		resource := flaghelpers.ResourceFlag{
			PipelineName: "mypipeline",
			ResourceName: "myresource",
		}

		BeforeEach(func() {
			team = new(fakes.FakeTeam)
			resourceVersions = []atc.ResourceVersion{
				{
					ID:      1,
					Version: atc.Version{"version": "v1"},
				},
				{
					ID:      2,
					Version: atc.Version{"version": "v1"},
				},
			}
		})

		When("resource versions exist", func() {
			It("returns latest resource version", func() {
				team.ResourceVersionsReturns(resourceVersions, concourse.Pagination{}, true, nil)
				latestResourceVersion, err := GetLatestResourceVersion(team, resource, atc.Version{"version": "v1"})
				Expect(err).NotTo(HaveOccurred())
				Expect(latestResourceVersion.Version).To(Equal(atc.Version{"version": "v1"}))
				Expect(latestResourceVersion.ID).To(Equal(1))
			})
		})

		When("call to resource versions returns an error", func() {
			It("returns an error", func() {
				team.ResourceVersionsReturns(nil, concourse.Pagination{}, false, errors.New("fake error"))
				_, err := GetLatestResourceVersion(team, resource, atc.Version{"version": "v1"})
				Expect(err).To(MatchError("fake error"))
			})
		})

		When("call to resource versions returns an empty array", func() {
			It("returns an error", func() {
				team.ResourceVersionsReturns([]atc.ResourceVersion{}, concourse.Pagination{}, true, nil)
				_, err := GetLatestResourceVersion(team, resource, atc.Version{"version": "v2"})
				Expect(err).To(MatchError("could not find version matching {\"version\":\"v2\"}"))
			})
		})
	})

	Describe("#GetTeam", func() {
		var target *rcfakes.FakeTarget
		var client *fakes.FakeClient

		BeforeEach(func() {
			target = new(rcfakes.FakeTarget)
			client = new(fakes.FakeClient)
			target.ClientReturns(client)
		})

		It("gets the team", func() {
			GetTeam(target, "team")

			Expect(client.TeamCallCount()).To(Equal(1), "target.Client().Team should be called once")
			Expect(client.TeamArgsForCall(0)).To(Equal("team"))
		})

		It("returns the target default if no team is provided", func() {
			GetTeam(target, "")

			Expect(target.TeamCallCount()).To(Equal(1), "target.Team should be called once")
		})
	})
})
