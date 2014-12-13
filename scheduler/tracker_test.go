package scheduler_test

import (
	"errors"
	"sync"

	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
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
		engine *fakes.FakeEngine

		tracker BuildTracker

		turbineServer *ghttp.Server

		lock   *dbfakes.FakeLock
		locker *fakes.FakeLocker
	)

	BeforeEach(func() {
		engine = new(fakes.FakeEngine)

		turbineServer = ghttp.NewServer()

		locker = new(fakes.FakeLocker)

		tracker = NewTracker(lagertest.NewTestLogger("test"), engine, locker)

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
				ID:       1,
				Guid:     "some-guid",
				Endpoint: turbineServer.URL(),
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

				engine.ResumeBuildStub = func(db.Build, lager.Logger) error {
					Ω(fakeLock.ReleaseCallCount()).Should(BeZero())
					return nil
				}
			})

			It("resumes the build, and releases the lock after", func() {
				Ω(locker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))
				lockedBuild := locker.AcquireWriteLockImmediatelyArgsForCall(0)
				Ω(lockedBuild).Should(Equal([]db.NamedLock{db.BuildTrackingLock(build.Guid)}))

				Ω(engine.ResumeBuildCallCount()).Should(Equal(1))
				resumedBuild, _ := engine.ResumeBuildArgsForCall(0)
				Ω(resumedBuild).Should(Equal(build))

				Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when the build is already being tracked", func() {
				var (
					resumed  chan struct{}
					tracking *sync.WaitGroup
				)

				BeforeEach(func() {
					resumed = make(chan struct{})
					tracking = new(sync.WaitGroup)

					engine.ResumeBuildStub = func(db.Build, lager.Logger) error {
						<-resumed
						return nil
					}

					tracking.Add(1)
					go func() {
						defer tracking.Done()
						tracker.TrackBuild(build)
					}()

					Eventually(engine.ResumeBuildCallCount).Should(Equal(1))
				})

				AfterEach(func() {
					close(resumed)
				})

				It("succeeds", func() {
					Ω(trackErr).ShouldNot(HaveOccurred())
				})

				It("does not resume twice", func() {
					Ω(engine.ResumeBuildCallCount()).Should(Equal(1))
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

			It("does not resume the build", func() {
				Ω(engine.ResumeBuildCallCount()).Should(BeZero())
			})
		})
	})
})
