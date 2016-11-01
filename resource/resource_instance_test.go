package resource_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceInstance", func() {
	var logger lager.Logger
	var resourceInstance ResourceInstance
	var fakeWorkerClient *wfakes.FakeClient

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeWorkerClient = new(wfakes.FakeClient)

		resourceInstance = NewBuildResourceInstance(
			"some-resource-type",
			atc.Version{"some": "version"},
			atc.Source{"some": "source"},
			atc.Params{"some": "params"},
			&dbng.Build{ID: 42},
		)
	})

	Describe("FindOn", func() {
		var foundVolume worker.Volume
		var found bool
		var findErr error

		JustBeforeEach(func() {
			foundVolume, found, findErr = resourceInstance.FindOn(logger, fakeWorkerClient)
		})

		Context("when one cache volume is present", func() {
			var workerVolume *wfakes.FakeVolume

			BeforeEach(func() {
				workerVolume = new(wfakes.FakeVolume)
				workerVolume.HandleReturns("found-volume-handle")
				fakeWorkerClient.ListVolumesReturns([]worker.Volume{workerVolume}, nil)
			})

			It("returns the volume and true", func() {
				Expect(foundVolume).To(Equal(workerVolume))
				Expect(found).To(BeTrue())
			})

			It("found it by querying for the correct properties", func() {
				_, spec := fakeWorkerClient.ListVolumesArgsForCall(0)
				Expect(spec).To(Equal(worker.VolumeProperties{
					"resource-type":    "some-resource-type",
					"resource-version": `{"some":"version"}`,
					"resource-source":  "968e27f71617a029e58a09fb53895f1e1875b51bdaa11293ddc2cb335960875cb42c19ae8bc696caec88d55221f33c2bcc3278a7d15e8d13f23782d1a05564f1",
					"resource-params":  "fe7d9dbc2ac75030c3e8c88e54a33676c38d8d9d2876700bc01d4961caf898e7cbe8e738232e86afcf6a5f64a9527c458a130277b08d72fb339962968d0d0967",
					"initialized":      "yep",
				}))
			})
		})

		Context("when multiple cache volumes are present", func() {
			var aVolume *wfakes.FakeVolume
			var bVolume *wfakes.FakeVolume

			BeforeEach(func() {
				aVolume = new(wfakes.FakeVolume)
				aVolume.HandleReturns("a")
				bVolume = new(wfakes.FakeVolume)
				bVolume.HandleReturns("b")
			})

			Context("with a, b order", func() {
				BeforeEach(func() {
					fakeWorkerClient.ListVolumesReturns([]worker.Volume{aVolume, bVolume}, nil)
				})

				It("selects the volume based on the lowest alphabetical name", func() {
					Expect(foundVolume).To(Equal(aVolume))
					Expect(found).To(BeTrue())

					Expect(aVolume.SetTTLCallCount()).To(Equal(0))
				})
			})

			Context("with b, a order", func() {
				BeforeEach(func() {
					fakeWorkerClient.ListVolumesReturns([]worker.Volume{bVolume, aVolume}, nil)
				})

				It("selects the volume based on the lowest alphabetical name", func() {
					Expect(foundVolume).To(Equal(aVolume))
					Expect(found).To(BeTrue())

					Expect(aVolume.SetTTLCallCount()).To(Equal(0))
				})
			})
		})

		Context("when a cache volume is not present", func() {
			BeforeEach(func() {
				fakeWorkerClient.ListVolumesReturns([]worker.Volume{}, nil)
			})

			It("does not error and returns false", func() {
				Expect(found).To(BeFalse())
				Expect(findErr).ToNot(HaveOccurred())
			})
		})
	})

	Context("CreateOn", func() {
		var createdVolume worker.Volume
		var createErr error

		JustBeforeEach(func() {
			createdVolume, createErr = resourceInstance.CreateOn(logger, fakeWorkerClient)
		})

		Context("when creating the volume succeeds", func() {
			var volume *wfakes.FakeVolume

			BeforeEach(func() {
				volume = new(wfakes.FakeVolume)
				fakeWorkerClient.CreateVolumeForResourceCacheReturns(volume, nil)
			})

			It("succeeds", func() {
				Expect(createErr).ToNot(HaveOccurred())
			})

			It("returns the volume", func() {
				Expect(createdVolume).To(Equal(volume))
			})

			It("created with the right properties", func() {
				_, spec := fakeWorkerClient.CreateVolumeForResourceCacheArgsForCall(0)
				Expect(spec).To(Equal(worker.VolumeSpec{
					Strategy: worker.NewBuildResourceCacheStrategy(
						`some-resource-type{"some":"source"}`,
						atc.Version{"some": "version"},
						&dbng.Build{ID: 42},
					),
					Properties: worker.VolumeProperties{
						"resource-type":    "some-resource-type",
						"resource-version": `{"some":"version"}`,
						"resource-source":  "968e27f71617a029e58a09fb53895f1e1875b51bdaa11293ddc2cb335960875cb42c19ae8bc696caec88d55221f33c2bcc3278a7d15e8d13f23782d1a05564f1",
						"resource-params":  "fe7d9dbc2ac75030c3e8c88e54a33676c38d8d9d2876700bc01d4961caf898e7cbe8e738232e86afcf6a5f64a9527c458a130277b08d72fb339962968d0d0967",
					},
					Privileged: true,
					TTL:        0,
				}))
			})
		})

		Context("when creating the volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerClient.CreateVolumeForResourceCacheReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(createErr).To(Equal(disaster))
			})
		})
	})

	Context("VolumeIdentifier", func() {
		It("returns a volume identifier corrsponding to the resource that the identifier is tracking", func() {
			expectedIdentifier := worker.VolumeIdentifier{
				ResourceCache: &db.ResourceCacheIdentifier{
					ResourceVersion: atc.Version{"some": "version"},
					ResourceHash:    `some-resource-type{"some":"source"}`,
				},
			}

			Expect(resourceInstance.VolumeIdentifier()).To(Equal(expectedIdentifier))
		})
	})
})

var _ = Describe("GenerateResourceHash", func() {
	It("returns a hash of the source and resource type", func() {
		Expect(GenerateResourceHash(atc.Source{"some": "source"}, "git")).To(Equal(`git{"some":"source"}`))
	})
})
