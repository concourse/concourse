package artifact_test

import (
	. "github.com/concourse/concourse/v5/atc/exec/artifact"
	"github.com/concourse/concourse/v5/atc/exec/artifact/artifactfakes"
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
	})
})
