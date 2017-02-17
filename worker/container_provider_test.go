package worker_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerProvider", func() {
	var (
		logger                    *lagertest.TestLogger
		fakeImageFetchingDelegate *wfakes.FakeImageFetchingDelegate

		fakeCreatingContainer *dbngfakes.FakeCreatingContainer
		fakeCreatedContainer  *dbngfakes.FakeCreatedContainer

		fakeGardenClient            *gfakes.FakeClient
		fakeGardenContainer         *gfakes.FakeContainer
		fakeBaggageclaimClient      *baggageclaimfakes.FakeClient
		fakeVolumeClient            *wfakes.FakeVolumeClient
		fakeImageFactory            *wfakes.FakeImageFactory
		fakeImage                   *wfakes.FakeImage
		fakeDBTeam                  *dbngfakes.FakeTeam
		fakeDBVolumeFactory         *dbngfakes.FakeVolumeFactory
		fakeDBResourceCacheFactory  *dbngfakes.FakeResourceCacheFactory
		fakeDBResourceConfigFactory *dbngfakes.FakeResourceConfigFactory
		fakeGardenWorkerDB          *wfakes.FakeGardenWorkerDB
		fakeWorker                  *wfakes.FakeWorker

		containerProvider        ContainerProvider
		containerProviderFactory ContainerProviderFactory
		outputPaths              map[string]string
		inputs                   []VolumeMount

		findOrCreateErr       error
		findOrCreateContainer Container
	)

	disasterErr := errors.New("disaster")

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		inputs = []VolumeMount{}

		fakeCreatingContainer = new(dbngfakes.FakeCreatingContainer)
		fakeCreatingContainer.HandleReturns("some-handle")
		fakeCreatedContainer = new(dbngfakes.FakeCreatedContainer)

		fakeImageFetchingDelegate = new(wfakes.FakeImageFetchingDelegate)

		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeVolumeClient = new(wfakes.FakeVolumeClient)
		fakeImageFactory = new(wfakes.FakeImageFactory)
		fakeImage = new(wfakes.FakeImage)
		fakeImageFactory.GetImageReturns(fakeImage, nil)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeWorker = new(wfakes.FakeWorker)

		fakeDBTeamFactory := new(dbngfakes.FakeTeamFactory)
		fakeDBTeam = new(dbngfakes.FakeTeam)
		fakeDBTeamFactory.GetByIDReturns(fakeDBTeam)
		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)
		fakeClock := fakeclock.NewFakeClock(time.Unix(0, 123))
		fakeDBResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)
		fakeDBResourceConfigFactory = new(dbngfakes.FakeResourceConfigFactory)
		fakeGardenContainer = new(gfakes.FakeContainer)
		fakeGardenClient.CreateReturns(fakeGardenContainer, nil)

		containerProviderFactory = NewContainerProviderFactory(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeVolumeClient,
			fakeImageFactory,
			fakeDBVolumeFactory,
			fakeDBResourceCacheFactory,
			fakeDBResourceConfigFactory,
			fakeDBTeamFactory,
			fakeGardenWorkerDB,
			"http://proxy.com",
			"https://proxy.com",
			"http://noproxy.com",
			fakeClock,
		)

		containerProvider = containerProviderFactory.ContainerProviderFor(fakeWorker)
		outputPaths = map[string]string{}
	})

	ItHandlesContainerInCreatingState := func() {
		Context("when container exists in garden", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
			})

			It("does not acquire lock", func() {
				Expect(fakeGardenWorkerDB.AcquireContainerCreatingLockCallCount()).To(Equal(0))
			})

			It("marks container as created", func() {
				Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(1))
			})

			It("returns worker container", func() {
				Expect(findOrCreateContainer).ToNot(BeNil())
			})
		})

		Context("when container does not exist in garden", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{})
			})

			It("gets image", func() {
				Expect(fakeImageFactory.GetImageCallCount()).To(Equal(1))
				Expect(fakeImage.FetchForContainerCallCount()).To(Equal(1))
			})

			It("acquires lock", func() {
				Expect(fakeGardenWorkerDB.AcquireContainerCreatingLockCallCount()).To(Equal(1))
			})

			It("creates container in garden", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
			})

			It("marks container as created", func() {
				Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(1))
			})

			It("returns worker container", func() {
				Expect(findOrCreateContainer).ToNot(BeNil())
			})

			Context("when failing to create container in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.CreateReturns(nil, disasterErr)
				})

				It("returns an error", func() {
					Expect(findOrCreateErr).To(Equal(disasterErr))
				})

				It("does not mark container as created", func() {
					Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(0))
				})
			})

			Context("when getting image fails", func() {
				BeforeEach(func() {
					fakeImageFactory.GetImageReturns(nil, disasterErr)
				})

				It("returns an error", func() {
					Expect(findOrCreateErr).To(Equal(disasterErr))
				})

				It("does not create container in garden", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(0))
				})
			})
		})
	}

	ItHandlesContainerInCreatedState := func() {
		Context("when container exists in garden", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
			})

			It("returns container", func() {
				Expect(findOrCreateContainer).ToNot(BeNil())
			})
		})

		Context("when container does not exist in garden", func() {
			var containerNotFoundErr error

			BeforeEach(func() {
				containerNotFoundErr = garden.ContainerNotFoundError{}
				fakeGardenClient.LookupReturns(nil, containerNotFoundErr)
			})

			It("returns an error", func() {
				Expect(findOrCreateErr).To(Equal(containerNotFoundErr))
			})
		})
	}

	ItHandlesNonExistentContainer := func(createDatabaseCallCountFunc func() int) {
		It("gets image", func() {
			Expect(fakeImageFactory.GetImageCallCount()).To(Equal(1))
			Expect(fakeImage.FetchForContainerCallCount()).To(Equal(1))
		})

		It("creates container in database", func() {
			Expect(createDatabaseCallCountFunc()).To(Equal(1))
		})

		It("acquires lock", func() {
			Expect(fakeGardenWorkerDB.AcquireContainerCreatingLockCallCount()).To(Equal(1))
		})

		It("creates container in garden", func() {
			Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
		})

		It("marks container as created", func() {
			Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(1))
		})

		Context("when getting image fails", func() {
			BeforeEach(func() {
				fakeImageFactory.GetImageReturns(nil, disasterErr)
			})

			It("returns an error", func() {
				Expect(findOrCreateErr).To(Equal(disasterErr))
			})

			It("does not create container in database", func() {
				Expect(createDatabaseCallCountFunc()).To(Equal(0))
			})

			It("does not create container in garden", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(0))
			})
		})

		Context("when failing to create container in garden", func() {
			BeforeEach(func() {
				fakeGardenClient.CreateReturns(nil, disasterErr)
			})

			It("returns an error", func() {
				Expect(findOrCreateErr).To(Equal(disasterErr))
			})

			It("does not mark container as created", func() {
				Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(0))
			})
		})
	}

	Describe("FindOrCreateBuildContainer", func() {
		BeforeEach(func() {
			fakeDBTeam.CreateBuildContainerReturns(fakeCreatingContainer, nil)
			fakeGardenWorkerDB.AcquireContainerCreatingLockReturns(new(lockfakes.FakeLock), true, nil)
		})

		JustBeforeEach(func() {
			findOrCreateContainer, findOrCreateErr = containerProvider.FindOrCreateBuildContainer(
				logger, nil,
				fakeImageFetchingDelegate,
				Identifier{},
				Metadata{},
				ContainerSpec{
					ImageSpec: ImageSpec{},
					Inputs:    inputs,
				},
				atc.ResourceTypes{
					{
						Type:   "some-resource",
						Name:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
				},
				outputPaths,
			)
		})

		Context("when container exists in database in creating state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindBuildContainerReturns(fakeCreatingContainer, nil, nil)
			})

			ItHandlesContainerInCreatingState()
		})

		Context("when container exists in database in created state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindBuildContainerReturns(nil, fakeCreatedContainer, nil)
			})

			ItHandlesContainerInCreatedState()
		})

		Context("when container does not exist in database", func() {
			BeforeEach(func() {
				fakeDBTeam.FindBuildContainerReturns(nil, nil, nil)
			})

			ItHandlesNonExistentContainer(func() int {
				return fakeDBTeam.CreateBuildContainerCallCount()
			})
		})
	})

	Describe("FindOrCreateResourceCheckContainer", func() {
		BeforeEach(func() {
			fakeDBResourceConfigFactory.FindOrCreateResourceConfigForResourceReturns(&dbng.UsedResourceConfig{
				ID: 42,
			}, nil)
			fakeDBTeam.CreateResourceCheckContainerReturns(fakeCreatingContainer, nil)
			fakeGardenWorkerDB.AcquireContainerCreatingLockReturns(new(lockfakes.FakeLock), true, nil)
		})

		JustBeforeEach(func() {
			findOrCreateContainer, findOrCreateErr = containerProvider.FindOrCreateResourceCheckContainer(
				logger,
				nil,
				fakeImageFetchingDelegate,
				Identifier{},
				Metadata{},
				ContainerSpec{
					ImageSpec: ImageSpec{},
					Inputs:    inputs,
				},
				atc.ResourceTypes{
					{
						Type:   "some-resource",
						Name:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
				},
				"some-resource",
				atc.Source{"some": "source"},
			)
		})

		Context("when container exists in database in creating state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceCheckContainerReturns(fakeCreatingContainer, nil, nil)
			})

			ItHandlesContainerInCreatingState()
		})

		Context("when container exists in database in created state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceCheckContainerReturns(nil, fakeCreatedContainer, nil)
			})

			ItHandlesContainerInCreatedState()
		})

		Context("when container does not exist in database", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceCheckContainerReturns(nil, nil, nil)
			})

			ItHandlesNonExistentContainer(func() int {
				return fakeDBTeam.CreateResourceCheckContainerCallCount()
			})
		})
	})

	Describe("FindOrCreateResourceTypeCheckContainer", func() {
		BeforeEach(func() {
			fakeDBTeam.CreateResourceCheckContainerReturns(fakeCreatingContainer, nil)
			fakeGardenWorkerDB.AcquireContainerCreatingLockReturns(new(lockfakes.FakeLock), true, nil)
		})

		JustBeforeEach(func() {
			findOrCreateContainer, findOrCreateErr = containerProvider.FindOrCreateResourceTypeCheckContainer(
				logger,
				nil,
				fakeImageFetchingDelegate,
				Identifier{},
				Metadata{},
				ContainerSpec{
					ImageSpec: ImageSpec{},
					Inputs:    inputs,
				},
				atc.ResourceTypes{
					{
						Type:   "some-resource",
						Name:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
				},
				"some-resource",
				atc.Source{"some": "source"},
			)
		})

		Context("when container exists in database in creating state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceCheckContainerReturns(fakeCreatingContainer, nil, nil)
			})

			ItHandlesContainerInCreatingState()
		})

		Context("when container exists in database in created state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceCheckContainerReturns(nil, fakeCreatedContainer, nil)
			})

			ItHandlesContainerInCreatedState()
		})

		Context("when container does not exist in database", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceCheckContainerReturns(nil, nil, nil)
			})

			ItHandlesNonExistentContainer(func() int {
				return fakeDBTeam.CreateResourceCheckContainerCallCount()
			})
		})
	})

	Describe("FindOrCreateResourceTypeCheckContainer", func() {
		BeforeEach(func() {
			fakeDBTeam.CreateResourceGetContainerReturns(fakeCreatingContainer, nil)
			fakeGardenWorkerDB.AcquireContainerCreatingLockReturns(new(lockfakes.FakeLock), true, nil)
		})

		JustBeforeEach(func() {
			findOrCreateContainer, findOrCreateErr = containerProvider.FindOrCreateResourceGetContainer(
				logger,
				nil,
				fakeImageFetchingDelegate,
				Identifier{},
				Metadata{},
				ContainerSpec{
					ImageSpec: ImageSpec{},
					Inputs:    inputs,
				},
				atc.ResourceTypes{
					{
						Type:   "some-resource",
						Name:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
				},
				outputPaths,
				"some-resource",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{},
			)
		})

		Context("when container exists in database in creating state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceGetContainerReturns(fakeCreatingContainer, nil, nil)
			})

			ItHandlesContainerInCreatingState()
		})

		Context("when container exists in database in created state", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceGetContainerReturns(nil, fakeCreatedContainer, nil)
			})

			ItHandlesContainerInCreatedState()
		})

		Context("when container does not exist in database", func() {
			BeforeEach(func() {
				fakeDBTeam.FindResourceGetContainerReturns(nil, nil, nil)
			})

			ItHandlesNonExistentContainer(func() int {
				return fakeDBTeam.CreateResourceGetContainerCallCount()
			})
		})
	})

	Describe("FindContainerByHandle", func() {
		var (
			foundContainer Container
			findErr        error
			found          bool
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = containerProvider.FindContainerByHandle(logger, "some-container-handle", 42)
		})

		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeDBVolumeFactory.FindVolumesForContainerReturns([]dbng.CreatedVolume{}, nil)

				fakeDBTeam.FindContainerByHandleReturns(fakeCreatedContainer, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("returns the container", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Describe("the found container", func() {
				It("can be destroyed", func() {
					err := foundContainer.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("destroying via garden")
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("provider-handle"))
				})
			})

			Context("when the concourse:volumes property is present", func() {
				var (
					handle1Volume         *baggageclaimfakes.FakeVolume
					handle2Volume         *baggageclaimfakes.FakeVolume
					expectedHandle1Volume Volume
					expectedHandle2Volume Volume
				)

				BeforeEach(func() {
					handle1Volume = new(baggageclaimfakes.FakeVolume)
					handle2Volume = new(baggageclaimfakes.FakeVolume)

					fakeVolume1 := new(dbngfakes.FakeCreatedVolume)
					fakeVolume2 := new(dbngfakes.FakeCreatedVolume)

					expectedHandle1Volume = NewVolume(handle1Volume, fakeVolume1)
					expectedHandle2Volume = NewVolume(handle2Volume, fakeVolume2)

					fakeVolume1.HandleReturns("handle-1")
					fakeVolume2.HandleReturns("handle-2")

					fakeVolume1.PathReturns("/handle-1/path")
					fakeVolume2.PathReturns("/handle-2/path")

					fakeDBVolumeFactory.FindVolumesForContainerReturns([]dbng.CreatedVolume{fakeVolume1, fakeVolume2}, nil)

					fakeBaggageclaimClient.LookupVolumeStub = func(logger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
						if handle == "handle-1" {
							return handle1Volume, true, nil
						} else if handle == "handle-2" {
							return handle2Volume, true, nil
						} else {
							panic("unknown handle: " + handle)
						}
					}
				})

				Describe("VolumeMounts", func() {
					It("returns all bound volumes based on properties on the container", func() {
						Expect(foundContainer.VolumeMounts()).To(ConsistOf([]VolumeMount{
							{Volume: expectedHandle1Volume, MountPath: "/handle-1/path"},
							{Volume: expectedHandle2Volume, MountPath: "/handle-2/path"},
						}))
					})

					Context("when LookupVolume returns an error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
						})

						It("returns the error on lookup", func() {
							Expect(findErr).To(Equal(disaster))
						})
					})
				})
			})

			Context("when the user property is present", func() {
				var (
					actualSpec garden.ProcessSpec
					actualIO   garden.ProcessIO
				)

				BeforeEach(func() {
					actualSpec = garden.ProcessSpec{
						Path: "some-path",
						Args: []string{"some", "args"},
						Env:  []string{"some=env"},
						Dir:  "some-dir",
					}

					actualIO = garden.ProcessIO{}

					fakeContainer.PropertiesReturns(garden.Properties{"user": "maverick"}, nil)
				})

				JustBeforeEach(func() {
					foundContainer.Run(actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
							User: "maverick",
						}))
						Expect(io).To(Equal(garden.ProcessIO{}))
					})
				})
			})

			Context("when the user property is not present", func() {
				var (
					actualSpec garden.ProcessSpec
					actualIO   garden.ProcessIO
				)

				BeforeEach(func() {
					actualSpec = garden.ProcessSpec{
						Path: "some-path",
						Args: []string{"some", "args"},
						Env:  []string{"some=env"},
						Dir:  "some-dir",
					}

					actualIO = garden.ProcessIO{}

					fakeContainer.PropertiesReturns(garden.Properties{"user": ""}, nil)
				})

				JustBeforeEach(func() {
					foundContainer.Run(actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the default user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
							User: "root",
						}))
						Expect(io).To(Equal(garden.ProcessIO{}))
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
					})
				})
			})
		})

		Context("when the gardenClient returns garden.ContainerNotFoundError", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "some-handle"})
			})

			It("returns false and no error", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the gardenClient returns an error", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("container not found")
				fakeGardenClient.LookupReturns(nil, expectedErr)
			})

			It("returns nil and forwards the error", func() {
				Expect(findErr).To(Equal(expectedErr))

				Expect(foundContainer).To(BeNil())
			})
		})

	})

})
