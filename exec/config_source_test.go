package exec_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ConfigSource", func() {
	var fakeArtifactSource *fakes.FakeArtifactSource

	BeforeEach(func() {
		fakeArtifactSource = new(fakes.FakeArtifactSource)
	})

	Describe("FileConfigSource", func() {
		var (
			configSource BuildConfigSource

			fetchedConfig atc.BuildConfig
			fetchErr      error
		)

		BeforeEach(func() {
			configSource = FileConfigSource{Path: "some/build.yml"}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(fakeArtifactSource)
		})

		Context("when the artifact source provides a proper file", func() {
			var (
				fileConfig atc.BuildConfig

				streamedOut *gbytes.Buffer
			)

			BeforeEach(func() {
				fileConfig = atc.BuildConfig{
					Image:  "some-image",
					Params: map[string]string{"PARAM": "value"},
					Run: atc.BuildRunConfig{
						Path: "ls",
						Args: []string{"-al"},
					},
					Inputs: []atc.BuildInputConfig{
						{Name: "some-input", Path: "some-path"},
					},
				}

				marshalled, err := candiedyaml.Marshal(fileConfig)
				Ω(err).ShouldNot(HaveOccurred())

				streamedOut = gbytes.BufferWithBytes(marshalled)
				fakeArtifactSource.StreamFileReturns(streamedOut, nil)
			})

			It("succeeds", func() {
				Ω(fetchErr).ShouldNot(HaveOccurred())
			})

			It("returns the unmarshalled config", func() {
				Ω(fetchedConfig).Should(Equal(fileConfig))
			})

			It("closes the stream", func() {
				Ω(streamedOut.Closed()).Should(BeTrue())
			})
		})

		Context("when the artifact source provides a malformed file", func() {
			var streamedOut *gbytes.Buffer

			BeforeEach(func() {
				streamedOut = gbytes.BufferWithBytes([]byte("bogus"))
				fakeArtifactSource.StreamFileReturns(streamedOut, nil)
			})

			It("fails", func() {
				Ω(fetchErr).Should(HaveOccurred())
			})

			It("closes the stream", func() {
				Ω(streamedOut.Closed()).Should(BeTrue())
			})
		})

		Context("when streaming the file out fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeArtifactSource.StreamFileReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(fetchErr).Should(HaveOccurred())
			})
		})
	})
})
