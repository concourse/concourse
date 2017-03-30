package worker_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/worker"

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
		fakeLockDB                        *workerfakes.FakeLockDB
		fakeDBVolumeFactory               *dbngfakes.FakeVolumeFactory
		fakeWorkerBaseResourceTypeFactory *dbngfakes.FakeWorkerBaseResourceTypeFactory
		fakeClock                         *fakeclock.FakeClock
		dbWorker                          *dbngfakes.FakeWorker

		volumeClient worker.VolumeClient
	)

	BeforeEach(func() {
		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeLockDB = new(workerfakes.FakeLockDB)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		dbWorker = new(dbngfakes.FakeWorker)
		dbWorker.NameReturns("some-worker")

		testLogger = lagertest.NewTestLogger("test")

		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)
		fakeWorkerBaseResourceTypeFactory = new(dbngfakes.FakeWorkerBaseResourceTypeFactory)
		fakeLock = new(lockfakes.FakeLock)

		volumeClient = worker.NewVolumeClient(
			fakeBaggageclaimClient,
			fakeLockDB,
			fakeDBVolumeFactory,
			fakeWorkerBaseResourceTypeFactory,
			fakeClock,
			dbWorker,
		)
	})

	Describe("FindOrCreateVolumeForContainer", func() {
		var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error
		var container dbng.CreatingContainer
		var fakeCreatingVolume *dbngfakes.FakeCreatingVolume
		var volumeStrategy worker.Strategy

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeCreatingVolume = new(dbngfakes.FakeCreatingVolume)
			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)
			fakeDBVolumeFactory.CreateContainerVolumeReturns(fakeCreatingVolume, nil)

			version := "some-version"
			volumeStrategy = worker.HostRootFSStrategy{
				Path:       "/some/path",
				WorkerName: "worker-name",
				Version:    &version,
			}
		})

		JustBeforeEach(func() {
			container = new(dbngfakes.FakeCreatingContainer)
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
					fakeLockDB.AcquireVolumeCreatingLockReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeLockDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeLockDB.AcquireVolumeCreatingLockStub = func(logger lager.Logger, volumeID int) (lock.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find volume again", func() {
					Expect(fakeLockDB.AcquireVolumeCreatingLockCallCount()).To(Equal(3))
					Expect(fakeDBVolumeFactory.FindContainerVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeLockDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
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
				fakeCreatedVolume := new(dbngfakes.FakeCreatedVolume)
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
					Expect(foundOrCreatedErr.Error()).To(ContainSubstring("failed-to-find-created-volume-in-baggageclaim"))
				})
			})
		})

		Context("when volume does not exist in db", func() {
			var fakeCreatedVolume *dbngfakes.FakeCreatedVolume

			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(nil, nil, nil)
				fakeLockDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
				creatingVolume := new(dbngfakes.FakeCreatingVolume)
				fakeDBVolumeFactory.CreateContainerVolumeReturns(creatingVolume, nil)
				fakeCreatedVolume = new(dbngfakes.FakeCreatedVolume)
				creatingVolume.CreatedReturns(fakeCreatedVolume, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeLockDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
			})

			Context("when volume is using ContainerRootFSStrategy", func() {
				var parentVolume *workerfakes.FakeVolume

				BeforeEach(func() {
					parentVolume = new(workerfakes.FakeVolume)
					parentVolume.HandleReturns("fake-parent-handle")
					volumeStrategy = worker.ContainerRootFSStrategy{
						Parent: parentVolume,
					}
					parentVolume.CreateChildForContainerReturns(fakeCreatingVolume, nil)
				})

				It("creates volume in creating state with parent volume", func() {
					Expect(parentVolume.CreateChildForContainerCallCount()).To(Equal(1))
					actualContainer, actualMountPath := parentVolume.CreateChildForContainerArgsForCall(0)
					Expect(actualContainer).To(Equal(container))
					Expect(actualMountPath).To(Equal("some-mount-path"))
				})
			})

			Context("when volume is using HostRootFSStrategy", func() {
				It("creates volume in creating state", func() {
					Expect(fakeDBVolumeFactory.CreateContainerVolumeCallCount()).To(Equal(1))
					actualTeamID, actualWorker, actualContainer, actualMountPath := fakeDBVolumeFactory.CreateContainerVolumeArgsForCall(0)
					Expect(actualTeamID).To(Equal(42))
					Expect(actualWorker).To(Equal(dbWorker))
					Expect(actualContainer).To(Equal(container))
					Expect(actualMountPath).To(Equal("some-mount-path"))
				})
			})

			It("creates volume in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(worker.NewVolume(fakeBaggageclaimVolume, fakeCreatedVolume)))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("CreateVolumeForResourceCache", func() {
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error

		var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
		var fakeCreatingVolume *dbngfakes.FakeCreatingVolume
		var resourcCache *dbng.UsedResourceCache
		var fakeCreatedVolume *dbngfakes.FakeCreatedVolume

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeBaggageclaimVolume.HandleReturns("created-volume")

			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)

			fakeCreatingVolume = new(dbngfakes.FakeCreatingVolume)

			resourcCache = &dbng.UsedResourceCache{ID: 52}

			fakeCreatedVolume = new(dbngfakes.FakeCreatedVolume)
			fakeDBVolumeFactory.FindResourceCacheVolumeReturns(nil, nil, nil)
			fakeDBVolumeFactory.CreateResourceCacheVolumeReturns(fakeCreatingVolume, nil)
			fakeLockDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
			fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)
		})

		JustBeforeEach(func() {
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.CreateVolumeForResourceCache(
				testLogger,
				worker.VolumeSpec{
					Strategy: worker.HostRootFSStrategy{
						Path:       "/some/path",
						WorkerName: "worker-name",
					},
					Properties: worker.VolumeProperties{
						"some": "property",
					},
					Privileged: true,
				},
				resourcCache,
			)
		})

		It("acquires the lock", func() {
			Expect(fakeLockDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
		})

		It("creates volume in creating state", func() {
			Expect(fakeDBVolumeFactory.CreateResourceCacheVolumeCallCount()).To(Equal(1))
			actualWorker, actualResourceCache := fakeDBVolumeFactory.CreateResourceCacheVolumeArgsForCall(0)
			Expect(actualWorker).To(Equal(dbWorker))
			Expect(actualResourceCache).To(Equal(resourcCache))
		})

		It("creates volume in baggageclaim", func() {
			Expect(foundOrCreatedErr).NotTo(HaveOccurred())
			Expect(foundOrCreatedVolume).To(Equal(worker.NewVolume(fakeBaggageclaimVolume, fakeCreatedVolume)))
			Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
		})
	})

	Describe("LookupVolume", func() {
		var handle string

		var found bool
		var lookupErr error

		BeforeEach(func() {
			handle = "some-handle"

			fakeCreatedVolume := new(dbngfakes.FakeCreatedVolume)
			fakeDBVolumeFactory.FindCreatedVolumeReturns(fakeCreatedVolume, true, nil)
		})

		JustBeforeEach(func() {
			_, found, lookupErr = worker.NewVolumeClient(
				fakeBaggageclaimClient,
				fakeLockDB,
				fakeDBVolumeFactory,
				fakeWorkerBaseResourceTypeFactory,
				fakeClock,
				dbWorker,
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
