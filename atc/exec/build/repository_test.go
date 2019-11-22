package build_test

import (
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
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

		Context("StreamFile", func() {
			var (
				source *artifactfakes.FakeRegisterableSource
				path   string
				fakeReader io.ReadCloser
				reader io.ReadCloser
				err error
			)

			BeforeEach(func() {
				fakeReader = fakeReadCloser{}
				source = new(artifactfakes.FakeRegisterableSource)
				source.StreamFileReturns(fakeReader, nil)
				repo.RegisterSource("third-source", source)
			})

			JustBeforeEach(func() {
				reader, err = repo.StreamFile(nil, nil, path)
			})

			Context("with correct path", func() {
				BeforeEach(func(){
					path = "third-source/a.txt"
				})

				It("should no error occurred", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should stream correct file content", func(){
					Expect(reader).To(Equal(fakeReader))
				})
			})

			Context("with bad path", func(){
				BeforeEach(func(){
					path = "foo"
				})
				It("should no error occurred", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(UnspecifiedArtifactSourceError{Path: "foo"}))
				})
			})

			Context("with bad source", func(){
				BeforeEach(func(){
					path = "foo/bar"
				})
				It("should no error occurred", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(UnknownArtifactSourceError{Name: "foo", Path: "foo/bar"}))
				})
			})
		})
	})
})

type fakeReadCloser struct {
}

func (r fakeReadCloser) Read(p []byte) (int, error) {
	return 1, nil
}

func (r fakeReadCloser) Close() error {
	return nil
}
