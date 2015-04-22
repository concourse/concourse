package getjob_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/web/getjob/fakes"
	"github.com/concourse/atc/web/group"

	. "github.com/concourse/atc/web/getjob"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeDB *fakes.FakeJobDB
	var fakeConfigDB *dbfakes.FakeConfigDB

	BeforeEach(func() {
		fakeDB = new(fakes.FakeJobDB)
		fakeConfigDB = new(dbfakes.FakeConfigDB)
	})

	Context("when the config database returns an error", func() {
		BeforeEach(func() {
			fakeConfigDB.GetConfigReturns(atc.Config{}, db.ConfigVersion(1), errors.New("disaster"))
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData(fakeDB, fakeConfigDB, "job-name")
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

			fakeConfigDB.GetConfigReturns(config, db.ConfigVersion(1), nil)
		})

		It("returns not found if the job cannot be found in the config", func() {
			_, err := FetchTemplateData(fakeDB, fakeConfigDB, "not-a-job-name")
			Ω(err).Should(HaveOccurred())
			Ω(err).Should(MatchError(ErrJobConfigNotFound))
		})

		Context("when the job can be found in the config", func() {
			Context("when the job builds lookup returns an error", func() {

				It("returns an error if the jobs's builds could not be retreived", func() {
					fakeDB.GetAllJobBuildsReturns([]db.Build{}, errors.New("disaster"))
					_, err := FetchTemplateData(fakeDB, fakeConfigDB, "job-name")
					Ω(err).Should(HaveOccurred())
				})
			})

			Context("when the job builds lookup returns a build", func() {
				var builds []db.Build

				BeforeEach(func() {
					builds = []db.Build{
						{
							ID:      1,
							Name:    "1",
							JobName: "job-name",
							Status:  db.StatusSucceeded,
						},
					}

					fakeDB.GetAllJobBuildsReturns(builds, nil)
				})

				Context("when the get job lookup returns an error", func() {
					It("returns an ", func() {
						fakeDB.GetJobReturns(db.Job{}, errors.New("disaster"))
						_, err := FetchTemplateData(fakeDB, fakeConfigDB, "job-name")
						Ω(err).Should(HaveOccurred())
					})

					Context("when the get job lookup returns a job", func() {
						var groupStates []group.State
						var dbJob db.Job

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

							dbJob = db.Job{
								Name:   "some-job",
								Paused: false,
							}

							fakeDB.GetJobReturns(dbJob, nil)
						})

						Context("when the current build lookup returns an error", func() {

							It("has the correct template data and sets the current build status to pending", func() {
								fakeDB.GetCurrentBuildReturns(db.Build{}, errors.New("No current build"))

								templateData, err := FetchTemplateData(fakeDB, fakeConfigDB, "job-name")
								Ω(err).ShouldNot(HaveOccurred())

								Ω(templateData.GroupStates).Should(ConsistOf(groupStates))
								Ω(templateData.Job).Should(Equal(job))
								Ω(templateData.DBJob).Should(Equal(dbJob))
								Ω(templateData.Builds).Should(Equal(builds))
								Ω(templateData.CurrentBuild).Should(Equal(db.Build{
									Status: db.StatusPending,
								}))
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
								templateData, err := FetchTemplateData(fakeDB, fakeConfigDB, "job-name")
								Ω(err).ShouldNot(HaveOccurred())

								Ω(templateData.GroupStates).Should(ConsistOf(groupStates))
								Ω(templateData.Job).Should(Equal(job))
								Ω(templateData.DBJob).Should(Equal(dbJob))
								Ω(templateData.Builds).Should(Equal(builds))
								Ω(templateData.CurrentBuild).Should(Equal(currentBuild))
							})

							Context("when the job is paused", func() {
								BeforeEach(func() {
									dbJob = db.Job{
										Name:   "some-job",
										Paused: true,
									}
									fakeDB.GetJobReturns(dbJob, nil)
								})

								It("has the correct template data and sets the current build status to paused", func() {
									templateData, err := FetchTemplateData(fakeDB, fakeConfigDB, "job-name")
									Ω(err).ShouldNot(HaveOccurred())

									Ω(templateData.GroupStates).Should(ConsistOf(groupStates))
									Ω(templateData.Job).Should(Equal(job))
									Ω(templateData.DBJob).Should(Equal(dbJob))
									Ω(templateData.Builds).Should(Equal(builds))
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
