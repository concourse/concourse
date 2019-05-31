package resource_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/db/dbfakes"
	"github.com/concourse/concourse/v5/atc/resource"
	"github.com/concourse/concourse/v5/atc/resource/resourcefakes"
	"github.com/concourse/concourse/v5/atc/worker"
	"github.com/concourse/concourse/v5/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceInstanceFetchSource", func() {
	var (
		fetchSourceFactory resource.FetchSourceFactory
		fetchSource        resource.FetchSource

		fakeContainer            *workerfakes.FakeContainer
		fakeVolume               *workerfakes.FakeVolume
		fakeResourceInstance     *resourcefakes.FakeResourceInstance
		fakeWorker               *workerfakes.FakeWorker
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeUsedResourceCache    *dbfakes.FakeUsedResourceCache
		fakeDelegate             *workerfakes.FakeImageFetchingDelegate
		resourceTypes            creds.VersionedResourceTypes

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

		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			_, err := io.Stdout.Write([]byte("{}"))
			Expect(err).NotTo(HaveOccurred())

			return inProcess, nil
		}

		fakeVolume = new(workerfakes.FakeVolume)
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

		fakeResourceInstance = new(resourcefakes.FakeResourceInstance)
		fakeResourceInstance.ResourceCacheReturns(fakeUsedResourceCache)
		fakeResourceInstance.ContainerOwnerReturns(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"), 42))
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCacheFactory.UpdateResourceCacheMetadataReturns(nil)
		fakeResourceCacheFactory.ResourceCacheMetadataReturns([]db.ResourceConfigMetadataField{
			{Name: "some", Value: "metadata"},
		}, nil)

		fakeDelegate = new(workerfakes.FakeImageFetchingDelegate)

		variables := template.StaticVariables{
			"secret-custom": "source",
		}

		resourceTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((secret-custom))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		})

		resourceFactory := resource.NewResourceFactory()
		fetchSourceFactory = resource.NewFetchSourceFactory(fakeResourceCacheFactory, resourceFactory)
		fetchSource = fetchSourceFactory.NewFetchSource(
			logger,
			fakeWorker,
			fakeResourceInstance,
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
			resource.Session{},
			fakeDelegate,
		)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Find", func() {
		Context("when there is volume", func() {
			var expectedInitializedVersionedSource resource.VersionedSource
			BeforeEach(func() {
				expectedMetadata := []atc.MetadataField{
					{Name: "some", Value: "metadata"},
				}
				expectedInitializedVersionedSource = resource.NewGetVersionedSource(fakeVolume, fakeResourceInstance.Version(), expectedMetadata)
				fakeResourceInstance.FindOnReturns(fakeVolume, true, nil)
			})

			It("finds initialized volume and sets versioned source", func() {
				versionedSource, found, err := fetchSource.Find()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versionedSource).To(Equal(expectedInitializedVersionedSource))
			})
		})

		Context("when there is no volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(nil, false, nil)
			})

			It("does not find volume", func() {
				versionedSource, found, err := fetchSource.Find()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(versionedSource).To(BeNil())
			})
		})
	})

	Describe("Create", func() {
		var (
			initErr                 error
			versionedSource         resource.VersionedSource
			expectedVersionedSource resource.VersionedSource
		)

		BeforeEach(func() {
			fakeResourceInstance.ResourceTypeReturns(resource.ResourceType("fake-resource-type"))
		})

		JustBeforeEach(func() {
			versionedSource, initErr = fetchSource.Create(ctx)
		})

		Context("when there is initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(fakeVolume, true, nil)
				expectedMetadata := []atc.MetadataField{
					{Name: "some", Value: "metadata"},
				}
				expectedVersionedSource = resource.NewGetVersionedSource(fakeVolume, fakeResourceInstance.Version(), expectedMetadata)
			})

			It("does not fetch resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(0))
			})

			It("finds initialized volume and sets versioned source", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(versionedSource).To(Equal(expectedVersionedSource))
			})
		})

		Context("when there is no initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(nil, false, nil)
			})

			It("creates container with volume and worker", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
				_, logger, delegate, owner, metadata, containerSpec, types := fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(delegate).To(Equal(fakeDelegate))
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"), 42)))
				Expect(metadata).To(BeZero())
				Expect(containerSpec).To(Equal(worker.ContainerSpec{
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
