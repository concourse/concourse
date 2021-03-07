package build_test

import (
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"

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
		_, found := repo.ArtifactFor("first-artifact")
		Expect(found).To(BeFalse())
	})

	Context("when a artifact is registered", func() {
		BeforeEach(func() {
			repo.RegisterArtifact("first-artifact", runtimetest.NewVolume("first-handle"))
		})

		Describe("ArtifactFor", func() {
			It("yields the artifact by the given name", func() {
				artifact, found := repo.ArtifactFor("first-artifact")
				Expect(artifact.Handle()).To(Equal("first-handle"))
				Expect(found).To(BeTrue())
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
					child.RegisterArtifact("second-artifact", runtimetest.NewVolume("second-handle"))
				})

				It("is present in the child but not the parent", func() {
					Expect(handles(child.AsMap())).To(Equal(map[ArtifactName]string{
						"first-artifact":  "first-handle",
						"second-artifact": "second-handle",
					}))

					Expect(handles(repo.AsMap())).To(Equal(map[ArtifactName]string{
						"first-artifact": "first-handle",
					}))
				})
			})

			Context("when an artifact is overridden", func() {
				BeforeEach(func() {
					child.RegisterArtifact("first-artifact", runtimetest.NewVolume("modified-first-handle"))
				})

				It("is overridden in the child but not the parent", func() {
					Expect(handles(child.AsMap())).To(Equal(map[ArtifactName]string{
						"first-artifact": "modified-first-handle",
					}))

					Expect(handles(repo.AsMap())).To(Equal(map[ArtifactName]string{
						"first-artifact": "first-handle",
					}))
				})
			})
		})

		Context("when a second artifact is registered", func() {
			BeforeEach(func() {
				repo.RegisterArtifact("second-artifact", runtimetest.NewVolume("second-handle"))
			})

			Describe("AsMap", func() {
				It("returns all artifacts", func() {
					Expect(handles(repo.AsMap())).To(Equal(map[ArtifactName]string{
						"first-artifact":  "first-handle",
						"second-artifact": "second-handle",
					}))
				})
			})

			Describe("ArtifactFor", func() {
				It("yields the first artifact by the given name", func() {
					actualArtifact, found := repo.ArtifactFor("first-artifact")
					Expect(actualArtifact.Handle()).To(Equal("first-handle"))
					Expect(found).To(BeTrue())
				})

				It("yields the second artifact by the given name", func() {
					actualArtifact, found := repo.ArtifactFor("second-artifact")
					Expect(actualArtifact.Handle()).To(Equal("second-handle"))
					Expect(found).To(BeTrue())
				})

				It("yields nothing for unregistered names", func() {
					_, found := repo.ArtifactFor("bogus-artifact")
					Expect(found).To(BeFalse())
				})
			})
		})
	})
})

func handles(artifacts map[ArtifactName]runtime.Volume) map[ArtifactName]string {
	handles := make(map[ArtifactName]string)
	for name, vol := range artifacts {
		handles[name] = vol.Handle()
	}
	return handles
}
