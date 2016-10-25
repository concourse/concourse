package worker_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
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
		testLogger *lagertest.TestLogger

		fakeBaggageclaimClient      *bfakes.FakeClient
		fakeGardenWorkerDB          *wfakes.FakeGardenWorkerDB
		fakeVolumeFactory           *wfakes.FakeVolumeFactory
		fakeDBVolumeFactory         *dbngfakes.FakeVolumeFactory
		fakeBaseResourceTypeFactory *dbngfakes.FakeBaseResourceTypeFactory
		workerName                  string

		volumeClient worker.VolumeClient
	)

	BeforeEach(func() {
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeVolumeFactory = new(wfakes.FakeVolumeFactory)

		workerName = "some-worker"

		testLogger = lagertest.NewTestLogger("test")

		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)
		fakeBaseResourceTypeFactory = new(dbngfakes.FakeBaseResourceTypeFactory)

		volumeClient = worker.NewVolumeClient(
			fakeBaggageclaimClient,
			fakeGardenWorkerDB,
			fakeVolumeFactory,
			fakeDBVolumeFactory,
			fakeBaseResourceTypeFactory,
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
					nil,
					fakeBaseResourceTypeFactory,
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
								ID: 2,
								Volume: db.Volume{
									Handle:     "vol-2-handle",
									Identifier: importVolumeIdentifier,
								},
							},
							{
								ID: 1,
								Volume: db.Volume{
									Handle:     "vol-1-handle",
									Identifier: importVolumeIdentifier,
								},
							},
							{
								ID: 3,
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

				fakeVolumeFactory.BuildWithIndefiniteTTLStub = func(testLogger lager.Logger, volume baggageclaim.Volume) (worker.Volume, error) {
					switch {
					case volume == bcVol2:
						return wVol2, nil
					case volume == bcVol3:
						return wVol3, nil
					}

					return new(wfakes.FakeVolume), nil
				}
			})

			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when looking up any of the extra volumes fails", func() {
				disaster := errors.New("some-error")

				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeStub = func(testLogger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
						switch {
						case handle == "vol-2-handle":
							return nil, false, disaster
						case handle == "vol-3-handle":
							return bcVol3, true, nil
						}
						return new(bfakes.FakeVolume), true, nil
					}
				})

				It("returns the error", func() {
					Expect(err).To(Equal(disaster))
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
					Expect(wVol3.DestroyCallCount()).To(Equal(1))
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
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))
					_, volume := fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))
				})

				Context("when building the worker volume succeeds", func() {
					var builtVolume *wfakes.FakeVolume

					BeforeEach(func() {
						builtVolume = new(wfakes.FakeVolume)
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(builtVolume, nil)
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
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(err).To(Equal(disaster))
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
	})

	Describe("FindOrCreateVolumeForContainer", func() {
		var fakeBaggageclaimVolume *bfakes.FakeVolume
		var foundOrCreatedVolume worker.Volume
		var foundOrCreatedErr error
		var team *dbng.Team
		var volumeWorker *dbng.Worker
		var container *dbng.CreatingContainer
		var fakeCreatingVolume *dbngfakes.FakeCreatingVolume

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(bfakes.FakeVolume)
			fakeCreatingVolume = new(dbngfakes.FakeCreatingVolume)
			fakeDBVolumeFactory.CreateContainerVolumeReturns(fakeCreatingVolume, nil)
			workerVolume := new(wfakes.FakeVolume)
			fakeVolumeFactory.BuildWithIndefiniteTTLReturns(workerVolume, nil)
		})

		JustBeforeEach(func() {
			team = &dbng.Team{}
			volumeWorker = &dbng.Worker{}
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
				volumeWorker,
				container,
				team,
				"some-mount-path",
			)
		})

		Context("when volume exists in creating state", func() {
			BeforeEach(func() {
				fakeDBVolumeFactory.FindContainerVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			Context("when volume exists in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
				})

				It("returns the volume", func() {
					Expect(foundOrCreatedErr).NotTo(HaveOccurred())
					Expect(foundOrCreatedVolume).NotTo(BeNil())
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))
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
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))
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
			})

			It("creates volume in creating state", func() {
				Expect(fakeDBVolumeFactory.CreateContainerVolumeCallCount()).To(Equal(1))
				actualTeam, actualWorker, actualContainer, actualMountPath := fakeDBVolumeFactory.CreateContainerVolumeArgsForCall(0)
				Expect(actualTeam).To(Equal(team))
				Expect(actualWorker).To(Equal(volumeWorker))
				Expect(actualContainer).To(Equal(container))
				Expect(actualMountPath).To(Equal("some-mount-path"))
			})

			It("creates volume in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).NotTo(BeNil())
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("CreateVolume", func() {
		var baggageclaimClient baggageclaim.Client

		var volumeSpec worker.VolumeSpec

		var createdVolume worker.Volume
		var createErr error

		var teamID int
		BeforeEach(func() {
			teamID = 123
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
				nil,
				fakeBaseResourceTypeFactory,
				"some-worker",
			).CreateVolume(testLogger, volumeSpec, teamID)
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
				fakeVolumeFactory.BuildWithIndefiniteTTLReturns(builtVolume, nil)
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

					_, _, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
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
					Expect(dbVolume.TeamID).To(Equal(teamID))
					Expect(dbVolume.WorkerName).To(Equal(workerName))
					Expect(dbVolume.TTL).To(Equal(volumeSpec.TTL))
					Expect(dbVolume.Identifier).To(Equal(db.VolumeIdentifier{
						ResourceCache: &db.ResourceCacheIdentifier{
							ResourceHash:    "some-resource-hash",
							ResourceVersion: atc.Version{"some": "resource-version"},
						},
					}))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(0)
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
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
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

					_, _, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
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
					Expect(dbVolume.TeamID).To(Equal(teamID))
					Expect(dbVolume.WorkerName).To(Equal(workerName))
					Expect(dbVolume.TTL).To(Equal(volumeSpec.TTL))
					Expect(dbVolume.Identifier).To(Equal(db.VolumeIdentifier{
						Output: &db.OutputIdentifier{
							Name: "some-output",
						},
					}))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(0)
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
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
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

					_, _, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
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
					Expect(dbVolume.TeamID).To(Equal(teamID))
					Expect(dbVolume.WorkerName).To(Equal(workerName))
					Expect(dbVolume.TTL).To(Equal(volumeSpec.TTL))
					Expect(dbVolume.Identifier).To(Equal(db.VolumeIdentifier{
						Import: &db.ImportIdentifier{
							WorkerName: "some-worker",
							Path:       "some-image-path",
						},
					}))
				})

				It("builds the baggageclaim.Volume into a worker.Volume", func() {
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(0)
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
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
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
				nil,
				fakeBaseResourceTypeFactory,
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
					fakeVolumeFactory.BuildWithIndefiniteTTLReturns(builtVolume, nil)
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
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(1))

					_, volume := fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume))

					Expect(foundVolume).To(Equal(builtVolume))
				})

				Context("when building the volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(lookupErr).To(Equal(disaster))
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
				nil,
				fakeBaseResourceTypeFactory,
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

					fakeVolumeFactory.BuildWithIndefiniteTTLStub = func(testLogger lager.Logger, volume baggageclaim.Volume) (worker.Volume, error) {
						switch volume.Handle() {
						case "found-volume-1":
							return builtVolume1, nil
						case "found-volume-2":
							return builtVolume2, nil
						case "found-volume-3":
							return builtVolume3, nil
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
					Expect(fakeVolumeFactory.BuildWithIndefiniteTTLCallCount()).To(Equal(3))

					_, volume := fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(0)
					Expect(volume).To(Equal(fakeBaggageclaimVolume1))

					_, volume = fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(1)
					Expect(volume).To(Equal(fakeBaggageclaimVolume2))

					_, volume = fakeVolumeFactory.BuildWithIndefiniteTTLArgsForCall(2)
					Expect(volume).To(Equal(fakeBaggageclaimVolume3))

					Expect(foundVolumes).To(ConsistOf(builtVolume1, builtVolume2, builtVolume3))
				})

				Context("when building a volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
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
