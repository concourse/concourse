package resource_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchSourceProvider", func() {
	var (
		fakeWorkerClient          *workerfakes.FakeClient
		fetchSourceProvider       resource.FetchSourceProvider
		fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate

		logger                   lager.Logger
		resourceOptions          *resourcefakes.FakeResourceOptions
		resourceInstance         *resourcefakes.FakeResourceInstance
		metadata                 = resource.EmptyMetadata{}
		session                  = resource.Session{}
		tags                     atc.Tags
		resourceTypes            atc.VersionedResourceTypes
		teamID                   = 3
		resourceCache            *db.UsedResourceCache
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fetchSourceProviderFactory := resource.NewFetchSourceProviderFactory(fakeWorkerClient, fakeResourceCacheFactory)
		logger = lagertest.NewTestLogger("test")
		resourceInstance = new(resourcefakes.FakeResourceInstance)
		tags = atc.Tags{"some", "tags"}
		resourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-type",
					Type:   "docker-image",
					Source: atc.Source{"some": "repository"},
				},
				Version: atc.Version{"some": "version"},
			},
		}
		resourceOptions = new(resourcefakes.FakeResourceOptions)
		resourceOptions.ResourceTypeReturns("some-resource-type")
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)
		resourceCache = &db.UsedResourceCache{ID: 42}
		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(resourceCache, nil)

		fetchSourceProvider = fetchSourceProviderFactory.NewFetchSourceProvider(
			logger,
			session,
			metadata,
			tags,
			teamID,
			resourceTypes,
			resourceInstance,
			resourceOptions,
			fakeImageFetchingDelegate,
		)
	})

	Describe("Get", func() {
		It("tries to find satisfying worker", func() {
			_, err := fetchSourceProvider.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeWorkerClient.SatisfyingCallCount()).To(Equal(1))
			_, resourceSpec, actualResourceTypes := fakeWorkerClient.SatisfyingArgsForCall(0)
			Expect(resourceSpec).To(Equal(worker.WorkerSpec{
				ResourceType: "some-resource-type",
				Tags:         tags,
				TeamID:       teamID,
			}))
			Expect(actualResourceTypes).To(Equal(resourceTypes))
		})

		Context("when worker is found for resource types", func() {
			var fakeWorker *workerfakes.FakeWorker

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeWorkerClient.SatisfyingReturns(fakeWorker, nil)
			})

			It("returns resource instance source", func() {
				source, err := fetchSourceProvider.Get()
				Expect(err).NotTo(HaveOccurred())

				expectedSource := resource.NewResourceInstanceFetchSource(
					logger,
					resourceCache,
					resourceInstance,
					fakeWorker,
					resourceOptions,
					resourceTypes,
					tags,
					teamID,
					session,
					metadata,
					fakeImageFetchingDelegate,
					fakeResourceCacheFactory,
				)
				Expect(source).To(Equal(expectedSource))
			})
		})

		Context("when worker is not found for resource types", func() {
			var workerNotFoundErr error

			BeforeEach(func() {
				workerNotFoundErr = errors.New("not-found")
				fakeWorkerClient.SatisfyingReturns(nil, workerNotFoundErr)
			})

			It("returns an error", func() {
				_, err := fetchSourceProvider.Get()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(workerNotFoundErr))
			})
		})
	})
})
