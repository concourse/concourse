package resource_test

import (
	"errors"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchSourceProvider", func() {
	var (
		fakeWorkerClient     *workerfakes.FakeClient
		fakeContainerCreator *resourcefakes.FakeFetchContainerCreator
		fetchSourceProvider  FetchSourceProvider

		logger          lager.Logger
		resourceOptions *resourcefakes.FakeResourceOptions
		cacheID         *resourcefakes.FakeCacheIdentifier
		tags            atc.Tags
		resourceTypes   atc.ResourceTypes
		teamID          = 3
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fetchSourceProviderFactory := NewFetchSourceProviderFactory(fakeWorkerClient)
		logger = lagertest.NewTestLogger("test")
		session := Session{}
		cacheID = new(resourcefakes.FakeCacheIdentifier)
		tags = atc.Tags{"some", "tags"}
		resourceTypes = atc.ResourceTypes{
			{
				Name: "some-resource-type",
			},
		}
		resourceOptions = new(resourcefakes.FakeResourceOptions)
		resourceOptions.ResourceTypeReturns("some-resource-type")
		fakeContainerCreator = new(resourcefakes.FakeFetchContainerCreator)

		fetchSourceProvider = fetchSourceProviderFactory.NewFetchSourceProvider(
			logger,
			session,
			tags,
			teamID,
			resourceTypes,
			cacheID,
			resourceOptions,
			fakeContainerCreator,
		)
	})

	Describe("Get", func() {
		Context("when container for session exists", func() {
			var fakeContainer *workerfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(workerfakes.FakeContainer)
				fakeWorkerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
			})

			It("returns container based source", func() {
				source, err := fetchSourceProvider.Get()
				Expect(err).NotTo(HaveOccurred())

				expectedSource := NewContainerFetchSource(logger, fakeContainer, resourceOptions)
				Expect(source).To(Equal(expectedSource))
			})
		})

		Context("when container for session does not exist", func() {
			BeforeEach(func() {
				fakeWorkerClient.FindContainerForIdentifierReturns(nil, false, nil)
			})

			It("tries to find satisfying worker", func() {
				_, err := fetchSourceProvider.Get()
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeWorkerClient.SatisfyingCallCount()).To(Equal(1))
				resourceSpec, actualResourceTypes := fakeWorkerClient.SatisfyingArgsForCall(0)
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

				Context("when volume is found on worker", func() {
					var fakeVolume *workerfakes.FakeVolume

					BeforeEach(func() {
						fakeVolume = new(workerfakes.FakeVolume)
						cacheID.FindOnReturns(fakeVolume, true, nil)
					})

					It("returns volume based source", func() {
						source, err := fetchSourceProvider.Get()
						Expect(err).NotTo(HaveOccurred())

						expectedSource := NewVolumeFetchSource(logger, fakeVolume, fakeWorker, resourceOptions, fakeContainerCreator)
						Expect(source).To(Equal(expectedSource))
					})
				})

				Context("when volume is not found on worker", func() {
					BeforeEach(func() {
						cacheID.FindOnReturns(nil, false, nil)
					})

					It("returns empty source", func() {
						source, err := fetchSourceProvider.Get()
						Expect(err).NotTo(HaveOccurred())

						expectedSource := NewEmptyFetchSource(logger, fakeWorker, cacheID, fakeContainerCreator, resourceOptions)
						Expect(source).To(Equal(expectedSource))
					})
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
})
