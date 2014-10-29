package radar_test

import (
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"

	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Radar", func() {
	var checker *fakes.FakeResourceChecker
	var tracker *fakes.FakeVersionDB
	var interval time.Duration

	var radar *Radar

	var resource config.Resource

	var locker *fakes.FakeLocker
	var readLock *dbfakes.FakeLock
	var writeLock *dbfakes.FakeLock

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("radar")
		checker = new(fakes.FakeResourceChecker)
		tracker = new(fakes.FakeVersionDB)
		interval = 100 * time.Millisecond
		locker = new(fakes.FakeLocker)

		radar = NewRadar(logger, tracker, interval, locker)

		resource = config.Resource{
			Name:   "some-resource",
			Type:   "git",
			Source: config.Source{"uri": "http://example.com"},
		}

		readLock = new(dbfakes.FakeLock)
		locker.AcquireReadLockReturns(readLock, nil)
		writeLock = new(dbfakes.FakeLock)
		locker.AcquireWriteLockReturns(writeLock, nil)
	})

	JustBeforeEach(func() {
		radar.Scan(checker, resource)
	})

	AfterEach(func() {
		radar.Stop()
	})

	Describe("checking", func() {
		var times chan time.Time

		BeforeEach(func() {
			times = make(chan time.Time, 100)

			checker.CheckResourceStub = func(config.Resource, db.Version) ([]db.Version, error) {
				times <- time.Now()
				return nil, nil
			}
		})

		It("checks on a specified interval", func() {
			var time1 time.Time
			var time2 time.Time

			Eventually(times).Should(Receive(&time1))
			Eventually(times).Should(Receive(&time2))

			Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/4))
		})

		It("grabs a lock before checking, releases after", func() {
			Eventually(times).Should(Receive())

			Ω(locker.AcquireReadLockCallCount()).Should(Equal(1))

			lockedInputs := locker.AcquireReadLockArgsForCall(0)
			Ω(lockedInputs).Should(Equal([]string{"resource: some-resource"}))

			Ω(readLock.ReleaseCallCount()).Should(Equal(1))

			Ω(locker.AcquireWriteLockCallCount()).Should(Equal(0))
		})

		Context("when there is no current version", func() {
			It("checks from nil", func() {
				Eventually(times).Should(Receive())

				resource, version := checker.CheckResourceArgsForCall(0)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(BeNil())
			})
		})

		Context("when there is a current version", func() {
			BeforeEach(func() {
				tracker.GetLatestVersionedResourceReturns(db.VersionedResource{
					Version: db.Version{"version": "1"},
				}, nil)
			})

			It("checks from it", func() {
				Eventually(times).Should(Receive())

				resource, version := checker.CheckResourceArgsForCall(0)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(Equal(db.Version{"version": "1"}))

				tracker.GetLatestVersionedResourceReturns(db.VersionedResource{
					Version: db.Version{"version": "2"},
				}, nil)

				Eventually(times).Should(Receive())

				resource, version = checker.CheckResourceArgsForCall(1)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(Equal(db.Version{"version": "2"}))
			})
		})

		Context("when the check returns versions", func() {
			var checkedFrom chan db.Version

			var nextVersions []db.Version

			BeforeEach(func() {
				checkedFrom = make(chan db.Version, 100)

				nextVersions = []db.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				}

				checkResults := map[int][]db.Version{
					0: nextVersions,
				}

				check := 0
				checker.CheckResourceStub = func(checkedResource config.Resource, from db.Version) ([]db.Version, error) {
					defer GinkgoRecover()

					Ω(checkedResource).Should(Equal(resource))

					checkedFrom <- from
					result := checkResults[check]
					check++

					return result, nil
				}
			})

			It("saves them all, in order", func() {
				Eventually(tracker.SaveVersionedResourceCallCount).Should(Equal(3))

				Ω(tracker.SaveVersionedResourceArgsForCall(0)).Should(Equal(db.VersionedResource{
					Name:    "some-resource",
					Type:    "git",
					Source:  db.Source{"uri": "http://example.com"},
					Version: db.Version{"version": "1"},
				}))

				Ω(tracker.SaveVersionedResourceArgsForCall(1)).Should(Equal(db.VersionedResource{
					Name:    "some-resource",
					Type:    "git",
					Source:  db.Source{"uri": "http://example.com"},
					Version: db.Version{"version": "2"},
				}))

				Ω(tracker.SaveVersionedResourceArgsForCall(2)).Should(Equal(db.VersionedResource{
					Name:    "some-resource",
					Type:    "git",
					Source:  db.Source{"uri": "http://example.com"},
					Version: db.Version{"version": "3"},
				}))
			})

			It("grabs a write lock around the save", func() {
				Eventually(tracker.SaveVersionedResourceCallCount).Should(Equal(3))

				Ω(locker.AcquireWriteLockCallCount()).Should(Equal(1))

				lockedInputs := locker.AcquireWriteLockArgsForCall(0)
				Ω(lockedInputs).Should(Equal([]string{"resource: some-resource"}))

				Ω(writeLock.ReleaseCallCount()).Should(Equal(1))
			})
		})

		Context("and checking takes a while", func() {
			BeforeEach(func() {
				checked := false

				checker.CheckResourceStub = func(config.Resource, db.Version) ([]db.Version, error) {
					times <- time.Now()

					if checked {
						time.Sleep(interval)
					}

					checked = true

					return nil, nil
				}
			})

			It("does not count it towards the interval", func() {
				var time1 time.Time
				var time2 time.Time

				Eventually(times).Should(Receive(&time1))
				Eventually(times, 2).Should(Receive(&time2))

				Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/2))
			})
		})
	})
})
