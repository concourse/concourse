package resource_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceInstanceFetchSource", func() {
	var (
		fetchSource resource.FetchSource

		fakeContainer            *workerfakes.FakeContainer
		fakeVolume               *workerfakes.FakeVolume
		fakeResourceInstance     *resourcefakes.FakeResourceInstance
		fakeWorker               *workerfakes.FakeWorker
		resourceCache            *db.UsedResourceCache
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeDelegate             *workerfakes.FakeImageFetchingDelegate
		resourceTypes            creds.VersionedResourceTypes

		signals <-chan os.Signal
		ready   chan<- struct{}
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		fakeContainer = new(workerfakes.FakeContainer)
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

		fakeVolume = new(workerfakes.FakeVolume)
		fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
			{
				Volume:    fakeVolume,
				MountPath: resource.ResourcesDir("get"),
			},
		})

		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

		resourceCache = &db.UsedResourceCache{
			ID: 42,
		}

		fakeResourceInstance = new(resourcefakes.FakeResourceInstance)
		fakeResourceInstance.ResourceCacheReturns(resourceCache)
		fakeResourceInstance.ContainerOwnerReturns(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id")))
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCacheFactory.ResourceCacheMetadataReturns([]db.ResourceMetadataField{
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

		fetchSource = resource.NewResourceInstanceFetchSource(
			logger,
			fakeResourceInstance,
			fakeWorker,
			resourceTypes,
			atc.Tags{},
			42,
			resource.Session{},
			resource.EmptyMetadata{},
			fakeDelegate,
			fakeResourceCacheFactory,
		)
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
			versionedSource, initErr = fetchSource.Create(signals, ready)
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
				_, _, delegate, owner, metadata, spec, types := fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(delegate).To(Equal(fakeDelegate))
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"))))
				Expect(metadata).To(BeZero())
				Expect(spec).To(Equal(worker.ContainerSpec{
					TeamID: 42,
					Tags:   []string{},
					ImageSpec: worker.ImageSpec{
						ResourceType: "fake-resource-type",
					},
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
				Expect(rc).To(Equal(resourceCache))
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
