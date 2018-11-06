package radar_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check Event Handler", func() {
	var (
		handler            v2.CheckEventHandler
		spaces             map[atc.Space]atc.Version
		fakeResourceConfig *dbfakes.FakeResourceConfig
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		fakeTx := new(dbfakes.FakeTx)
		fakeResourceConfig := new(dbfakes.FakeResourceConfig)
		spaces = make(map[atc.Space]atc.Version)

		handler = NewCheckEventHandler(logger, fakeTx, fakeResourceConfig, spaces)
	})

	Describe("DefaultSpace", func() {
		BeforeEach(func() {
			err := handler.DefaultSpace(atc.Space("space"))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the space is not empty", func() {
			It("saves the default space", func() {
				Expect(fakeResourceConfig.SaveDefaultSpaceCallCount()).To(Equal(1))
				_, space := fakeResourceConfig.DefaultSpaceArgsForCall(0)
				Expect(space).To(eq(atc.Space("space")))
			})
		})

		Context("when the space is empty", func() {
			It("does not save the space", func() {
				Expect(fakeResourceConfig.SaveDefaultSpaceCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Discovered", func() {
		var (
			space    atc.Space
			version  atc.Version
			metadata atc.Metadata
		)

		BeforeEach(func() {
			space = atc.Space("space")
			version = atc.Version{"ref": "v2"}
			metadata = atc.Metadata{atc.MetadataField{Name: "name", Value: "value"}}
		})

		JustBeforeEach(func() {
			err := handler.Discovered(space, version, metadata)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the space does not exist", func() {
			It("saves the space", func() {
				Expect(fakeResourceConfig.SaveSpaceCallCount()).To(Equal(1))
				_, savedSpace := fakeResourceConfig.SaveSpaceArgsForCall(0)
				Expect(savedSpace).To(Equal(space))
			})

			It("updates the handler spaces", func() {
				Expect(spaces).To(Equal(map[atc.Space]atc.Version{space: version}))
			})
		})

		Context("when the space exists", func() {
			BeforeEach(func() {
				spaces = map[atc.Space]atc.Version{
					atc.Space("space"):       atc.Version{"ref": "v1"},
					atc.Space("other-space"): atc.Version{"ref": "v1"},
				}
			})

			It("does not save the space", func() {
				Expect(fakeResourceConfig.SaveSpaceCallCount()).To(Equal(0))
			})

			It("updates the handler spaces", func() {
				Expect(spaces).To(HaveLen(2))
				Expect(spaces).To(Equal(map[atc.Space]atc.Version{space: version, atc.Space("other-space"): atc.Version{"ref": "v1"}}))
			})
		})

		It("saves the version", func() {
			Expect(fakeResourceConfig.SaveVersionCallCount()).To(Equal(1))
			_, savedVersion := fakeResourceConfig.SaveVersionArgsForCall(0)
			Expect(savedVersion).To(Equal(atc.SpaceVersion{space, version, metadata}))
		})
	})

	Describe("LatestVersions", func() {
		JustBeforeEach(func() {
			err := handler.LatestVersions()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the handler spaces is empty", func() {
			It("does not save the latest versions", func() {
				Expect(resourceConfig.SaveSpaceLatestVersionCallCount()).To(Equal(0))
			})
		})

		Context("when the handler spaces contain latest versions", func() {
			BeforeEach(func() {
				spaces = map[atc.Space]atc.Version{
					atc.Space("space"):       atc.Version{"ref": "v1"},
					atc.Space("other-space"): atc.Version{"ref": "v2"},
				}
			})

			It("saves the latest versions", func() {
				Expect(resourceConfig.SaveSpaceLatestVersionCallCount()).To(Equal(2))

				_, space, version := resourceConfig.SaveSpaceLatestVersionArgsForCall(0)
				Expect(space).To(Equal(atc.Space("space")))
				Expect(version).To(Equal(atc.Version{"ref": "v1"}))

				_, space, version = resourceConfig.SaveSpaceLatestVersionArgsForCall(1)
				Expect(space).To(Equal(atc.Space("other-space")))
				Expect(version).To(Equal(atc.Version{"ref": "v2"}))
			})
		})
	})
})
