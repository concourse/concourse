package resource_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceInstance", func() {
	var (
		logger                   lager.Logger
		resourceInstance         ResourceInstance
		fakeWorkerClient         *workerfakes.FakeClient
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		disaster                 error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		disaster = errors.New("disaster")

		resourceInstance = NewResourceInstance(
			"some-resource-type",
			atc.Version{"some": "version"},
			atc.Source{"some": "source"},
			atc.Params{"some": "params"},
			db.ForBuild(42),
			db.NewBuildStepContainerOwner(42, atc.PlanID("some-plan-id")),
			atc.VersionedResourceTypes{},
			fakeResourceCacheFactory,
		)
	})

	Describe("FindOn", func() {
		var (
			foundVolume worker.Volume
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundVolume, found, findErr = resourceInstance.FindOn(logger, fakeWorkerClient)
		})

		It("'find-or-create's the resource cache with the same user", func() {
			_, user, _, _, _, _, _ := fakeResourceCacheFactory.FindOrCreateResourceCacheArgsForCall(0)
			Expect(user).To(Equal(db.ForBuild(42)))
		})

		Context("when failing to find or create cache in database", func() {
			BeforeEach(func() {
				fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
			})
		})

		Context("when initialized volume for resource cache exists on worker", func() {
			var fakeVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeVolume = new(workerfakes.FakeVolume)
				fakeWorkerClient.FindVolumeForResourceCacheReturns(fakeVolume, true, nil)
			})

			It("returns found volume", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundVolume).To(Equal(fakeVolume))
			})
		})

		Context("when initialized volume for resource cache does not exist on worker", func() {
			BeforeEach(func() {
				fakeWorkerClient.FindVolumeForResourceCacheReturns(nil, false, nil)
			})

			It("does not return any volume", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundVolume).To(BeNil())
			})
		})

		Context("when worker errors in finding the cache", func() {
			BeforeEach(func() {
				fakeWorkerClient.FindVolumeForResourceCacheReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
				Expect(found).To(BeFalse())
				Expect(foundVolume).To(BeNil())
			})
		})
	})

	Context("ResourceCacheIdentifier", func() {
		It("returns a volume identifier corrsponding to the resource that the identifier is tracking", func() {
			expectedIdentifier := worker.ResourceCacheIdentifier{
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    `some-resource-type{"some":"source"}`,
			}

			Expect(resourceInstance.ResourceCacheIdentifier()).To(Equal(expectedIdentifier))
		})
	})
})

var _ = Describe("GenerateResourceHash", func() {
	It("returns a hash of the source and resource type", func() {
		Expect(GenerateResourceHash(atc.Source{"some": "source"}, "git")).To(Equal(`git{"some":"source"}`))
	})
})
