package getjob_test

import (
	"errors"

	. "github.com/concourse/atc/web/getjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/getjob/fakes"
)

var _ = Describe("Paginator", func() {
	var paginator Paginator
	var fakeJobPaginatorDB *fakes.FakeJobPaginatorDB

	BeforeEach(func() {
		fakeJobPaginatorDB = new(fakes.FakeJobPaginatorDB)
		paginator = Paginator{
			PaginatorDB: fakeJobPaginatorDB,
		}
	})

	Context("PaginateJobBuilds", func() {
		jobName := "some-job"
		newerJobBuilds := false
		startingJobBuildID := 3

		Context("getting builds", func() {
			Context("when getting the builds returns an error", func() {
				BeforeEach(func() {
					fakeJobPaginatorDB.GetJobBuildsCursorReturns([]db.Build{}, false, errors.New("OH MY GOD"))
				})

				It("returns an error", func() {
					_, _, err := paginator.PaginateJobBuilds(jobName, startingJobBuildID, newerJobBuilds)
					Ω(err).Should(HaveOccurred())
				})
			})

			Context("when we do not return builds", func() {
				BeforeEach(func() {
					fakeJobPaginatorDB.GetJobBuildsCursorReturns([]db.Build{}, false, nil)
				})

				It("returns a version of pagination data that says hasPagination is false", func() {
					_, paginationData, _ := paginator.PaginateJobBuilds(jobName, startingJobBuildID, newerJobBuilds)
					Ω(paginationData.HasPagination()).Should(BeFalse())
				})

				It("calls to get the max id for job builds", func() {
					paginator.PaginateJobBuilds(jobName, startingJobBuildID, newerJobBuilds)

					Ω(fakeJobPaginatorDB.GetJobBuildsMaxIDCallCount()).Should(Equal(1))

					argJobName := fakeJobPaginatorDB.GetJobBuildsMaxIDArgsForCall(0)

					Ω(argJobName).Should(Equal(jobName))
				})

				It("calls to get 100 job builds in a direction starting with the passed in ID", func() {
					paginator.PaginateJobBuilds(jobName, startingJobBuildID, newerJobBuilds)

					Ω(fakeJobPaginatorDB.GetJobBuildsCursorCallCount()).Should(Equal(1))

					argJobName, argStartingJobBuildID, argResultsGreaterThanStartingID, argLimit := fakeJobPaginatorDB.GetJobBuildsCursorArgsForCall(0)

					Ω(argJobName).Should(Equal(jobName))
					Ω(argResultsGreaterThanStartingID).Should(Equal(newerJobBuilds))
					Ω(argStartingJobBuildID).Should(Equal(startingJobBuildID))
					Ω(argLimit).Should(Equal(100))
				})

				Context("when startingJobBuildID is 0 and resultsGreaterThanStartingID is false", func() {
					It("sets the starting id passed in to GetJobBuildsCursor to the maxID returned from GetJobBuildsMaxID", func() {
						fakeJobPaginatorDB.GetJobBuildsMaxIDReturns(298, nil)

						paginator.PaginateJobBuilds(jobName, 0, false)

						Ω(fakeJobPaginatorDB.GetJobBuildsCursorCallCount()).Should(Equal(1))

						_, argStartingJobBuildID, argResultsGreaterThanStartingID, _ := fakeJobPaginatorDB.GetJobBuildsCursorArgsForCall(0)

						Ω(argStartingJobBuildID).Should(Equal(298))
						Ω(argResultsGreaterThanStartingID).Should(BeFalse())
					})
				})
			})

			Context("when we return builds", func() {
				var builds []db.Build
				var moreResultsInGivenDirection bool

				BeforeEach(func() {
					builds = []db.Build{
						{
							ID: 10,
						},
						{
							ID: 9,
						},
					}
					moreResultsInGivenDirection = false

				})

				JustBeforeEach(func() {
					fakeJobPaginatorDB.GetJobBuildsCursorReturns(builds, moreResultsInGivenDirection, nil)
				})

				It("returns the builds we got back from the database call", func() {
					retBuilds, _, err := paginator.PaginateJobBuilds(jobName, startingJobBuildID, newerJobBuilds)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(retBuilds).Should(Equal(builds))
				})

			})
		})

	})
})
