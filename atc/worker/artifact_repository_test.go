package worker_test

import (
	"bytes"
	"errors"
	"io"

	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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

			Describe("StreamTo", func() {
				var fakeDestination *workerfakes.FakeArtifactDestination
				var streamErr error

				BeforeEach(func() {
					fakeDestination = new(workerfakes.FakeArtifactDestination)
				})

				JustBeforeEach(func() {
					streamErr = repo.StreamTo(fakeDestination)
				})

				It("succeeds", func() {
					Expect(streamErr).NotTo(HaveOccurred())
				})

				It("streams both sources to the destination under subdirectories", func() {
					someStream := new(bytes.Buffer)

					Expect(firstSource.StreamToCallCount()).To(Equal(1))
					Expect(secondSource.StreamToCallCount()).To(Equal(1))

					firstDestination := firstSource.StreamToArgsForCall(0)
					secondDestination := secondSource.StreamToArgsForCall(0)

					Expect(firstDestination.StreamIn("foo", someStream)).To(Succeed())

					Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
					destDir, stream := fakeDestination.StreamInArgsForCall(0)
					Expect(destDir).To(Equal("first-source/foo"))
					Expect(stream).To(Equal(someStream))

					Expect(secondDestination.StreamIn("foo", someStream)).To(Succeed())

					Expect(fakeDestination.StreamInCallCount()).To(Equal(2))
					destDir, stream = fakeDestination.StreamInArgsForCall(1)
					Expect(destDir).To(Equal("second-source/foo"))
					Expect(stream).To(Equal(someStream))
				})

				Context("when the any of the sources fails to stream", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						secondSource.StreamToReturns(disaster)
					})

					It("returns the error", func() {
						Expect(streamErr).To(Equal(disaster))
					})
				})
			})

			Describe("StreamFile", func() {
				var path string

				var stream io.Reader
				var streamErr error

				JustBeforeEach(func() {
					stream, streamErr = repo.StreamFile(path)
				})

				Context("from a path not referring to any source", func() {
					BeforeEach(func() {
						path = "bogus"
					})

					It("returns ErrFileNotFound", func() {
						Expect(streamErr).To(MatchError(FileNotFoundError{Path: "bogus"}))
					})
				})

				Context("from a path referring to a source", func() {
					var outStream io.ReadCloser

					BeforeEach(func() {
						path = "first-source/foo"

						outStream = gbytes.NewBuffer()
						firstSource.StreamFileReturns(outStream, nil)
					})

					It("streams out from the source", func() {
						Expect(stream).To(Equal(outStream))

						Expect(firstSource.StreamFileArgsForCall(0)).To(Equal("foo"))
					})

					Context("when streaming out from the source fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							firstSource.StreamFileReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(streamErr).To(Equal(disaster))
						})
					})
				})
			})
		})
	})
})
