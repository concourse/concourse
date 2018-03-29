package drainer_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/worker/beacon"
	"github.com/concourse/worker/beacon/beaconfakes"
	. "github.com/concourse/worker/drainer"
	"github.com/concourse/worker/drainer/drainerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Drainer", func() {
	var drainer *Drainer
	var logger *lagertest.TestLogger
	var fakeWatchProcess *drainerfakes.FakeWatchProcess
	var fakeClock *fakeclock.FakeClock
	var waitInterval time.Duration
	var fakeBeaconClient *beaconfakes.FakeBeaconClient

	BeforeEach(func() {
		waitInterval = 5 * time.Second
		logger = lagertest.NewTestLogger("drainer")
		fakeWatchProcess = new(drainerfakes.FakeWatchProcess)
		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))
		fakeBeaconClient = new(beaconfakes.FakeBeaconClient)
	})

	Context("when shutting down", func() {
		BeforeEach(func() {
			drainer = &Drainer{
				BeaconClient: fakeBeaconClient,
				IsShutdown:   true,
				WatchProcess: fakeWatchProcess,
				Clock:        fakeClock,
			}
		})

		Context("when beacon process is not running", func() {
			BeforeEach(func() {
				fakeWatchProcess.IsRunningReturns(false, nil)
			})

			It("returns right away", func() {
				err := drainer.Drain(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(0))
			})
		})

		Context("when failing to check if process is running", func() {
			var disaster = errors.New("disaster")

			BeforeEach(func() {
				fakeWatchProcess.IsRunningReturns(false, disaster)
			})

			It("returns an error", func() {
				err := drainer.Drain(logger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disaster))
			})
		})

		Context("if watched process is still running", func() {
			BeforeEach(func() {
				callCount := 0
				fakeWatchProcess.IsRunningStub = func(lager.Logger) (bool, error) {
					callCount++
					if callCount > 5 {
						return false, nil
					}

					fakeClock.Increment(waitInterval)
					return true, nil
				}
			})

			It("runs retire-worker until it exits with wait interval", func() {
				err := drainer.Drain(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(5))
				Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(0))
			})

			Context("when retiring worker fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeBeaconClient.RetireWorkerReturns(disaster)
				})

				It("does not return an error and keeps retrying", func() {
					err := drainer.Drain(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(5))
					Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(0))
				})
			})

			Context("when retiring worker fails to reach any tsa", func() {
				BeforeEach(func() {
					fakeBeaconClient.RetireWorkerReturns(beacon.ErrFailedToReachAnyTSA)
				})

				It("does not return an error and stops retrying", func() {
					err := drainer.Drain(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(1))
					Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(0))
				})
			})

			Context("when drain timeout is specified", func() {
				BeforeEach(func() {
					timeoutInterval := 3 * waitInterval
					drainer.Timeout = &timeoutInterval
				})

				It("exits after timeout and deletes the worker forcibly", func() {
					err := drainer.Drain(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(3))
					Expect(fakeBeaconClient.DeleteWorkerCallCount()).To(Equal(1))
					Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(0))
				})

				Context("when deleting worker fails", func() {
					var disaster = errors.New("disaster")

					BeforeEach(func() {
						fakeBeaconClient.DeleteWorkerReturns(disaster)
					})

					It("returns an error", func() {
						err := drainer.Drain(logger)
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(disaster))
					})
				})

				Context("when deleting worker fails to reach any tsa", func() {
					BeforeEach(func() {
						fakeBeaconClient.DeleteWorkerReturns(beacon.ErrFailedToReachAnyTSA)
					})

					It("does not return an error", func() {
						err := drainer.Drain(logger)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})

	Context("when not shutting down", func() {
		BeforeEach(func() {
			drainer = &Drainer{
				BeaconClient: fakeBeaconClient,
				IsShutdown:   false,
				WatchProcess: fakeWatchProcess,
				Clock:        fakeClock,
			}
		})

		Context("when beacon process is not running", func() {
			BeforeEach(func() {
				fakeWatchProcess.IsRunningReturns(false, nil)
			})

			It("returns right away", func() {
				err := drainer.Drain(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(0))
			})
		})

		Context("when failing to check if process is running", func() {
			var disaster = errors.New("disaster")

			BeforeEach(func() {
				fakeWatchProcess.IsRunningReturns(false, disaster)
			})

			It("returns an error", func() {
				err := drainer.Drain(logger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disaster))
			})
		})

		Context("if watched process is still running", func() {
			BeforeEach(func() {
				callCount := 0
				fakeWatchProcess.IsRunningStub = func(lager.Logger) (bool, error) {
					callCount++
					if callCount > 5 {
						return false, nil
					}

					fakeClock.Increment(waitInterval)
					return true, nil
				}
			})

			It("runs land-worker until it exits with wait interval", func() {
				err := drainer.Drain(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(5))
				Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(0))
			})

			Context("when landing worker fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeBeaconClient.LandWorkerReturns(disaster)
				})

				It("does not return an error and keeps retrying", func() {
					err := drainer.Drain(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(5))
					Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(0))
				})
			})

			Context("when landing worker fails to reach any tsa", func() {
				BeforeEach(func() {
					fakeBeaconClient.LandWorkerReturns(beacon.ErrFailedToReachAnyTSA)
				})

				It("does not return an error and stops retrying", func() {
					err := drainer.Drain(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(1))
					Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(0))
				})
			})

			Context("when drain timeout is specified", func() {
				BeforeEach(func() {
					timeoutInterval := 3 * waitInterval
					drainer.Timeout = &timeoutInterval
				})

				It("exits after timeout", func() {
					err := drainer.Drain(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBeaconClient.LandWorkerCallCount()).To(Equal(3))
					Expect(fakeBeaconClient.DeleteWorkerCallCount()).To(Equal(0))
					Expect(fakeBeaconClient.RetireWorkerCallCount()).To(Equal(0))
				})
			})
		})
	})
})
