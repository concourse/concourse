package maxinflight_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/scheduler/buildstarter/maxinflight"
	"github.com/concourse/atc/scheduler/buildstarter/maxinflight/maxinflightfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Updater", func() {
	var (
		fakeDB   *maxinflightfakes.FakeUpdaterDB
		updater  maxinflight.Updater
		disaster error
	)

	BeforeEach(func() {
		fakeDB = new(maxinflightfakes.FakeUpdaterDB)
		updater = maxinflight.NewUpdater(fakeDB)
		disaster = errors.New("bad thing")
	})

	Describe("UpdateMaxInFlightReached", func() {
		var rawMaxInFlight int
		var serialGroups []string
		var updateErr error
		var reached bool

		JustBeforeEach(func() {
			reached, updateErr = updater.UpdateMaxInFlightReached(
				lagertest.NewTestLogger("test"),
				atc.JobConfig{
					Name:           "some-job",
					SerialGroups:   serialGroups,
					RawMaxInFlight: rawMaxInFlight,
				},
				57,
			)
		})

		itReturnsFalseAndNoError := func() {
			It("returns false and no error", func() {
				Expect(updateErr).NotTo(HaveOccurred())
				Expect(reached).To(BeFalse())
				Expect(fakeDB.SetMaxInFlightReachedCallCount()).To(Equal(1))
				jobName, actualReached := fakeDB.SetMaxInFlightReachedArgsForCall(0)
				Expect(jobName).To(Equal("some-job"))
				Expect(actualReached).To(BeFalse())
			})
		}

		itReturnsTrueAndNoError := func() {
			It("returns true and no error", func() {
				Expect(updateErr).NotTo(HaveOccurred())
				Expect(reached).To(BeTrue())
				Expect(fakeDB.SetMaxInFlightReachedCallCount()).To(Equal(1))
				jobName, actualReached := fakeDB.SetMaxInFlightReachedArgsForCall(0)
				Expect(jobName).To(Equal("some-job"))
				Expect(actualReached).To(BeTrue())
			})
		}

		itReturnsTheError := func() {
			It("returns the error", func() {
				Expect(updateErr).To(Equal(disaster))
				Expect(fakeDB.SetMaxInFlightReachedCallCount()).To(Equal(0))
			})
		}

		Context("when the job config doesn't specify max in flight", func() {
			BeforeEach(func() {
				rawMaxInFlight = 0
				serialGroups = []string{}
			})

			itReturnsFalseAndNoError()

			It("doesn't look at the database", func() {
				Expect(fakeDB.GetRunningBuildsBySerialGroupCallCount()).To(BeZero())
				Expect(fakeDB.GetNextPendingBuildBySerialGroupCallCount()).To(BeZero())
			})

			Context("when setting max in flight reached fails", func() {
				BeforeEach(func() {
					fakeDB.SetMaxInFlightReachedReturns(disaster)
				})

				It("returns the error", func() {
					Expect(updateErr).To(Equal(disaster))
				})
			})
		})

		itReturnsFalseIfOurBuildIsNext := func() {
			Context("when the build we are trying to run is no longer pending", func() {
				BeforeEach(func() {
					fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, nil)
				})

				itReturnsTrueAndNoError()
			})

			Context("when there is another build ahead of us in line", func() {
				var fakeBuild *dbfakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(dbfakes.FakeBuild)
					fakeBuild.IDReturns(101)
					fakeDB.GetNextPendingBuildBySerialGroupReturns(fakeBuild, true, nil)
				})

				itReturnsTrueAndNoError()
			})

			Context("when the build we are trying to run is first in line", func() {
				var fakeBuild *dbfakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(dbfakes.FakeBuild)
					fakeBuild.IDReturns(57)
					fakeDB.GetNextPendingBuildBySerialGroupReturns(fakeBuild, true, nil)
				})

				itReturnsFalseAndNoError()
			})
		}

		Context("when the job config specifies max in flight = 3", func() {
			BeforeEach(func() {
				rawMaxInFlight = 3
				serialGroups = []string{}
			})

			Context("when looking up the running builds fails", func() {
				BeforeEach(func() {
					fakeDB.GetRunningBuildsBySerialGroupReturns(nil, disaster)
				})

				itReturnsTheError()

				It("looked up the running builds with the right job name and serial group", func() {
					Expect(fakeDB.GetRunningBuildsBySerialGroupCallCount()).To(Equal(1))
					actualJobName, actualSerialGroups := fakeDB.GetRunningBuildsBySerialGroupArgsForCall(0)
					Expect(actualJobName).To(Equal("some-job"))
					Expect(actualSerialGroups).To(ConsistOf("some-job"))
				})
			})

			Context("when there are 3 builds of the job running", func() {
				BeforeEach(func() {
					fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{nil, nil, nil}, nil)
				})

				itReturnsTrueAndNoError()

				It("doesn't look up the next pending build", func() {
					Expect(fakeDB.GetNextPendingBuildBySerialGroupCallCount()).To(BeZero())
				})
			})

			Context("when there are 2 builds of the job running", func() {
				BeforeEach(func() {
					fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{nil, nil}, nil)
				})

				Context("when looking up the next pending build returns an error", func() {
					BeforeEach(func() {
						fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, disaster)
					})

					itReturnsTheError()

					It("looked up the next pending build with the right job name and serial group", func() {
						Expect(fakeDB.GetNextPendingBuildBySerialGroupCallCount()).To(Equal(1))
						actualJobName, actualSerialGroups := fakeDB.GetNextPendingBuildBySerialGroupArgsForCall(0)
						Expect(actualJobName).To(Equal("some-job"))
						Expect(actualSerialGroups).To(ConsistOf("some-job"))
					})
				})

				itReturnsFalseIfOurBuildIsNext()
			})
		})

		Context("when the job is in serial groups", func() {
			BeforeEach(func() {
				rawMaxInFlight = 0
				serialGroups = []string{"serial-group-1", "serial-group-2"}
			})

			Context("when looking up the running builds fails", func() {
				BeforeEach(func() {
					fakeDB.GetRunningBuildsBySerialGroupReturns(nil, disaster)
				})

				itReturnsTheError()

				It("looked up the running builds with the right job name and serial group", func() {
					Expect(fakeDB.GetRunningBuildsBySerialGroupCallCount()).To(Equal(1))
					actualJobName, actualSerialGroups := fakeDB.GetRunningBuildsBySerialGroupArgsForCall(0)
					Expect(actualJobName).To(Equal("some-job"))
					Expect(actualSerialGroups).To(ConsistOf("serial-group-1", "serial-group-2"))
				})
			})

			Context("when a job in the serial group is running", func() {
				BeforeEach(func() {
					fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{nil}, nil)
				})

				itReturnsTrueAndNoError()

				It("doesn't look up the next pending build", func() {
					Expect(fakeDB.GetNextPendingBuildBySerialGroupCallCount()).To(BeZero())
				})
			})

			Context("when no job in the serial group is running", func() {
				BeforeEach(func() {
					fakeDB.GetRunningBuildsBySerialGroupReturns([]db.Build{}, nil)
				})

				Context("when looking up the next pending build returns an error", func() {
					BeforeEach(func() {
						fakeDB.GetNextPendingBuildBySerialGroupReturns(nil, false, disaster)
					})

					itReturnsTheError()

					It("looked up the next pending build with the right job name and serial group", func() {
						Expect(fakeDB.GetNextPendingBuildBySerialGroupCallCount()).To(Equal(1))
						actualJobName, actualSerialGroups := fakeDB.GetNextPendingBuildBySerialGroupArgsForCall(0)
						Expect(actualJobName).To(Equal("some-job"))
						Expect(actualSerialGroups).To(ConsistOf("serial-group-1", "serial-group-2"))
					})
				})

				itReturnsFalseIfOurBuildIsNext()
			})
		})
	})
})
