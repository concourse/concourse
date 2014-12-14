package scheduler_test

import (
	"errors"

	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	efakes "github.com/concourse/atc/engine/fakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Tracker", func() {
	var (
		engine    *efakes.FakeEngine
		trackerDB *fakes.FakeTrackerDB

		tracker BuildTracker

		turbineServer *ghttp.Server

		lock   *dbfakes.FakeLock
		locker *fakes.FakeLocker
	)

	BeforeEach(func() {
		engine = new(efakes.FakeEngine)
		trackerDB = new(fakes.FakeTrackerDB)

		turbineServer = ghttp.NewServer()

		locker = new(fakes.FakeLocker)

		tracker = NewTracker(lagertest.NewTestLogger("test"), engine, trackerDB, locker)

		lock = new(dbfakes.FakeLock)
		locker.AcquireWriteLockImmediatelyReturns(lock, nil)
	})

	AfterEach(func() {
		turbineServer.CloseClientConnections()
		turbineServer.Close()
	})

	Describe("TrackBuild", func() {
		var (
			build db.Build

			trackErr error
		)

		BeforeEach(func() {
			build = db.Build{
				ID:             1,
				Engine:         "some-engine",
				EngineMetadata: "some-engine-metadata",
			}
		})

		JustBeforeEach(func() {
			trackErr = tracker.TrackBuild(build)
		})

		Context("when acquiring the lock succeeds", func() {
			var fakeLock *dbfakes.FakeLock

			BeforeEach(func() {
				fakeLock = new(dbfakes.FakeLock)
				locker.AcquireWriteLockImmediatelyReturns(fakeLock, nil)
			})

			Context("and the build is found", func() {
				var fakeBuild *efakes.FakeBuild

				BeforeEach(func() {
					fakeBuild = new(efakes.FakeBuild)
					fakeBuild.ResumeStub = func(lager.Logger) error {
						Ω(fakeLock.ReleaseCallCount()).Should(BeZero())
						return nil
					}

					engine.LookupBuildReturns(fakeBuild, nil)
				})

				It("resumes the build, and releases the lock after", func() {
					Ω(locker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))
					lockedBuild := locker.AcquireWriteLockImmediatelyArgsForCall(0)
					Ω(lockedBuild).Should(Equal([]db.NamedLock{db.BuildTrackingLock(build.ID)}))

					Ω(engine.LookupBuildCallCount()).Should(Equal(1))
					Ω(engine.LookupBuildArgsForCall(0)).Should(Equal(build))

					Ω(fakeBuild.ResumeCallCount()).Should(Equal(1))

					Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("and looking up the build fails", func() {
				BeforeEach(func() {
					engine.LookupBuildReturns(nil, errors.New("nope"))
				})

				It("marks the build as failed", func() {
					Ω(trackerDB.SaveBuildStatusCallCount()).Should(Equal(1))
					id, status := trackerDB.SaveBuildStatusArgsForCall(0)
					Ω(id).Should(Equal(build.ID))
					Ω(status).Should(Equal(db.StatusErrored))
				})
			})
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				locker.AcquireWriteLockImmediatelyReturns(nil, errors.New("no lock for you"))
			})

			It("succeeds", func() {
				Ω(trackErr).ShouldNot(HaveOccurred())
			})

			It("does not look up the build", func() {
				Ω(engine.LookupBuildCallCount()).Should(BeZero())
			})
		})
	})
})
