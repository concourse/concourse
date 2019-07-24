package build_test

import (
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/build/artifactfakes"
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

	It("initially does not contain any sources", func() {
		source, found := repo.SourceFor("first-source")
		Expect(source).To(BeNil())
		Expect(found).To(BeFalse())
	})

	Context("when a source is registered", func() {
		var firstSource *artifactfakes.FakeRegisterableSource

		BeforeEach(func() {
			firstSource = new(artifactfakes.FakeRegisterableSource)
			repo.RegisterSource("first-source", firstSource)
		})

		Describe("SourceFor", func() {
			It("yields the source by the given name", func() {
				source, found := repo.SourceFor("first-source")
				Expect(source).To(Equal(firstSource))
				Expect(found).To(BeTrue())
			})

			It("yields nothing for unregistered names", func() {
				source, found := repo.SourceFor("bogus-source")
				Expect(source).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a second source is registered", func() {
			var secondSource *artifactfakes.FakeRegisterableSource

			BeforeEach(func() {
				secondSource = new(artifactfakes.FakeRegisterableSource)
				repo.RegisterSource("second-source", secondSource)
			})

			Describe("SourceFor", func() {
				It("yields the first source by the given name", func() {
					source, found := repo.SourceFor("first-source")
					Expect(source).To(Equal(firstSource))
					Expect(found).To(BeTrue())
				})

				It("yields the second source by the given name", func() {
					source, found := repo.SourceFor("second-source")
					Expect(source).To(Equal(firstSource))
					Expect(found).To(BeTrue())
				})

				It("yields nothing for unregistered names", func() {
					source, found := repo.SourceFor("bogus-source")
					Expect(source).To(BeNil())
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
