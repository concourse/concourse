package worker_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("ResourceInstanceFetchSource", func() {
	var (
		fetchSourceFactory worker.FetchSourceFactory
		fetchSource        worker.FetchSource

		fakeContainer            *workerfakes.FakeContainer
		fakeVolume               *workerfakes.FakeVolume
		fakeWorker               *workerfakes.FakeWorker
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeUsedResourceCache    *dbfakes.FakeUsedResourceCache
		fakeResource             *resourcefakes.FakeResource
		fakeDelegate             *workerfakes.FakeImageFetchingDelegate
		resourceTypes            atc.VersionedResourceTypes
		metadata                 db.ContainerMetadata
		owner                    db.ContainerOwner

		ctx    context.Context
		cancel func()
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		fakeContainer = new(workerfakes.FakeContainer)

		ctx, cancel = context.WithCancel(context.Background())

		fakeContainer.PropertyReturns("", errors.New("nope"))
		inProcess := new(gardenfakes.FakeProcess)
		inProcess.IDReturns("process-id")
		inProcess.WaitStub = func() (int, error) {
			return 0, nil
		}

		fakeContainer.AttachReturns(nil, errors.New("process not found"))

		fakeContainer.RunStub = func(ctx context.Context, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			_, err := io.Stdout.Write([]byte("{}"))
			Expect(err).NotTo(HaveOccurred())

			return inProcess, nil
		}

		fakeVolume = new(workerfakes.FakeVolume)
		fakeVolume.HandleReturns("some-handle")
		fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
			{
				Volume:    fakeVolume,
				MountPath: resource.ResourcesDir("get"),
			},
		})

		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
		fakeUsedResourceCache.IDReturns(42)
		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeUsedResourceCache, nil)

		owner = db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"), 42)
		metadata = db.ContainerMetadata{Type: db.ContainerTypeGet}

		fakeResource = new(resourcefakes.FakeResource)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCacheFactory.UpdateResourceCacheMetadataReturns(nil)
		fakeResourceCacheFactory.ResourceCacheMetadataReturns([]db.ResourceConfigMetadataField{
			{Name: "some", Value: "metadata"},
		}, nil)

		fakeDelegate = new(workerfakes.FakeImageFetchingDelegate)

		resourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		// resourceFactory := resource.NewResourceFactory()
		fetchSourceFactory = worker.NewFetchSourceFactory(fakeResourceCacheFactory)
		fetchSource = fetchSourceFactory.NewFetchSource(
			logger,
			fakeWorker,
			owner,
			fakeUsedResourceCache,
			fakeResource,
			resourceTypes,
			worker.ContainerSpec{
				TeamID: 42,
				Tags:   []string{},
				ImageSpec: worker.ImageSpec{
					ResourceType: "fake-resource-type",
				},
				Outputs: map[string]string{
					"resource": resource.ResourcesDir("get"),
				},
			},
			runtime.ProcessSpec{},
			metadata,
			fakeDelegate,
		)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Find", func() {
		var expectedGetResult worker.GetResult

		Context("when there is volume", func() {
			BeforeEach(func() {
				fakeWorker.FindVolumeForResourceCacheReturns(fakeVolume, true, nil)

				expectedMetadata := []atc.MetadataField{
					{Name: "some", Value: "metadata"},
				}
				expectedGetResult = worker.GetResult{
					Status: 0,
					VersionResult: runtime.VersionResult{Metadata: expectedMetadata},
					GetArtifact: runtime.GetArtifact{fakeVolume.Handle()},
				}
			})

			It("finds the resource cache volume and returns the correct result", func() {
				getResult, volume, found, err := fetchSource.Find()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(volume).To(Equal(fakeVolume))
				Expect(getResult).To(Equal(expectedGetResult))
			})
		})

		Context("when there is no volume", func() {
			BeforeEach(func() {
				fakeWorker.FindVolumeForResourceCacheReturns(nil, false, nil)
				expectedGetResult = worker.GetResult{}
			})

			It("does not find volume", func() {
				getResult, volume, found, err := fetchSource.Find()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(volume).To(BeNil())
				Expect(getResult).To(Equal(expectedGetResult))
			})
		})
	})

	Describe("Create", func() {
		var (
			initErr           error
			getResult         worker.GetResult
			expectedGetResult worker.GetResult
			volume            worker.Volume
		)

		JustBeforeEach(func() {
			getResult, volume, initErr = fetchSource.Create(ctx)
		})

		Context("when there is initialized volume", func() {
			BeforeEach(func() {
				fakeWorker.FindVolumeForResourceCacheReturns(fakeVolume, true, nil)

				expectedMetadata := []atc.MetadataField{
					{Name: "some", Value: "metadata"},
				}
				expectedGetResult = worker.GetResult{
					Status: 0,
					VersionResult: runtime.VersionResult{Metadata: expectedMetadata},
					GetArtifact: runtime.GetArtifact{fakeVolume.Handle()},
				}
			})

			It("does not fetch resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(0))
			})

			It("finds initialized volume and sets versioned source", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(getResult).To(Equal(expectedGetResult))
			})
		})

		Context("when there is no initialized volume", func() {
			BeforeEach(func() {
				fakeWorker.FindVolumeForResourceCacheReturns(nil, false, nil)
			})

			It("creates container with volume and worker", func() {
				Expect(initErr).NotTo(HaveOccurred())

				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
				_, logger, delegate, owner, actualMetadata, containerSpec, types := fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(delegate).To(Equal(fakeDelegate))
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"), 42)))
				Expect(actualMetadata).To(Equal(metadata))
				Expect(containerSpec).To(Equal(
					worker.ContainerSpec{
						TeamID: 42,
						Tags:   []string{},
						ImageSpec: worker.ImageSpec{
							ResourceType: "fake-resource-type",
						},
						BindMounts: []worker.BindMountSource{&worker.CertsVolumeMount{Logger: logger}},
						Outputs: map[string]string{
							"resource": resource.ResourcesDir("get"),
						},
					}))
				Expect(types).To(Equal(resourceTypes))
			})

			It("fetches versioned source", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeContainer.RunCallCount()).To(Equal(1))
			})

			It("initializes cache", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeVolume.InitializeResourceCacheCallCount()).To(Equal(1))
				rc := fakeVolume.InitializeResourceCacheArgsForCall(0)
				Expect(rc).To(Equal(fakeUsedResourceCache))
			})

			It("updates resource cache metadata", func() {
				Expect(fakeResourceCacheFactory.UpdateResourceCacheMetadataCallCount()).To(Equal(1))
				passedResourceCache, _ := fakeResourceCacheFactory.UpdateResourceCacheMetadataArgsForCall(0)
				Expect(passedResourceCache).To(Equal(fakeUsedResourceCache))
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
