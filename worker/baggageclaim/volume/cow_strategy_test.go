package volume_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/volumefakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("COWStrategy", func() {
	var (
		strategy Strategy
	)

	BeforeEach(func() {
		strategy = COWStrategy{"parent-volume"}
	})

	Describe("Materialize", func() {
		var (
			fakeFilesystem *volumefakes.FakeFilesystem

			materializedVolume FilesystemInitVolume
			materializeErr     error
		)

		BeforeEach(func() {
			fakeFilesystem = new(volumefakes.FakeFilesystem)
		})

		JustBeforeEach(func() {
			materializedVolume, materializeErr = strategy.Materialize(
				lagertest.NewTestLogger("test"),
				"some-volume",
				fakeFilesystem,
				new(volumefakes.FakeStreamer),
			)
		})

		Context("when the parent volume can be found", func() {
			var parentVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				parentVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeFilesystem.LookupVolumeReturns(parentVolume, true, nil)
			})

			Context("when creating the sub volume succeeds", func() {
				var fakeVolume *volumefakes.FakeFilesystemInitVolume

				BeforeEach(func() {
					parentVolume.NewSubvolumeReturns(fakeVolume, nil)
				})

				It("succeeds", func() {
					Expect(materializeErr).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					Expect(materializedVolume).To(Equal(fakeVolume))
				})

				It("created it with the correct handle", func() {
					handle := parentVolume.NewSubvolumeArgsForCall(0)
					Expect(handle).To(Equal("some-volume"))
				})

				It("looked up the parent with the correct handle", func() {
					handle := fakeFilesystem.LookupVolumeArgsForCall(0)
					Expect(handle).To(Equal("parent-volume"))
				})
			})

			Context("when creating the sub volume fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					parentVolume.NewSubvolumeReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(materializeErr).To(Equal(disaster))
				})
			})
		})

		Context("when no parent volume is given", func() {
			BeforeEach(func() {
				strategy = COWStrategy{""}
			})

			It("returns ErrNoParentVolumeProvided", func() {
				Expect(materializeErr).To(Equal(ErrNoParentVolumeProvided))
			})

			It("does not look it up", func() {
				Expect(fakeFilesystem.LookupVolumeCallCount()).To(Equal(0))
			})
		})

		Context("when the parent handle does not exist", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrParentVolumeNotFound", func() {
				Expect(materializeErr).To(Equal(ErrParentVolumeNotFound))
			})
		})

		Context("when looking up the parent volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(materializeErr).To(Equal(disaster))
			})
		})
	})
})
