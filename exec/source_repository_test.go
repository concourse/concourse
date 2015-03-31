package exec_test

import (
	"bytes"
	"errors"
	"io"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("SourceRepository", func() {
	var (
		repo *SourceRepository
	)

	BeforeEach(func() {
		repo = NewSourceRepository()
	})

	It("initially does not contain any sources", func() {
		source, found := repo.SourceFor("first-source")
		Ω(source).Should(BeNil())
		Ω(found).Should(BeFalse())
	})

	Context("when a source is registered", func() {
		var firstSource *fakes.FakeArtifactSource

		BeforeEach(func() {
			firstSource = new(fakes.FakeArtifactSource)
			repo.RegisterSource("first-source", firstSource)
		})

		Describe("SourceFor", func() {
			It("yields the source by the given name", func() {
				source, found := repo.SourceFor("first-source")
				Ω(source).Should(Equal(firstSource))
				Ω(found).Should(BeTrue())
			})

			It("yields nothing for unregistered names", func() {
				source, found := repo.SourceFor("bogus-source")
				Ω(source).Should(BeNil())
				Ω(found).Should(BeFalse())
			})
		})

		Context("when a second source is registered", func() {
			var secondSource *fakes.FakeArtifactSource

			BeforeEach(func() {
				secondSource = new(fakes.FakeArtifactSource)
				repo.RegisterSource("second-source", secondSource)
			})

			Describe("SourceFor", func() {
				It("yields the first source by the given name", func() {
					source, found := repo.SourceFor("first-source")
					Ω(source).Should(Equal(firstSource))
					Ω(found).Should(BeTrue())
				})

				It("yields the second source by the given name", func() {
					source, found := repo.SourceFor("second-source")
					Ω(source).Should(Equal(firstSource))
					Ω(found).Should(BeTrue())
				})

				It("yields nothing for unregistered names", func() {
					source, found := repo.SourceFor("bogus-source")
					Ω(source).Should(BeNil())
					Ω(found).Should(BeFalse())
				})
			})

			Describe("StreamTo", func() {
				var fakeDestination *fakes.FakeArtifactDestination
				var streamErr error

				BeforeEach(func() {
					fakeDestination = new(fakes.FakeArtifactDestination)
				})

				JustBeforeEach(func() {
					streamErr = repo.StreamTo(fakeDestination)
				})

				It("succeeds", func() {
					Ω(streamErr).ShouldNot(HaveOccurred())
				})

				It("streams both sources to the destination under subdirectories", func() {
					someStream := new(bytes.Buffer)

					Ω(firstSource.StreamToCallCount()).Should(Equal(1))
					Ω(secondSource.StreamToCallCount()).Should(Equal(1))

					firstDestination := firstSource.StreamToArgsForCall(0)
					secondDestination := secondSource.StreamToArgsForCall(0)

					Ω(firstDestination.StreamIn("foo", someStream)).Should(Succeed())

					Ω(fakeDestination.StreamInCallCount()).Should(Equal(1))
					destDir, stream := fakeDestination.StreamInArgsForCall(0)
					Ω(destDir).Should(Equal("first-source/foo"))
					Ω(stream).Should(Equal(someStream))

					Ω(secondDestination.StreamIn("foo", someStream)).Should(Succeed())

					Ω(fakeDestination.StreamInCallCount()).Should(Equal(2))
					destDir, stream = fakeDestination.StreamInArgsForCall(1)
					Ω(destDir).Should(Equal("second-source/foo"))
					Ω(stream).Should(Equal(someStream))
				})

				Context("when the any of the sources fails to stream", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						secondSource.StreamToReturns(disaster)
					})

					It("returns the error", func() {
						Ω(streamErr).Should(Equal(disaster))
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
						Ω(streamErr).Should(Equal(ErrFileNotFound))
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
						Ω(stream).Should(Equal(outStream))

						Ω(firstSource.StreamFileArgsForCall(0)).Should(Equal("foo"))
					})

					Context("when streaming out from the source fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							firstSource.StreamFileReturns(nil, disaster)
						})

						It("returns the error", func() {
							Ω(streamErr).Should(Equal(disaster))
						})
					})
				})
			})
		})
	})
})
