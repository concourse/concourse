package integration_test

import (
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna"
	"github.com/concourse/atc/cessna/cessnafakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RootFSable", func() {

	Describe("BaseResourceType", func() {
		It("creates a cow of an imported rootfs corresponding to the given BRT", func() {
			var callCount int
			parentVolume := new(baggageclaimfakes.FakeVolume)

			importVolume := new(baggageclaimfakes.FakeVolume)
			importVolume.PathReturns("/importpath")

			fakeBaggageClaimClient.CreateVolumeStub = func(lager.Logger, string, baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
				callCount++
				if callCount == 1 {
					return parentVolume, nil
				} else {
					return importVolume, nil
				}

			}
			brt := BaseResourceType{
				RootFSPath: "foobar",
				Name:       "test-brt",
			}

			gardenHandle, err := brt.RootFSPathFor(logger, fakeWorker)
			Expect(err).NotTo(HaveOccurred())

			Expect(gardenHandle).To(Equal("raw:///importpath"))

			_, _, spec := fakeBaggageClaimClient.CreateVolumeArgsForCall(0)
			Expect(spec).To(Equal(baggageclaim.VolumeSpec{
				Strategy: baggageclaim.ImportStrategy{
					Path: "foobar",
				},
				Privileged: true,
			}))

			_, _, spec = fakeBaggageClaimClient.CreateVolumeArgsForCall(1)
			Expect(spec).To(Equal(baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: parentVolume,
				},
				Privileged: true,
			}))
		})
	})

	Describe("ResourceGet", func() {
		It("creates a cow of the volume is returned from the get", func() {
			fakeContainer := new(gardenfakes.FakeContainer)
			fakeGardenClient.CreateReturns(fakeContainer, nil)

			fakeContainerProcess := new(gardenfakes.FakeProcess)
			fakeContainer.RunReturns(fakeContainerProcess, nil)

			fakeContainerProcess.WaitReturns(0, nil)

			fakeRootFSable := new(cessnafakes.FakeRootFSable)

			emptyVolume := new(baggageclaimfakes.FakeVolume)

			importVolume := new(baggageclaimfakes.FakeVolume)
			importVolume.PathReturns("/importpath")

			var callCount int
			fakeBaggageClaimClient.CreateVolumeStub = func(lager.Logger, string, baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
				callCount++
				if callCount == 1 {
					return emptyVolume, nil
				} else {
					return importVolume, nil
				}

			}

			resourceGet := ResourceGet{
				Resource: Resource{
					ResourceType: fakeRootFSable,
					Source:       atc.Source{},
				},
				Version: atc.Version{
					"foo": "bar",
				},
			}

			gardenHandle, err := resourceGet.RootFSPathFor(logger, fakeWorker)
			Expect(err).NotTo(HaveOccurred())

			Expect(gardenHandle).To(Equal("raw:///importpath"))

			_, _, spec := fakeBaggageClaimClient.CreateVolumeArgsForCall(0)
			Expect(spec).To(Equal(baggageclaim.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: true,
			}))

			_, _, spec = fakeBaggageClaimClient.CreateVolumeArgsForCall(1)
			Expect(spec).To(Equal(baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: emptyVolume,
				},
				Privileged: true,
			}))

		})
	})
})
