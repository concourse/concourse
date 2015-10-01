package exec_test

import (
	"errors"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ConfigSource", func() {
	var (
		someConfig = atc.TaskConfig{
			Platform: "some-platform",
			Tags:     []string{"some", "tags"},
			Image:    "some-image",
			Params:   map[string]string{"PARAM": "value"},
			Run: atc.TaskRunConfig{
				Path: "ls",
				Args: []string{"-al"},
			},
			Inputs: []atc.TaskInputConfig{
				{Name: "some-input", Path: "some-path"},
			},
		}

		repo *SourceRepository
	)

	BeforeEach(func() {
		repo = NewSourceRepository()
	})

	Describe("StaticConfigSource", func() {
		var (
			configSource TaskConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			configSource = StaticConfigSource{Config: someConfig}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		It("succeeds", func() {
			Expect(fetchErr).NotTo(HaveOccurred())
		})

		It("returns the static config", func() {
			Expect(fetchedConfig).To(Equal(someConfig))
		})
	})

	Describe("FileConfigSource", func() {
		var (
			configSource FileConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			configSource = FileConfigSource{Path: "some/build.yml"}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when the path does not indicate an artifact source", func() {
			BeforeEach(func() {
				configSource.Path = "foo-bar.yml"
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(UnspecifiedArtifactSourceError{"foo-bar.yml"}))
			})
		})

		Context("when the file's artifact source can be found in the repository", func() {
			var fakeArtifactSource *fakes.FakeArtifactSource

			BeforeEach(func() {
				fakeArtifactSource = new(fakes.FakeArtifactSource)
				repo.RegisterSource("some", fakeArtifactSource)
			})

			Context("when the artifact source provides a proper file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					marshalled, err := yaml.Marshal(someConfig)
					Expect(err).NotTo(HaveOccurred())

					streamedOut = gbytes.BufferWithBytes(marshalled)
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("fetches the file via the correct path", func() {
					Expect(fakeArtifactSource.StreamFileArgsForCall(0)).To(Equal("build.yml"))
				})

				It("succeeds", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("returns the unmarshalled config", func() {
					Expect(fetchedConfig).To(Equal(someConfig))
				})

				It("closes the stream", func() {
					Expect(streamedOut.Closed()).To(BeTrue())
				})
			})

			Context("when the artifact source provides an invalid configuration", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					invalidConfig := someConfig
					invalidConfig.Platform = ""
					invalidConfig.Run = atc.TaskRunConfig{}

					marshalled, err := yaml.Marshal(invalidConfig)
					Expect(err).NotTo(HaveOccurred())

					streamedOut = gbytes.BufferWithBytes(marshalled)
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("returns an error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})

			Context("when the artifact source provides a malformed file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					streamedOut = gbytes.BufferWithBytes([]byte("bogus"))
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("fails", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("closes the stream", func() {
					Expect(streamedOut.Closed()).To(BeTrue())
				})
			})

			Context("when streaming the file out fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeArtifactSource.StreamFileReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})
		})

		Context("when the file's artifact source cannot be found in the repository", func() {
			It("returns an UnknownArtifactSourceError", func() {
				Expect(fetchErr).To(Equal(UnknownArtifactSourceError{"some"}))
			})
		})
	})

	Describe("MergedConfigSource", func() {
		var (
			fakeConfigSourceA *fakes.FakeTaskConfigSource
			fakeConfigSourceB *fakes.FakeTaskConfigSource

			configSource TaskConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			fakeConfigSourceA = new(fakes.FakeTaskConfigSource)
			fakeConfigSourceB = new(fakes.FakeTaskConfigSource)

			configSource = MergedConfigSource{
				A: fakeConfigSourceA,
				B: fakeConfigSourceB,
			}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when fetching via A succeeds", func() {
			var configA = atc.TaskConfig{
				Image:  "some-image",
				Params: map[string]string{"PARAM": "A"},
			}

			BeforeEach(func() {
				fakeConfigSourceA.FetchConfigReturns(configA, nil)
			})

			Context("and fetching via B succeeds", func() {
				var configB = atc.TaskConfig{
					Params: map[string]string{"PARAM": "B"},
				}

				BeforeEach(func() {
					fakeConfigSourceB.FetchConfigReturns(configB, nil)
				})

				It("fetches via the input source", func() {
					Expect(fakeConfigSourceA.FetchConfigArgsForCall(0)).To(Equal(repo))
					Expect(fakeConfigSourceB.FetchConfigArgsForCall(0)).To(Equal(repo))
				})

				It("succeeds", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("returns the merged config", func() {
					Expect(fetchedConfig).To(Equal(atc.TaskConfig{
						Image:  "some-image",
						Params: map[string]string{"PARAM": "B"},
					}))

				})
			})

			Context("and fetching via B fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeConfigSourceB.FetchConfigReturns(atc.TaskConfig{}, disaster)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(Equal(disaster))
				})
			})
		})

		Context("when fetching via A fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeConfigSourceA.FetchConfigReturns(atc.TaskConfig{}, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})

			It("does not fetch via B", func() {
				Expect(fakeConfigSourceB.FetchConfigCallCount()).To(Equal(0))
			})
		})
	})
})
