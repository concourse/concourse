package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func serialGroupsBehavior(database *dbSharedBehaviorInput) func() {
	return func() {
		Describe("Scheduling multiple builds within the same serial groups", func() {
			var jobOneConfig atc.JobConfig
			var jobOneTwoConfig atc.JobConfig
			var jobTwoConfig atc.JobConfig

			BeforeEach(func() {
				jobOneConfig = atc.JobConfig{
					Name:         "job-one",
					SerialGroups: []string{"one"},
				}
				jobOneTwoConfig = atc.JobConfig{
					Name:         "job-one-two",
					SerialGroups: []string{"one", "two"},
				}
				jobTwoConfig = atc.JobConfig{
					Name:         "job-two",
					SerialGroups: []string{"two"},
				}
			})

			Context("When a job is not the next most pending job within a serial group", func() {
				It("should not be scheduled", func() {
					buildOne, err := database.PipelineDB.CreateJobBuild(jobOneConfig.Name)
					Ω(err).ShouldNot(HaveOccurred())

					buildTwo, err := database.PipelineDB.CreateJobBuild(jobOneConfig.Name)
					Ω(err).ShouldNot(HaveOccurred())

					buildThree, err := database.PipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := database.PipelineDB.ScheduleBuild(buildOne.ID, jobOneConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())

					scheduled, err = database.PipelineDB.ScheduleBuild(buildTwo.ID, jobOneConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeFalse())
					scheduled, err = database.PipelineDB.ScheduleBuild(buildThree.ID, jobOneTwoConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeFalse())

					Ω(database.FinishBuild(buildOne.ID, db.StatusSucceeded)).Should(Succeed())

					scheduled, err = database.PipelineDB.ScheduleBuild(buildThree.ID, jobOneTwoConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeFalse())

					scheduled, err = database.PipelineDB.ScheduleBuild(buildTwo.ID, jobOneConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})
			})

			Context("when a build is running under job-one", func() {
				BeforeEach(func() {
					var err error
					build, err := database.PipelineDB.CreateJobBuild(jobOneConfig.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("and we schedule a build for job-one", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobOneConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})

				Context("and we schedule a build for job-one-two", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})

				Context("and we schedule a build for job-two", func() {
					It("succeeds", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobTwoConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})
				})
			})

			Context("When a build is running in job-one-two", func() {
				BeforeEach(func() {
					var err error
					build, err := database.PipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("and we schedule a build for job-one", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobOneConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})

				Context("and we schedule a build for job-one-two", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})

				Context("and we schedule a build for job-two", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobTwoConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Context("When a build is running in job two", func() {
				BeforeEach(func() {
					var err error
					build, err := database.PipelineDB.CreateJobBuild(jobTwoConfig.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("and we schedule a build for job-one", func() {
					It("succeeds", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobOneConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})
				})

				Context("and we schedule a build for job-one-two", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobOneTwoConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})

				Context("and we schedule a build for job-two", func() {
					It("fails", func() {
						build, err := database.PipelineDB.CreateJobBuild(jobTwoConfig.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := database.PipelineDB.ScheduleBuild(build.ID, jobTwoConfig)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})
		})
	}
}
