package build_test

import (
	"context"
	"io"

	"github.com/concourse/concourse/atc/compression"
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Artifact string

func (a Artifact) StreamOut(_ context.Context, _ string, _ compression.Compression) (io.ReadCloser, error) {
	panic("unimplemented")
}

var _ = Describe("ArtifactRepository", func() {
	var (
		repo *Repository
	)

	BeforeEach(func() {
		repo = NewRepository()
	})

	It("initially does not contain any artifacts", func() {
		_, found := repo.ArtifactFor("first-artifact")
		Expect(found).To(BeFalse())
	})

	Context("when a artifact is registered", func() {
		BeforeEach(func() {
			repo.RegisterArtifact("first-artifact", Artifact("first"))
		})

		Describe("ArtifactFor", func() {
			It("yields the artifact by the given name", func() {
				artifact, found := repo.ArtifactFor("first-artifact")
				Expect(found).To(BeTrue())
				Expect(artifact).To(Equal(Artifact("first")))
			})

			It("yields nothing for unregistered names", func() {
				_, found := repo.ArtifactFor("bogus-artifact")
				Expect(found).To(BeFalse())
			})
		})

		Describe("NewLocalScope", func() {
			var child *Repository

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
				BeforeEach(func() {
					child.RegisterArtifact("second-artifact", Artifact("second"))
				})

				It("is present in the child but not the parent", func() {
					Expect(child.AsMap()).To(Equal(map[ArtifactName]runtime.Artifact{
						"first-artifact":  Artifact("first"),
						"second-artifact": Artifact("second"),
					}))

					Expect(repo.AsMap()).To(Equal(map[ArtifactName]runtime.Artifact{
						"first-artifact": Artifact("first"),
					}))
				})
			})

			Context("when an artifact is overridden", func() {
				BeforeEach(func() {
					child.RegisterArtifact("first-artifact", Artifact("modified-first"))
				})

				It("is overridden in the child but not the parent", func() {
					Expect(child.AsMap()).To(Equal(map[ArtifactName]runtime.Artifact{
						"first-artifact": Artifact("modified-first"),
					}))

					Expect(repo.AsMap()).To(Equal(map[ArtifactName]runtime.Artifact{
						"first-artifact": Artifact("first"),
					}))
				})
			})
		})

		Context("when a second artifact is registered", func() {
			BeforeEach(func() {
				repo.RegisterArtifact("second-artifact", Artifact("second"))
			})

			Describe("AsMap", func() {
				It("returns all artifacts", func() {
					Expect(repo.AsMap()).To(Equal(map[ArtifactName]runtime.Artifact{
						"first-artifact":  Artifact("first"),
						"second-artifact": Artifact("second"),
					}))
				})
			})

			Describe("ArtifactFor", func() {
				It("yields the first artifact by the given name", func() {
					actualArtifact, found := repo.ArtifactFor("first-artifact")
					Expect(found).To(BeTrue())
					Expect(actualArtifact).To(Equal(Artifact("first")))
				})

				It("yields the second artifact by the given name", func() {
					actualArtifact, found := repo.ArtifactFor("second-artifact")
					Expect(found).To(BeTrue())
					Expect(actualArtifact).To(Equal(Artifact("second")))
				})

				It("yields nothing for unregistered names", func() {
					_, found := repo.ArtifactFor("bogus-artifact")
					Expect(found).To(BeFalse())
				})
			})
		})
	})
})
