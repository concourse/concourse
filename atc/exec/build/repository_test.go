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
			var artifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				artifact = new(runtimefakes.FakeArtifact)
				repo.RegisterArtifact("second-artifact", artifact)
			})

			Describe("ArtifactFor", func() {
				It("yields the first artifact by the given name", func() {
					artifact, found := repo.ArtifactFor("first-artifact")
					Expect(artifact).To(Equal(firstArtifact))
					Expect(found).To(BeTrue())
				})

				It("yields the second artifact by the given name", func() {
					artifact, found := repo.ArtifactFor("second-artifact")
					Expect(artifact).To(Equal(firstArtifact))
					Expect(found).To(BeTrue())
				})

				It("yields nothing for unregistered names", func() {
					artifact, found := repo.ArtifactFor("bogus-artifact")
					Expect(artifact).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})

	})
})

