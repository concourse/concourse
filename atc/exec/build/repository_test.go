package build_test

import (
	"github.com/concourse/concourse/atc/exec/build"
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"
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

		Describe("NewLocalScope", func() {
			var child *build.Repository

			BeforeEach(func() {
				child = repo.NewLocalScope()
			})

			It("contains the same artifacts as the parent", func() {
				Expect(child.AsMap()).To(Equal(repo.AsMap()))
			})

			It("maintains a reference to the parent", func() {
				Expect(child.Parent()).To(Equal(repo))
			})

			Context("when an artifact is registered", func() {
				var secondArtifact *runtimefakes.FakeArtifact

				BeforeEach(func() {
					secondArtifact = new(runtimefakes.FakeArtifact)
					secondArtifact.IDReturns("some-second")

					child.RegisterArtifact("second-artifact", secondArtifact)
				})

				It("is present in the child but not the parent", func() {
					Expect(child.AsMap()).To(Equal(map[build.ArtifactName]runtime.Artifact{
						"first-artifact":  firstArtifact,
						"second-artifact": secondArtifact,
					}))

					Expect(repo.AsMap()).To(Equal(map[build.ArtifactName]runtime.Artifact{
						"first-artifact": firstArtifact,
					}))
				})
			})

			Context("when an artifact is overridden", func() {
				var firstPrimeArtifact *runtimefakes.FakeArtifact

				BeforeEach(func() {
					firstPrimeArtifact = new(runtimefakes.FakeArtifact)
					firstPrimeArtifact.IDReturns("some-second")

					child.RegisterArtifact("first-artifact", firstPrimeArtifact)
				})

				It("is overridden in the child but not the parent", func() {
					Expect(child.AsMap()).To(Equal(map[build.ArtifactName]runtime.Artifact{
						"first-artifact": firstPrimeArtifact,
					}))

					Expect(repo.AsMap()).To(Equal(map[build.ArtifactName]runtime.Artifact{
						"first-artifact": firstArtifact,
					}))
				})
			})
		})

		Context("when a second artifact is registered", func() {
			var secondArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				secondArtifact = new(runtimefakes.FakeArtifact)
				secondArtifact.IDReturns("some-second")

				repo.RegisterArtifact("second-artifact", secondArtifact)
			})

			Describe("AsMap", func() {
				It("returns all artifacts", func() {
					Expect(repo.AsMap()).To(Equal(map[build.ArtifactName]runtime.Artifact{
						"first-artifact":  firstArtifact,
						"second-artifact": secondArtifact,
					}))
				})
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
