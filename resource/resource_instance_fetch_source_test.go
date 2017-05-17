package resource_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeFetchSource", func() {
	var (
		fetchSource resource.FetchSource

		fakeContainer            *workerfakes.FakeContainer
		resourceOptions          *resourcefakes.FakeResourceOptions
		fakeVolume               *workerfakes.FakeVolume
		fakeResourceInstance     *resourcefakes.FakeResourceInstance
		fakeWorker               *workerfakes.FakeWorker
		resourceCache            *dbng.UsedResourceCache
		fakeResourceCacheFactory *dbngfakes.FakeResourceCacheFactory

		signals <-chan os.Signal
		ready   chan<- struct{}
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		fakeContainer = new(workerfakes.FakeContainer)
		resourceOptions = new(resourcefakes.FakeResourceOptions)
		signals = make(<-chan os.Signal)
		ready = make(chan<- struct{})

		fakeContainer.PropertyReturns("", errors.New("nope"))
		inProcess := new(gardenfakes.FakeProcess)
		inProcess.IDReturns("process-id")
		inProcess.WaitStub = func() (int, error) {
			return 0, nil
		}

		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			_, err := io.Stdout.Write([]byte("{}"))
			Expect(err).NotTo(HaveOccurred())

			return inProcess, nil
		}

		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.CreateResourceGetContainerReturns(fakeContainer, nil)

		fakeVolume = new(workerfakes.FakeVolume)
		fakeResourceInstance = new(resourcefakes.FakeResourceInstance)
		fakeResourceInstance.CreateOnReturns(fakeVolume, nil)
		resourceCache = &dbng.UsedResourceCache{
			ID: 42,
			Metadata: []dbng.ResourceMetadataField{
				{Name: "some", Value: "metadata"},
			},
		}
		fakeResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)
		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(resourceCache, nil)
		fetchSource = resource.NewResourceInstanceFetchSource(
			logger,
			resourceCache,
			fakeResourceInstance,
			fakeWorker,
			resourceOptions,
			nil,
			atc.Tags{},
			42,
			resource.Session{},
			resource.EmptyMetadata{},
			new(workerfakes.FakeImageFetchingDelegate),
			fakeResourceCacheFactory,
		)
	})

	Describe("FindInitialized", func() {
		Context("when there is initialized volume", func() {
			var expectedInitializedVersionedSource resource.VersionedSource
			BeforeEach(func() {
				expectedMetadata := []atc.MetadataField{
					{Name: "some", Value: "metadata"},
				}
				expectedInitializedVersionedSource = resource.NewGetVersionedSource(fakeVolume, resourceOptions.Version(), expectedMetadata)
				fakeResourceInstance.FindInitializedOnReturns(fakeVolume, true, nil)
			})

			It("finds initialized volume and sets versioned source", func() {
				versionedSource, found, err := fetchSource.FindInitialized()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versionedSource).To(Equal(expectedInitializedVersionedSource))
			})
		})

		Context("when there is no initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindInitializedOnReturns(nil, false, nil)
			})

			It("does not find initialized volume", func() {
				versionedSource, found, err := fetchSource.FindInitialized()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(versionedSource).To(BeNil())
			})
		})
	})

	Describe("Initialize", func() {
		var (
			initErr                            error
			versionedSource                    resource.VersionedSource
			expectedInitializedVersionedSource resource.VersionedSource
		)

		BeforeEach(func() {
			resourceOptions.ResourceTypeReturns(resource.ResourceType("fake-resource-type"))
		})

		JustBeforeEach(func() {
			versionedSource, initErr = fetchSource.Initialize(signals, ready)
		})

		Context("when there is initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindInitializedOnReturns(fakeVolume, true, nil)
				expectedMetadata := []atc.MetadataField{
					{Name: "some", Value: "metadata"},
				}
				expectedInitializedVersionedSource = resource.NewGetVersionedSource(fakeVolume, resourceOptions.Version(), expectedMetadata)
			})

			It("does not fetch resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeResourceInstance.CreateOnCallCount()).To(Equal(0))
				Expect(fakeContainer.RunCallCount()).To(Equal(0))
			})

			It("finds initialized volume and sets versioned source", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(versionedSource).To(Equal(expectedInitializedVersionedSource))
			})
		})

		Context("when there is no initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindInitializedOnReturns(nil, false, nil)
			})

			It("creates volume for resource instance on provided worker", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeResourceInstance.CreateOnCallCount()).To(Equal(1))
				_, worker := fakeResourceInstance.CreateOnArgsForCall(0)
				Expect(worker).To(Equal(fakeWorker))
			})

			It("creates container with volume and worker", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeWorker.CreateResourceGetContainerCallCount()).To(Equal(1))
			})

			It("fetches versioned source", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeContainer.RunCallCount()).To(Equal(1))
			})

			It("initializes cache", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeVolume.InitializeCallCount()).To(Equal(1))
			})

			It("updates resource cache metadata", func() {
				Expect(fakeResourceCacheFactory.UpdateResourceCacheMetadataCallCount()).To(Equal(1))
				passedResourceCache, _ := fakeResourceCacheFactory.UpdateResourceCacheMetadataArgsForCall(0)
				Expect(passedResourceCache).To(Equal(resourceCache))
			})

			Context("when getting resource fails with ErrAborted", func() {
				BeforeEach(func() {
					fakeContainer.RunReturns(nil, resource.ErrAborted)
				})

				It("returns ErrInterrupted", func() {
					Expect(initErr).To(HaveOccurred())
					Expect(initErr).To(Equal(resource.ErrInterrupted))
				})
			})

			Context("when getting resource fails with other error", func() {
				var disaster error

				BeforeEach(func() {
					disaster = errors.New("failed")
					fakeContainer.RunReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(initErr).To(HaveOccurred())
					Expect(initErr).To(Equal(disaster))
				})
			})
		})
	})
})
