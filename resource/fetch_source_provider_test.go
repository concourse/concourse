package resource_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchSourceProvider", func() {
	var (
		fakeWorkerClient          *workerfakes.FakeClient
		fetchSourceProvider       FetchSourceProvider
		fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate

		logger           lager.Logger
		resourceOptions  *resourcefakes.FakeResourceOptions
		resourceInstance *resourcefakes.FakeResourceInstance
		metadata         = EmptyMetadata{}
		session          = Session{}
		tags             atc.Tags
		resourceTypes    atc.ResourceTypes
		teamID           = 3
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fetchSourceProviderFactory := NewFetchSourceProviderFactory(fakeWorkerClient)
		logger = lagertest.NewTestLogger("test")
		resourceInstance = new(resourcefakes.FakeResourceInstance)
		tags = atc.Tags{"some", "tags"}
		resourceTypes = atc.ResourceTypes{
			{
				Name: "some-resource-type",
			},
		}
		resourceOptions = new(resourcefakes.FakeResourceOptions)
		resourceOptions.ResourceTypeReturns("some-resource-type")
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

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
		Context("when container for session exists", func() {
			var fakeContainer *workerfakes.FakeContainer
			var fakeVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeContainer = new(workerfakes.FakeContainer)
				fakeVolume = new(workerfakes.FakeVolume)
				fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
					worker.VolumeMount{
						Volume:    fakeVolume,
						MountPath: "/tmp/build/get",
					},
				})

				fakeWorkerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
			})

			It("returns container based source", func() {
				source, err := fetchSourceProvider.Get()
				Expect(err).NotTo(HaveOccurred())

				expectedSource := NewContainerFetchSource(logger, fakeContainer, fakeVolume, resourceOptions)
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
				var fakeVolume *workerfakes.FakeVolume

				BeforeEach(func() {
					fakeWorker = new(workerfakes.FakeWorker)
					fakeVolume = new(workerfakes.FakeVolume)
					fakeWorkerClient.SatisfyingReturns(fakeWorker, nil)
					resourceInstance.FindOrCreateOnReturns(fakeVolume, nil)
				})

				It("returns volume based source", func() {
					source, err := fetchSourceProvider.Get()
					Expect(resourceInstance.FindOrCreateOnCallCount()).To(Equal(1))
					Expect(err).NotTo(HaveOccurred())

					expectedSource := NewVolumeFetchSource(
						logger,
						fakeVolume,
						fakeWorker,
						resourceOptions,
						resourceTypes,
						tags,
						teamID,
						session,
						metadata,
						fakeImageFetchingDelegate,
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
})
