package resource_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/db/dbfakes"
	. "github.com/concourse/concourse/v5/atc/resource"
	"github.com/concourse/concourse/v5/atc/worker"
	"github.com/concourse/concourse/v5/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceInstance", func() {
	var (
		logger           lager.Logger
		resourceInstance ResourceInstance
		fakeWorker       *workerfakes.FakeWorker
		disaster         error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeWorker = new(workerfakes.FakeWorker)
		disaster = errors.New("disaster")
		fakeResourceCache := new(dbfakes.FakeUsedResourceCache)

		resourceInstance = NewResourceInstance(
			"some-resource-type",
			atc.Version{"some": "version"},
			atc.Source{"some": "source"},
			atc.Params{"some": "params"},
			creds.VersionedResourceTypes{},
			fakeResourceCache,
			db.NewBuildStepContainerOwner(42, atc.PlanID("some-plan-id"), 1),
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
