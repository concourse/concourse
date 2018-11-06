package resource_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

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
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeResourceConfig       *dbfakes.FakeResourceConfig
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

			if spec.Path == "/info" {
				return inProcess, garden.ExecutableNotFoundError{Message: "file or directory not found"}
			} else {
				return inProcess, nil
			}
		}

		fakeVolume = new(workerfakes.FakeVolume)
		fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
			{
				Volume:    fakeVolume,
				MountPath: atc.ResourcesDir("get"),
			},
		})

		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfig.SaveUncheckedVersionReturns(true, nil)
		fakeUsedResourceCache.IDReturns(42)
		fakeUsedResourceCache.ResourceConfigReturns(fakeResourceConfig)
		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeUsedResourceCache, nil)

		fakeResourceInstance = new(resourcefakes.FakeResourceInstance)
		fakeResourceInstance.ResourceCacheReturns(fakeUsedResourceCache)
		fakeResourceInstance.ContainerOwnerReturns(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"), 42))

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

	AfterEach(func() {
		cancel()
	})

	Describe("Find", func() {
		Context("when there is volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(fakeVolume, true, nil)
			})

			It("finds initialized volume and returns it", func() {
				volume, found, err := fetchSource.Find()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(volume).To(Equal(fakeVolume))
			})
		})

		Context("when there is no volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(nil, false, nil)
			})

			It("does not find volume", func() {
				volume, found, err := fetchSource.Find()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(volume).To(BeNil())
			})
		})
	})

	Describe("Create", func() {
		var (
			initErr error
			volume  worker.Volume
		)

		BeforeEach(func() {
			fakeResourceInstance.ResourceTypeReturns(resource.ResourceType("fake-resource-type"))
		})

		JustBeforeEach(func() {
			volume, initErr = fetchSource.Create(ctx)
		})

		Context("when there is initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(fakeVolume, true, nil)
			})

			It("does not fetch resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(0))
			})

			It("finds initialized volume and returns it", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(volume).To(Equal(fakeVolume))
			})
		})

		Context("when there is no initialized volume", func() {
			BeforeEach(func() {
				fakeResourceInstance.FindOnReturns(nil, false, nil)
			})

			It("creates container with volume and worker", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
				_, logger, delegate, owner, metadata, spec, types := fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(delegate).To(Equal(fakeDelegate))
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(43, atc.PlanID("some-plan-id"), 42)))
				Expect(metadata).To(BeZero())
				Expect(spec).To(Equal(worker.ContainerSpec{
					TeamID: 42,
					Tags:   []string{},
					ImageSpec: worker.ImageSpec{
						ResourceType: "fake-resource-type",
					},
					BindMounts: []worker.BindMountSource{&worker.CertsVolumeMount{Logger: logger}},
					Outputs: map[string]string{
						"resource": atc.ResourcesDir("get"),
					},
				}))
				Expect(types).To(Equal(resourceTypes))
			})

			It("fetches volume", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeContainer.RunCallCount()).To(Equal(2))
			})

			It("initializes cache", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(fakeVolume.InitializeResourceCacheCallCount()).To(Equal(1))
				rc := fakeVolume.InitializeResourceCacheArgsForCall(0)
				Expect(rc).To(Equal(fakeUsedResourceCache))
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
