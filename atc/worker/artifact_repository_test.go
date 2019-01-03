package worker_test

import (
	. "github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactRepository", func() {
	var (
		repo *ArtifactRepository
	)

	BeforeEach(func() {
		repo = NewArtifactRepository()
	})

	It("initially does not contain any sources", func() {
		source, found := repo.SourceFor("first-source")
		Expect(source).To(BeNil())
		Expect(found).To(BeFalse())
	})

	Context("when a source is registered", func() {
		var firstSource *workerfakes.FakeArtifactSource

		BeforeEach(func() {
			firstSource = new(workerfakes.FakeArtifactSource)
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
			var secondSource *workerfakes.FakeArtifactSource

			BeforeEach(func() {
				secondSource = new(workerfakes.FakeArtifactSource)
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
