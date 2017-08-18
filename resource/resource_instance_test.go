package resource_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
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
		fakeWorker               *workerfakes.FakeWorker
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		disaster                 error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		disaster = errors.New("disaster")

		resourceInstance = NewResourceInstance(
			"some-resource-type",
			atc.Version{"some": "version"},
			atc.Source{"some": "source"},
			atc.Params{"some": "params"},
			creds.VersionedResourceTypes{},
			&db.UsedResourceCache{},
			db.NewBuildStepContainerOwner(42, atc.PlanID("some-plan-id")),
		)
	})

	Describe("FindOn", func() {
		var (
			foundVolume worker.Volume
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundVolume, found, findErr = resourceInstance.FindOn(logger, fakeWorker)
		})

		Context("when initialized volume for resource cache exists on worker", func() {
			var fakeVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeVolume = new(workerfakes.FakeVolume)
				fakeWorker.FindVolumeForResourceCacheReturns(fakeVolume, true, nil)
			})

			It("returns found volume", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundVolume).To(Equal(fakeVolume))
			})
		})

		Context("when initialized volume for resource cache does not exist on worker", func() {
			BeforeEach(func() {
				fakeWorker.FindVolumeForResourceCacheReturns(nil, false, nil)
			})

			It("does not return any volume", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundVolume).To(BeNil())
			})
		})

		Context("when worker errors in finding the cache", func() {
			BeforeEach(func() {
				fakeWorker.FindVolumeForResourceCacheReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
				Expect(found).To(BeFalse())
				Expect(foundVolume).To(BeNil())
			})
		})
	})
})
