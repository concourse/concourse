package worker_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"

	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim/baggageclaimfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeClient", func() {
	var (
		fakeLock   *lockfakes.FakeLock
		testLogger *lagertest.TestLogger

		fakeBaggageclaimClient            *baggageclaimfakes.FakeClient
		fakeLockFactory                   *lockfakes.FakeLockFactory
		fakeDBVolumeFactory               *dbfakes.FakeVolumeFactory
		fakeWorkerBaseResourceTypeFactory *dbfakes.FakeWorkerBaseResourceTypeFactory
		fakeWorkerTaskCacheFactory        *dbfakes.FakeWorkerTaskCacheFactory
		fakeClock                         *fakeclock.FakeClock
		dbWorker                          *dbfakes.FakeWorker

		volumeClient worker.VolumeClient
	)

	BeforeEach(func() {
		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		dbWorker = new(dbfakes.FakeWorker)
		dbWorker.NameReturns("some-worker")

		testLogger = lagertest.NewTestLogger("test")

		fakeDBVolumeFactory = new(dbfakes.FakeVolumeFactory)
		fakeWorkerBaseResourceTypeFactory = new(dbfakes.FakeWorkerBaseResourceTypeFactory)
		fakeWorkerTaskCacheFactory = new(dbfakes.FakeWorkerTaskCacheFactory)
		fakeLock = new(lockfakes.FakeLock)

		volumeClient = worker.NewVolumeClient(
			fakeBaggageclaimClient,
			dbWorker,
			fakeClock,

			fakeLockFactory,
			fakeDBVolumeFactory,
			fakeWorkerBaseResourceTypeFactory,
			fakeWorkerTaskCacheFactory,
		)
	})

	Describe("FindOrCreateVolumeForContainer", func() {
		var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error
		var container db.CreatingContainer
		var fakeCreatingVolume *dbfakes.FakeCreatingVolume
		var volumeStrategy baggageclaim.Strategy

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeCreatingVolume = new(dbfakes.FakeCreatingVolume)
			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)
			fakeDBVolumeFactory.CreateContainerVolumeReturns(fakeCreatingVolume, nil)

			volumeStrategy = baggageclaim.ImportStrategy{
				Path: "/some/path",
			}
		})

		JustBeforeEach(func() {
			container = new(dbfakes.FakeCreatingContainer)
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.FindOrCreateVolumeForContainer(
				testLogger,
				worker.VolumeSpec{
					Strategy: volumeStrategy,
				},
				container,
				42,
				"some-mount-path",
			)
		})

		Context("when volume exists in creating state", func() {
			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			Context("when acquiring volume creating lock fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeLockFactory.AcquireStub = func(logger lager.Logger, lockID lock.LockID) (lock.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find volume again", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(3))
					Expect(fakeDBVolumeFactory.FindContainerVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(fakeLock, true, nil)
				})

				Context("when checking for the volume in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeStub = func(lager.Logger, string) (baggageclaim.Volume, bool, error) {
							Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
							return nil, false, nil
						}
					})

					It("does so with the lock held", func() {
						Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))
					})
				})

				Context("when volume exists in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
					})

					It("returns the volume", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
					})
				})

				Context("when volume does not exist in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})

					It("creates volume in baggageclaim", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
						Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
					})

					Context("when creating the volume in baggageclaim fails", func() {
						BeforeEach(func() {
							fakeBaggageclaimClient.CreateVolumeReturns(nil, errors.New("failed to create volume, oh no"))
						})

						It("marks the creating volume as failed", func() {
							Expect(fakeCreatingVolume.FailedCallCount()).To(Equal(1))
						})
					})
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})
		})

		Context("when volume exists in created state", func() {
			BeforeEach(func() {
				fakeCreatedVolume := new(dbfakes.FakeCreatedVolume)
				fakeCreatedVolume.HandleReturns("fake-handle")
				fakeDBVolumeFactory.FindContainerVolumeReturns(nil, fakeCreatedVolume, nil)
			})

			Context("when volume exists in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
				})

				It("returns the volume", func() {
					Expect(foundOrCreatedErr).NotTo(HaveOccurred())
					Expect(foundOrCreatedVolume).NotTo(BeNil())
				})
			})

			Context("when volume does not exist in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(foundOrCreatedErr).To(HaveOccurred())
					Expect(foundOrCreatedErr.Error()).To(ContainSubstring("failed to find created volume in baggageclaim. Volume handle: fake-handle"))
				})
			})
		})

		Context("when volume does not exist in db", func() {
			var fakeCreatedVolume *dbfakes.FakeCreatedVolume

			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(nil, nil, nil)
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
				fakeDBVolumeFactory.CreateContainerVolumeReturns(fakeCreatingVolume, nil)
				fakeCreatedVolume = new(dbfakes.FakeCreatedVolume)
				fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
			})

			It("creates volume in creating state", func() {
				Expect(fakeDBVolumeFactory.CreateContainerVolumeCallCount()).To(Equal(1))
				actualTeamID, actualWorkerName, actualContainer, actualMountPath := fakeDBVolumeFactory.CreateContainerVolumeArgsForCall(0)
				Expect(actualTeamID).To(Equal(42))
				Expect(actualWorkerName).To(Equal(dbWorker.Name()))
				Expect(actualContainer).To(Equal(container))
				Expect(actualMountPath).To(Equal("some-mount-path"))
			})

			It("creates volume in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(worker.NewVolume(fakeBaggageclaimVolume, fakeCreatedVolume, volumeClient)))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})

			Context("when creating the volume in baggageclaim fails", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.CreateVolumeReturns(nil, errors.New("failed to create volume, oh no"))
				})

				It("marks the creating volume for removal", func() {
					Expect(fakeCreatingVolume.FailedCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("FindOrCreateCOWVolumeForContainer", func() {
		var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error
		var container db.CreatingContainer
		var fakeCreatingVolume *dbfakes.FakeCreatingVolume
		var fakeCreatedVolume *dbfakes.FakeCreatedVolume
		var volumeStrategy baggageclaim.Strategy
		var parentVolume *workerfakes.FakeVolume
		var fakeParentBCVolume *baggageclaimfakes.FakeVolume

		BeforeEach(func() {
			parentVolume = new(workerfakes.FakeVolume)
			parentVolume.HandleReturns("fake-parent-handle")

			fakeParentBCVolume = new(baggageclaimfakes.FakeVolume)

			volumeStrategy = baggageclaim.COWStrategy{
				Parent: fakeParentBCVolume,
			}

			fakeCreatingVolume = new(dbfakes.FakeCreatingVolume)
			parentVolume.CreateChildForContainerReturns(fakeCreatingVolume, nil)

			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)

			fakeCreatedVolume = new(dbfakes.FakeCreatedVolume)
			fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)
		})

		JustBeforeEach(func() {
			container = new(dbfakes.FakeCreatingContainer)
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.FindOrCreateCOWVolumeForContainer(
				testLogger,
				worker.VolumeSpec{
					Strategy: volumeStrategy,
				},
				container,
				parentVolume,
				42,
				"some-mount-path",
			)
		})

		Context("when volume exists in creating state", func() {
			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			Context("when acquiring volume creating lock fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeLockFactory.AcquireStub = func(logger lager.Logger, lockID lock.LockID) (lock.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find volume again", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(3))
					Expect(fakeDBVolumeFactory.FindContainerVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(fakeLock, true, nil)
				})

				Context("when volume exists in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
					})

					It("returns the volume", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
					})
				})

				Context("when volume does not exist in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})

					It("creates volume in baggageclaim", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
						Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
					})
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})
		})

		Context("when volume exists in created state", func() {
			BeforeEach(func() {
				fakeCreatedVolume := new(dbfakes.FakeCreatedVolume)
				fakeCreatedVolume.HandleReturns("fake-handle")
				fakeDBVolumeFactory.FindContainerVolumeReturns(nil, fakeCreatedVolume, nil)
			})

			Context("when volume exists in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
				})

				It("returns the volume", func() {
					Expect(foundOrCreatedErr).NotTo(HaveOccurred())
					Expect(foundOrCreatedVolume).NotTo(BeNil())
				})
			})

			Context("when volume does not exist in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(foundOrCreatedErr).To(HaveOccurred())
					Expect(foundOrCreatedErr.Error()).To(ContainSubstring("failed to find created volume in baggageclaim. Volume handle: fake-handle"))
				})
			})
		})

		Context("when volume does not exist in db", func() {
			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(nil, nil, nil)
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
			})

			It("creates volume in creating state with parent volume", func() {
				Expect(parentVolume.CreateChildForContainerCallCount()).To(Equal(1))
				actualContainer, actualMountPath := parentVolume.CreateChildForContainerArgsForCall(0)
				Expect(actualContainer).To(Equal(container))
				Expect(actualMountPath).To(Equal("some-mount-path"))
			})

			It("creates volume in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(worker.NewVolume(fakeBaggageclaimVolume, fakeCreatedVolume, volumeClient)))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("FindVolumeForTaskCache", func() {
		Context("when worker task cache does not exist", func() {
			BeforeEach(func() {
				fakeWorkerTaskCacheFactory.FindReturns(nil, false, nil)
			})

			It("returns false", func() {
				_, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when worker task cache exists", func() {
			var taskCache *db.UsedWorkerTaskCache
			BeforeEach(func() {
				taskCache = &db.UsedWorkerTaskCache{
					ID: 123,
				}

				fakeWorkerTaskCacheFactory.FindReturns(taskCache, true, nil)
			})

			Context("when task cache volume does not exist in db", func() {
				BeforeEach(func() {
					fakeDBVolumeFactory.FindTaskCacheVolumeReturns(nil, nil, nil)
				})

				It("returns false", func() {
					_, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			Context("when task cache volume exists in db", func() {
				var dbVolume db.CreatedVolume

				BeforeEach(func() {
					dbVolume = new(dbfakes.FakeCreatedVolume)
					fakeDBVolumeFactory.FindTaskCacheVolumeReturns(nil, dbVolume, nil)
				})

				Context("when task cache volume does not exist in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})

					It("returns false", func() {
						_, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeFalse())
					})
				})

				Context("when task cache volume exists in baggageclaim", func() {
					var bcVolume *baggageclaimfakes.FakeVolume

					BeforeEach(func() {
						bcVolume = new(baggageclaimfakes.FakeVolume)
						fakeBaggageclaimClient.LookupVolumeReturns(bcVolume, true, nil)
					})

					It("returns volume", func() {
						volume, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						Expect(volume).To(Equal(worker.NewVolume(bcVolume, dbVolume, volumeClient)))
					})
				})
			})
		})
	})

	Describe("LookupVolume", func() {
		var handle string

		var found bool
		var lookupErr error

		BeforeEach(func() {
			handle = "some-handle"

			fakeCreatedVolume := new(dbfakes.FakeCreatedVolume)
			fakeDBVolumeFactory.FindCreatedVolumeReturns(fakeCreatedVolume, true, nil)
		})

		JustBeforeEach(func() {
			_, found, lookupErr = worker.NewVolumeClient(
				fakeBaggageclaimClient,
				dbWorker,
				fakeClock,

				fakeLockFactory,
				fakeDBVolumeFactory,
				fakeWorkerBaseResourceTypeFactory,
				fakeWorkerTaskCacheFactory,
			).LookupVolume(testLogger, handle)
		})

		Context("when the volume can be found on baggageclaim", func() {
			var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume

			BeforeEach(func() {
				fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
				fakeBaggageclaimVolume.HandleReturns(handle)
				fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
			})

			It("looks up the volume via BaggageClaim", func() {
				Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))

				_, lookedUpHandle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
				Expect(lookedUpHandle).To(Equal(handle))
			})
		})

		Context("when the volume cannot be found on baggageclaim", func() {
			BeforeEach(func() {
				fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when the volume cannot be found in database", func() {
			BeforeEach(func() {
				fakeDBVolumeFactory.FindCreatedVolumeReturns(nil, false, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when looking up the volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})
	})
})
