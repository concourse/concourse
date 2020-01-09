package build_test

import (
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactRepository", func() {
	var (
		repo *Repository
	)

	BeforeEach(func() {
		repo = NewRepository()
	})

	It("initially does not contain any artifacts", func() {
		artifact, found := repo.ArtifactFor("first-artifact")
		Expect(artifact).To(BeNil())
		Expect(found).To(BeFalse())
	})

	Context("when a artifact is registered", func() {
		var firstArtifact *runtimefakes.FakeArtifact

		BeforeEach(func() {
			firstArtifact = new(runtimefakes.FakeArtifact)
			firstArtifact.IDReturns("some-first")
			repo.RegisterArtifact("first-artifact", firstArtifact)
		})

		Describe("ArtifactFor", func() {
			It("yields the artifact by the given name", func() {
				artifact, found := repo.ArtifactFor("first-artifact")
				Expect(artifact).To(Equal(firstArtifact))
				Expect(found).To(BeTrue())
			})

			It("yields nothing for unregistered names", func() {
				artifact, found := repo.ArtifactFor("bogus-artifact")
				Expect(artifact).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a second artifact is registered", func() {
			var secondArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				secondArtifact = new(runtimefakes.FakeArtifact)
				secondArtifact.IDReturns("some-second")

				repo.RegisterArtifact("second-artifact", secondArtifact)
			})

			Describe("ArtifactFor", func() {
				It("yields the first artifact by the given name", func() {
					actualArtifact, found := repo.ArtifactFor("first-artifact")
					Expect(actualArtifact).To(Equal(firstArtifact))
					Expect(found).To(BeTrue())
				})

				It("yields the second artifact by the given name", func() {
					actualArtifact, found := repo.ArtifactFor("second-artifact")
					Expect(actualArtifact).To(Equal(secondArtifact))
					Expect(found).To(BeTrue())
				})

				It("yields nothing for unregistered names", func() {
					actualArtifact, found := repo.ArtifactFor("bogus-artifact")
					Expect(actualArtifact).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})

	})
})
