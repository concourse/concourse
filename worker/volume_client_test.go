package worker_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"

	wfakes "github.com/concourse/atc/worker/workerfakes"
	bfakes "github.com/concourse/baggageclaim/baggageclaimfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeClient", func() {
	var (
		fakeLock   *dbfakes.FakeLock
		testLogger *lagertest.TestLogger

		fakeBaggageclaimClient      *bfakes.FakeClient
		fakeGardenWorkerDB          *wfakes.FakeGardenWorkerDB
		fakeDBVolumeFactory         *dbngfakes.FakeVolumeFactory
		fakeBaseResourceTypeFactory *dbngfakes.FakeBaseResourceTypeFactory
		fakeClock                   *fakeclock.FakeClock
		dbWorker                    *dbng.Worker

		volumeClient worker.VolumeClient
	)

	BeforeEach(func() {
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		dbWorker = &dbng.Worker{Name: "some-worker"}

		testLogger = lagertest.NewTestLogger("test")

		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)
		fakeBaseResourceTypeFactory = new(dbngfakes.FakeBaseResourceTypeFactory)
		fakeLock = new(dbfakes.FakeLock)

		volumeClient = worker.NewVolumeClient(
			fakeBaggageclaimClient,
			fakeGardenWorkerDB,
			fakeDBVolumeFactory,
			fakeBaseResourceTypeFactory,
			fakeClock,
			dbWorker,
		)
	})

	Describe("FindOrCreateVolumeForContainer", func() {
		var fakeBaggageclaimVolume *bfakes.FakeVolume
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error
		var team *dbng.Team
		var container *dbng.CreatingContainer
		var fakeCreatingVolume *dbngfakes.FakeCreatingVolume

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(bfakes.FakeVolume)
			fakeCreatingVolume = new(dbngfakes.FakeCreatingVolume)
			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)
			fakeDBVolumeFactory.CreateContainerVolumeReturns(fakeCreatingVolume, nil)
		})

		JustBeforeEach(func() {
			team = &dbng.Team{}
			container = &dbng.CreatingContainer{}

			version := "some-version"
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.FindOrCreateVolumeForContainer(
				testLogger,
				worker.VolumeSpec{
					Strategy: worker.HostRootFSStrategy{
						Path:       "/some/path",
						WorkerName: "worker-name",
						Version:    &version,
					},
				},
				container,
				team,
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
					fakeGardenWorkerDB.AcquireVolumeCreatingLockReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeGardenWorkerDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeGardenWorkerDB.AcquireVolumeCreatingLockStub = func(logger lager.Logger, volumeID int) (db.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find volume again", func() {
					Expect(fakeGardenWorkerDB.AcquireVolumeCreatingLockCallCount()).To(Equal(3))
					Expect(fakeDBVolumeFactory.FindContainerVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeGardenWorkerDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
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
			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(nil, nil, nil)
				fakeGardenWorkerDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeGardenWorkerDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
			})

			It("creates volume in creating state", func() {
				Expect(fakeDBVolumeFactory.CreateContainerVolumeCallCount()).To(Equal(1))
				actualTeam, actualWorker, actualContainer, actualMountPath := fakeDBVolumeFactory.CreateContainerVolumeArgsForCall(0)
				Expect(actualTeam).To(Equal(team))
				Expect(actualWorker).To(Equal(dbWorker))
				Expect(actualContainer).To(Equal(container))
				Expect(actualMountPath).To(Equal("some-mount-path"))
			})

			It("creates volume in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(fakeBaggageclaimVolume))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("FindOrCreateVolumeForResourceCache", func() {
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error

		var fakeBaggageclaimVolume *wfakes.FakeVolume
		var fakeCreatingVolume *dbngfakes.FakeCreatingVolume
		var resourcCache *dbng.UsedResourceCache

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(wfakes.FakeVolume)
			fakeBaggageclaimVolume.HandleReturns("created-volume")

			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)

			fakeCreatingVolume = new(dbngfakes.FakeCreatingVolume)

			resourcCache = &dbng.UsedResourceCache{ID: 52}
		})

		JustBeforeEach(func() {
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.FindOrCreateVolumeForResourceCache(
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

		Context("when volume exists in creating state", func() {
			BeforeEach(func() {
				fakeDBVolumeFactory.FindResourceCacheVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			Context("when acquiring volume creating lock fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeGardenWorkerDB.AcquireVolumeCreatingLockReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeGardenWorkerDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeGardenWorkerDB.AcquireVolumeCreatingLockStub = func(logger lager.Logger, volumeID int) (db.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find volume again", func() {
					Expect(fakeGardenWorkerDB.AcquireVolumeCreatingLockCallCount()).To(Equal(3))
					Expect(fakeDBVolumeFactory.FindResourceCacheVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeGardenWorkerDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
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
				fakeDBVolumeFactory.FindResourceCacheVolumeReturns(nil, fakeCreatedVolume, nil)
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
			BeforeEach(func() {
				fakeDBVolumeFactory.FindResourceCacheVolumeReturns(nil, nil, nil)
				fakeDBVolumeFactory.CreateResourceCacheVolumeReturns(fakeCreatingVolume, nil)
				fakeGardenWorkerDB.AcquireVolumeCreatingLockReturns(fakeLock, true, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeGardenWorkerDB.AcquireVolumeCreatingLockCallCount()).To(Equal(1))
			})

			It("creates volume in creating state", func() {
				Expect(fakeDBVolumeFactory.CreateResourceCacheVolumeCallCount()).To(Equal(1))
				actualWorker, actualResourceCache := fakeDBVolumeFactory.CreateResourceCacheVolumeArgsForCall(0)
				Expect(actualWorker).To(Equal(dbWorker))
				Expect(actualResourceCache).To(Equal(resourcCache))
			})

			It("creates volume in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(fakeBaggageclaimVolume))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("LookupVolume", func() {
		var baggageclaimClient baggageclaim.Client

		var handle string

		var foundVolume worker.Volume
		var found bool
		var lookupErr error

		BeforeEach(func() {
			handle = "some-handle"
		})

		JustBeforeEach(func() {
			foundVolume, found, lookupErr = worker.NewVolumeClient(
				baggageclaimClient,
				fakeGardenWorkerDB,
				nil,
				fakeBaseResourceTypeFactory,
				fakeClock,
				dbWorker,
			).LookupVolume(testLogger, handle)
		})

		Context("when there is no baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = nil
			})

			It("returns false", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when there is a baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = fakeBaggageclaimClient
			})

			Context("when the volume can be found on baggageclaim", func() {
				var fakeBaggageclaimVolume *bfakes.FakeVolume
				var builtVolume *wfakes.FakeVolume

				BeforeEach(func() {
					fakeBaggageclaimVolume = new(bfakes.FakeVolume)
					fakeBaggageclaimVolume.HandleReturns(handle)

					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)

					builtVolume = new(wfakes.FakeVolume)
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

	Describe("ListVolumes", func() {
		var baggageclaimClient baggageclaim.Client

		var properties worker.VolumeProperties

		var foundVolumes []worker.Volume
		var listErr error

		BeforeEach(func() {
			properties = worker.VolumeProperties{
				"some": "properties",
			}
		})

		JustBeforeEach(func() {
			foundVolumes, listErr = worker.NewVolumeClient(
				baggageclaimClient,
				fakeGardenWorkerDB,
				nil,
				fakeBaseResourceTypeFactory,
				fakeClock,
				dbWorker,
			).ListVolumes(testLogger, properties)
		})

		Context("when there is no baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = nil
			})

			It("succeeds", func() {
				Expect(listErr).ToNot(HaveOccurred())
			})

			It("returns no volumes", func() {
				Expect(foundVolumes).To(BeEmpty())
			})
		})

		Context("when there is a baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = fakeBaggageclaimClient
			})

			Context("when the volume can be found on baggageclaim", func() {
				var fakeBaggageclaimVolume1 *bfakes.FakeVolume
				var fakeBaggageclaimVolume2 *bfakes.FakeVolume
				var fakeBaggageclaimVolume3 *bfakes.FakeVolume

				var builtVolume1 *wfakes.FakeVolume
				var builtVolume2 *wfakes.FakeVolume
				var builtVolume3 *wfakes.FakeVolume

				BeforeEach(func() {
					fakeBaggageclaimVolume1 = new(bfakes.FakeVolume)
					fakeBaggageclaimVolume1.HandleReturns("found-volume-1")

					fakeBaggageclaimVolume2 = new(bfakes.FakeVolume)
					fakeBaggageclaimVolume2.HandleReturns("found-volume-2")

					fakeBaggageclaimVolume3 = new(bfakes.FakeVolume)
					fakeBaggageclaimVolume3.HandleReturns("found-volume-3")

					fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{
						fakeBaggageclaimVolume1,
						fakeBaggageclaimVolume2,
						fakeBaggageclaimVolume3,
					}, nil)

					builtVolume1 = new(wfakes.FakeVolume)
					builtVolume2 = new(wfakes.FakeVolume)
					builtVolume3 = new(wfakes.FakeVolume)
				})

				It("succeeds", func() {
					Expect(listErr).ToNot(HaveOccurred())
				})

				It("lists up the volumes via BaggageClaim", func() {
					Expect(fakeBaggageclaimClient.ListVolumesCallCount()).To(Equal(1))

					_, listedProperties := fakeBaggageclaimClient.ListVolumesArgsForCall(0)
					Expect(listedProperties).To(Equal(baggageclaim.VolumeProperties(properties)))
				})
			})

			Context("when looking up the volume fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBaggageclaimClient.ListVolumesReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(listErr).To(Equal(disaster))
				})
			})
		})
	})
})
