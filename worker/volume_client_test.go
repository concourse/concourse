package worker_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	wfakes "github.com/concourse/atc/worker/fakes"
	bfakes "github.com/concourse/baggageclaim/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeClient", func() {
	var (
		testLogger *lagertest.TestLogger

		fakeBaggageclaimClient *bfakes.FakeClient
		fakeGardenWorkerDB     *wfakes.FakeGardenWorkerDB
		fakeVolumeFactory      *wfakes.FakeVolumeFactory
		workerName             string

		volumeClient worker.VolumeClient
	)

	BeforeEach(func() {
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeVolumeFactory = new(wfakes.FakeVolumeFactory)
		workerName = "some-worker"

		testLogger = lagertest.NewTestLogger("test")

		volumeClient = worker.NewVolumeClient(
			fakeBaggageclaimClient,
			fakeGardenWorkerDB,
			fakeVolumeFactory,
			workerName,
		)
	})

	Describe("FindVolume", func() {
		var (
			foundVolume worker.Volume
			found       bool
			err         error
		)

		JustBeforeEach(func() {
			version := "some-version"
			foundVolume, found, err = volumeClient.FindVolume(testLogger, worker.VolumeSpec{
				Strategy: worker.HostRootFSStrategy{
					Path:       "/some/path",
					WorkerName: "worker-name",
					Version:    &version,
				},
			})
		})

		Context("when there is no baggageclaim client", func() {
			BeforeEach(func() {
				volumeClient = worker.NewVolumeClient(
					nil,
					fakeGardenWorkerDB,
					fakeVolumeFactory,
					"some-worker",
				)
			})

			It("returns ErrNoVolumeManager", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(worker.ErrNoVolumeManager))
				Expect(found).To(BeFalse())
			})
		})

		It("tries to find the volume in the db", func() {
			Expect(fakeGardenWorkerDB.GetVolumesByIdentifierCallCount()).To(Equal(1))
			version := "some-version"
			Expect(fakeGardenWorkerDB.GetVolumesByIdentifierArgsForCall(0)).To(Equal(db.VolumeIdentifier{
				Import: &db.ImportIdentifier{
					Path:       "/some/path",
					WorkerName: "worker-name",
					Version:    &version,
				},
			}))
		})

		Context("when many matching volumes are found in the db", func() {
			var bcVol2, bcVol3 *bfakes.FakeVolume
			var wVol2, wVol3 *wfakes.FakeVolume

			BeforeEach(func() {
				version1 := "some-version"
				importVolumeIdentifier := db.VolumeIdentifier{
					Import: &db.ImportIdentifier{
						WorkerName: "some-worker",
						Path:       "some/path",
						Version:    &version1,
					},
				}

				fakeGardenWorkerDB.GetVolumesByIdentifierStub = func(id db.VolumeIdentifier) ([]db.SavedVolume, error) {
					if id.Import.Version != nil {
						return []db.SavedVolume{
							{
								Volume: db.Volume{
									Handle:     "vol-1-handle",
									Identifier: importVolumeIdentifier,
								},
							},
							{
								Volume: db.Volume{
									Handle:     "vol-2-handle",
									Identifier: importVolumeIdentifier,
								},
							},
							{
								Volume: db.Volume{
									Handle:     "vol-3-handle",
									Identifier: importVolumeIdentifier,
								},
							},
						}, nil
					}

					return []db.SavedVolume{}, nil
				}

				bcVol2 = new(bfakes.FakeVolume)
				bcVol3 = new(bfakes.FakeVolume)

				fakeBaggageclaimClient.LookupVolumeStub = func(testLogger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
					switch {
					case handle == "vol-2-handle":
						return bcVol2, true, nil
					case handle == "vol-3-handle":
						return bcVol3, true, nil
					}
					return new(bfakes.FakeVolume), true, nil
				}

				wVol2 = new(wfakes.FakeVolume)
				wVol3 = new(wfakes.FakeVolume)

				fakeVolumeFactory.BuildStub = func(testLogger lager.Logger, volume baggageclaim.Volume) (worker.Volume, bool, error) {
					switch {
					case volume == bcVol2:
						return wVol2, true, nil
					case volume == bcVol3:
						return wVol3, true, nil
					}

					return new(wfakes.FakeVolume), true, nil
				}
			})

			It("releases all of the volumes except the oldest one", func() {
				Expect(wVol2.ReleaseCallCount()).To(Equal(1))
				Expect(wVol2.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(5 * time.Minute)))

				Expect(wVol3.ReleaseCallCount()).To(Equal(1))
				Expect(wVol3.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(5 * time.Minute)))
			})

			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when looking up any of the extra volumes fails", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeStub = func(testLogger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
						switch {
						case handle == "vol-2-handle":
							return nil, false, errors.New("some-error")
						case handle == "vol-3-handle":
							return bcVol3, true, nil
						}
						return new(bfakes.FakeVolume), true, nil
					}
				})

				It("should continue to the next volume", func() {
					Expect(wVol3.ReleaseCallCount()).To(Equal(1))
					Expect(wVol3.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(5 * time.Minute)))
				})
			})

			Context("when a volume which is going to be expired can't be found", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeStub = func(testLogger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
						switch {
						case handle == "vol-2-handle":
							return nil, false, nil
						case handle == "vol-3-handle":
							return bcVol3, true, nil
						}
						return new(bfakes.FakeVolume), true, nil
					}
				})

				It("should continue to the next volume", func() {
					Expect(wVol3.ReleaseCallCount()).To(Equal(1))
					Expect(wVol3.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(5 * time.Minute)))
				})
			})
		})

		Context("when the volume is found in the db", func() {
			BeforeEach(func() {
				fakeGardenWorkerDB.GetVolumesByIdentifierReturns([]db.SavedVolume{
					{
						Volume: db.Volume{
							Handle: "db-vol-handle",
						},
					},
				}, nil)
			})

			It("tries to find the db volume in baggageclaim", func() {
				Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))
				_, actualHandle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
				Expect(actualHandle).To(Equal("db-vol-handle"))
			})

			Context("when the volume can be found in baggageclaim", func() {
				var fakeBaggageclaimVolume *bfakes.FakeVolume

				BeforeEach(func() {
					fakeBaggageclaimVolume = new(bfakes.FakeVolume)
					fakeBaggageclaimVolume.HandleReturns("bg-vol-handle")

					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)

				})

				It("tries to build the worker volume", func() {
					Expect(fakeVolumeFactory.BuildCallCount()).To(Equal(1))
					_, volume := fakeVolumeFactory.BuildArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))
				})

				Context("when building the worker volume succeeds", func() {
					var builtVolume *wfakes.FakeVolume

					BeforeEach(func() {
						builtVolume = new(wfakes.FakeVolume)
						fakeVolumeFactory.BuildReturns(builtVolume, true, nil)
					})

					It("returns the worker volume", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(foundVolume).To(Equal(builtVolume))
					})
				})

				Context("when building the worker volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, disaster)
					})

					It("returns the error", func() {
						Expect(err).To(Equal(disaster))
						Expect(found).To(BeFalse())
					})
				})

				Context("when the volume ttl cannot be found in the database", func() {
					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, nil)
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeFalse())
					})
				})
			})

			Context("when the volume cannot be found in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
				})

				It("does not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
				})

				It("reaps the volume from the db", func() {
					Expect(fakeGardenWorkerDB.ReapVolumeCallCount()).To(Equal(1))
					Expect(fakeGardenWorkerDB.ReapVolumeArgsForCall(0)).To(Equal("db-vol-handle"))
				})
			})

			Context("when looking up the volume in baggageclaim fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
				})

				It("returns the error", func() {
					Expect(err).To(Equal(disaster))
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when the volume is not found in the db", func() {
			BeforeEach(func() {
				fakeGardenWorkerDB.GetVolumesByIdentifierReturns(nil, nil)
			})

			It("returns an error", func() {
				Expect(err).To(Equal(worker.ErrMissingVolume))
				Expect(found).To(BeFalse())
			})
		})

		Context("when finding the volume in the db results in an error", func() {
			var dbErr error

			BeforeEach(func() {
				dbErr = errors.New("an-error")
				fakeGardenWorkerDB.GetVolumesByIdentifierReturns(nil, dbErr)
			})

			It("returns an error", func() {
				Expect(err).To(Equal(dbErr))
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("CreateVolume", func() {
		var baggageclaimClient baggageclaim.Client

		var volumeSpec worker.VolumeSpec

		var createdVolume worker.Volume
		var createErr error

		BeforeEach(func() {
			volumeSpec = worker.VolumeSpec{
				Properties: worker.VolumeProperties{
					"some": "property",
				},
				Privileged: true,
				TTL:        6 * time.Minute,
			}
		})

		JustBeforeEach(func() {
			createdVolume, createErr = worker.NewVolumeClient(
				baggageclaimClient,
				fakeGardenWorkerDB,
				fakeVolumeFactory,
				"some-worker",
			).CreateVolume(testLogger, volumeSpec)
		})

		Context("when there is no baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = nil
			})

			It("returns ErrNoVolumeManager", func() {
				Expect(createErr).To(Equal(worker.ErrNoVolumeManager))
			})
		})

		Context("when there is a baggageclaim client", func() {
			var fakeBaggageclaimVolume *bfakes.FakeVolume
			var builtVolume *wfakes.FakeVolume

			BeforeEach(func() {
				baggageclaimClient = fakeBaggageclaimClient

				fakeBaggageclaimVolume = new(bfakes.FakeVolume)
				fakeBaggageclaimVolume.HandleReturns("created-volume")

				fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)

				builtVolume = new(wfakes.FakeVolume)
				fakeVolumeFactory.BuildReturns(builtVolume, true, nil)
			})

			Context("when creating a ResourceCacheStrategy volume", func() {
				BeforeEach(func() {
					volumeSpec.Strategy = worker.ResourceCacheStrategy{
						ResourceHash:    "some-resource-hash",
						ResourceVersion: atc.Version{"some": "resource-version"},
					}
				})

				It("succeeds", func() {
					Expect(createErr).ToNot(HaveOccurred())
				})

				It("creates the volume via BaggageClaim", func() {
					Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))

					_, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
					Expect(spec).To(Equal(baggageclaim.VolumeSpec{
						Strategy:   baggageclaim.EmptyStrategy{},
						Properties: baggageclaim.VolumeProperties(volumeSpec.Properties),
						TTL:        volumeSpec.TTL,
						Privileged: volumeSpec.Privileged,
					}))
				})

				It("inserts the volume into the database", func() {
					Expect(fakeGardenWorkerDB.InsertVolumeCallCount()).To(Equal(1))

					dbVolume := fakeGardenWorkerDB.InsertVolumeArgsForCall(0)
					Expect(dbVolume).To(Equal(db.Volume{
						Handle:     "created-volume",
						WorkerName: workerName,
						TTL:        volumeSpec.TTL,
						Identifier: db.VolumeIdentifier{
							ResourceCache: &db.ResourceCacheIdentifier{
								ResourceHash:    "some-resource-hash",
								ResourceVersion: atc.Version{"some": "resource-version"},
							},
						},
					}))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))

					Expect(createdVolume).To(Equal(builtVolume))
				})

				Context("when creating the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeBaggageclaimClient.CreateVolumeReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when inserting the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeGardenWorkerDB.InsertVolumeReturns(disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when building the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when building the volume cannot find the volume in the database", func() {
					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, nil)
					})

					It("returns ErrMissingVolume", func() {
						Expect(createErr).To(Equal(worker.ErrMissingVolume))
					})
				})
			})

			Context("when creating a OutputStrategy volume", func() {
				BeforeEach(func() {
					volumeSpec.Strategy = worker.OutputStrategy{
						Name: "some-output",
					}
				})

				It("succeeds", func() {
					Expect(createErr).ToNot(HaveOccurred())
				})

				It("creates the volume via BaggageClaim", func() {
					Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))

					_, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
					Expect(spec).To(Equal(baggageclaim.VolumeSpec{
						Strategy:   baggageclaim.EmptyStrategy{},
						Properties: baggageclaim.VolumeProperties(volumeSpec.Properties),
						TTL:        volumeSpec.TTL,
						Privileged: volumeSpec.Privileged,
					}))
				})

				It("inserts the volume into the database", func() {
					Expect(fakeGardenWorkerDB.InsertVolumeCallCount()).To(Equal(1))

					dbVolume := fakeGardenWorkerDB.InsertVolumeArgsForCall(0)
					Expect(dbVolume).To(Equal(db.Volume{
						Handle:     "created-volume",
						WorkerName: workerName,
						TTL:        volumeSpec.TTL,
						Identifier: db.VolumeIdentifier{
							Output: &db.OutputIdentifier{
								Name: "some-output",
							},
						},
					}))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))

					Expect(createdVolume).To(Equal(builtVolume))
				})

				Context("when creating the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeBaggageclaimClient.CreateVolumeReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when inserting the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeGardenWorkerDB.InsertVolumeReturns(disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when building the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when building the volume cannot find the volume in the database", func() {
					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, nil)
					})

					It("returns ErrMissingVolume", func() {
						Expect(createErr).To(Equal(worker.ErrMissingVolume))
					})
				})
			})

			Context("when creating an HostRootFSStrategy volume", func() {
				BeforeEach(func() {
					volumeSpec.Strategy = worker.HostRootFSStrategy{
						Path:       "some-image-path",
						WorkerName: workerName,
					}
				})

				It("succeeds", func() {
					Expect(createErr).ToNot(HaveOccurred())
				})

				It("creates the volume via BaggageClaim", func() {
					Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))

					_, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
					Expect(spec).To(Equal(baggageclaim.VolumeSpec{
						Strategy:   baggageclaim.ImportStrategy{Path: "some-image-path"},
						Properties: baggageclaim.VolumeProperties(volumeSpec.Properties),
						TTL:        volumeSpec.TTL,
						Privileged: volumeSpec.Privileged,
					}))
				})

				It("inserts the volume into the database", func() {
					Expect(fakeGardenWorkerDB.InsertVolumeCallCount()).To(Equal(1))

					dbVolume := fakeGardenWorkerDB.InsertVolumeArgsForCall(0)
					Expect(dbVolume).To(Equal(db.Volume{
						Handle:     "created-volume",
						WorkerName: workerName,
						TTL:        volumeSpec.TTL,
						Identifier: db.VolumeIdentifier{
							Import: &db.ImportIdentifier{
								WorkerName: "some-worker",
								Path:       "some-image-path",
							},
						},
					}))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))

					Expect(createdVolume).To(Equal(builtVolume))
				})

				Context("when creating the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeBaggageclaimClient.CreateVolumeReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when inserting the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeGardenWorkerDB.InsertVolumeReturns(disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when building the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})

				Context("when building the volume cannot find the volume in the database", func() {
					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, nil)
					})

					It("returns ErrMissingVolume", func() {
						Expect(createErr).To(Equal(worker.ErrMissingVolume))
					})
				})
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
				fakeVolumeFactory,
				workerName,
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
					fakeVolumeFactory.BuildReturns(builtVolume, true, nil)
				})

				It("succeeds", func() {
					Expect(lookupErr).ToNot(HaveOccurred())
				})

				It("looks up the volume via BaggageClaim", func() {
					Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))

					_, lookedUpHandle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
					Expect(lookedUpHandle).To(Equal(handle))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))

					Expect(foundVolume).To(Equal(builtVolume))
				})

				Context("when building the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, disaster)
					})

					It("returns the error", func() {
						Expect(lookupErr).To(Equal(disaster))
					})
				})

				Context("when building the volume cannot find the volume in the database", func() {
					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, nil)
					})

					It("returns false", func() {
						Expect(found).To(BeFalse())
					})
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
				fakeVolumeFactory,
				workerName,
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
					builtVolume3 = new(wfakes.FakeVolume)

					fakeVolumeFactory.BuildStub = func(testLogger lager.Logger, volume baggageclaim.Volume) (worker.Volume, bool, error) {
						switch volume.Handle() {
						case "found-volume-1":
							return builtVolume1, true, nil
						case "found-volume-2":
							return nil, false, nil
						case "found-volume-3":
							return builtVolume3, true, nil
						default:
							panic("unknown volume: " + volume.Handle())
						}
					}
				})

				It("succeeds", func() {
					Expect(listErr).ToNot(HaveOccurred())
				})

				It("lists up the volumes via BaggageClaim", func() {
					Expect(fakeBaggageclaimClient.ListVolumesCallCount()).To(Equal(1))

					_, listedProperties := fakeBaggageclaimClient.ListVolumesArgsForCall(0)
					Expect(listedProperties).To(Equal(baggageclaim.VolumeProperties(properties)))
				})

				It("builds the baggageclaim.Volumes into a worker.Volume, omitting those who are not found in the database", func() {
					Expect(fakeVolumeFactory.BuildCallCount()).To(Equal(3))

					_, volume := fakeVolumeFactory.BuildArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume1))

					_, volume = fakeVolumeFactory.BuildArgsForCall(1)
					Expect(volume).To(Equal(fakeBaggageclaimVolume2))

					_, volume = fakeVolumeFactory.BuildArgsForCall(2)
					Expect(volume).To(Equal(fakeBaggageclaimVolume3))

					Expect(foundVolumes).To(Equal([]worker.Volume{builtVolume1, builtVolume3}))
				})

				Context("when building a volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildReturns(nil, false, disaster)
					})

					It("returns the error", func() {
						Expect(listErr).To(Equal(disaster))
					})
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
