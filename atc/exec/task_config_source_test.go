package exec_test

import (
	"errors"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("TaskConfigSource", func() {
	var (
		taskConfig atc.TaskConfig
		taskPlan   atc.TaskPlan
		repo       *worker.ArtifactRepository
	)

	BeforeEach(func() {
		repo = worker.NewArtifactRepository()
		taskConfig = atc.TaskConfig{
			Platform:  "some-platform",
			RootfsURI: "some-image",
			ImageResource: &atc.ImageResource{
				Type:    "docker",
				Source:  atc.Source{"a": "b"},
				Params:  &atc.Params{"some": "params"},
				Version: &atc.Version{"some": "version"},
			},
			Params: map[string]string{
				"task-config-param-key": "task-config-param-val-1",
				"common-key":            "task-config-param-val-2",
			},
			Run: atc.TaskRunConfig{
				Path: "ls",
				Args: []string{"-al"},
				Dir:  "some/dir",
				User: "some-user",
			},
			Inputs: []atc.TaskInputConfig{
				{Name: "some-input", Path: "some-path"},
			},
		}

		taskPlan = atc.TaskPlan{
			Params: atc.Params{
				"task-plan-param-key": "task-plan-param-val-1",
				"common-key":          "task-plan-param-val-2",
			},
			Config: &taskConfig,
		}
	})

	Describe("StaticConfigSource", func() {
		var (
			configSource TaskConfigSource
		)

		JustBeforeEach(func() {
			configSource = StaticConfigSource{Plan: taskPlan}
		})

		Context("when the params contain a floating point value", func() {
			BeforeEach(func() {
				taskPlan.Params["int-val"] = float64(1059262)
				taskPlan.Params["float-val"] = float64(1059262.987345987)
			})

			It("does the right thing", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(HaveKeyWithValue("int-val", "1059262"))
				Expect(fetchedConfig.Params).To(HaveKeyWithValue("float-val", "1059262.987345987"))
			})
		})

		It("merges task params prefering params in task plan", func() {
			fetchedConfig, err := configSource.FetchConfig(repo)
			Expect(err).ToNot(HaveOccurred())
			Expect(fetchedConfig.Params).To(Equal(map[string]string{
				"task-plan-param-key":   "task-plan-param-val-1",
				"task-config-param-key": "task-config-param-val-1",
				"common-key":            "task-plan-param-val-2",
			}))
		})

		Context("when task config params are not set", func() {
			BeforeEach(func() {
				taskConfig = atc.TaskConfig{}
			})

			It("uses params from task plan", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(Equal(map[string]string{
					"task-plan-param-key": "task-plan-param-val-1",
					"common-key":          "task-plan-param-val-2",
				}))
			})
		})

		Context("when task plan params are not set", func() {
			BeforeEach(func() {
				taskPlan = atc.TaskPlan{
					Config: &taskConfig,
				}
			})

			It("uses params from task config", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(Equal(map[string]string{
					"task-config-param-key": "task-config-param-val-1",
					"common-key":            "task-config-param-val-2",
				}))
			})
		})

		Context("when the plan has no task config", func() {
			BeforeEach(func() {
				taskPlan.Config = nil
			})

			Context("when plan has params", func() {
				It("returns an config with plan params", func() {
					fetchedConfig, err := configSource.FetchConfig(repo)
					Expect(err).ToNot(HaveOccurred())
					Expect(fetchedConfig).To(Equal(atc.TaskConfig{
						Params: map[string]string{
							"task-plan-param-key": "task-plan-param-val-1",
							"common-key":          "task-plan-param-val-2",
						},
					}))
				})
			})

			Context("when plan does not have params", func() {
				BeforeEach(func() {
					taskPlan.Params = nil
				})

				It("returns an empty config", func() {
					fetchedConfig, err := configSource.FetchConfig(repo)
					Expect(err).ToNot(HaveOccurred())
					Expect(fetchedConfig).To(Equal(atc.TaskConfig{}))
				})
			})
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
			var fakeArtifactSource *workerfakes.FakeArtifactSource

			BeforeEach(func() {
				fakeArtifactSource = new(workerfakes.FakeArtifactSource)
				repo.RegisterSource("some", fakeArtifactSource)
			})

			Context("when the artifact source provides a proper file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					marshalled, err := yaml.Marshal(taskConfig)
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
					Expect(fetchedConfig).To(Equal(taskConfig))
				})

				It("closes the stream", func() {
					Expect(streamedOut.Closed()).To(BeTrue())
				})
			})

			Context("when the artifact source provides an invalid configuration", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					invalidConfig := taskConfig
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

			Context("when the artifact source provides a valid file with invalid keys", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					streamedOut = gbytes.BufferWithBytes([]byte(`
platform: beos

intputs: []

run: {path: a/file}
`))
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

			Context("when the file task is not found", func() {
				BeforeEach(func() {
					fakeArtifactSource.StreamFileReturns(nil, baggageclaim.ErrFileNotFound)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(HaveOccurred())
					Expect(fetchErr.Error()).To(Equal("task config 'some/build.yml' not found"))
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
			fakeConfigSourceA *execfakes.FakeTaskConfigSource
			fakeConfigSourceB *execfakes.FakeTaskConfigSource

			configSource TaskConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error

			configA atc.TaskConfig
			configB atc.TaskConfig
		)

		BeforeEach(func() {
			fakeConfigSourceA = new(execfakes.FakeTaskConfigSource)
			fakeConfigSourceB = new(execfakes.FakeTaskConfigSource)

			configSource = &MergedConfigSource{
				A: fakeConfigSourceA,
				B: fakeConfigSourceB,
			}

			configA = atc.TaskConfig{
				Platform:  "some-platform",
				RootfsURI: "some-image",
				Params:    map[string]string{"PARAM": "A"},
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}
			configB = atc.TaskConfig{
				Params: map[string]string{"PARAM": "B"},
			}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when fetching via A succeeds", func() {
			BeforeEach(func() {
				fakeConfigSourceA.FetchConfigReturns(configA, nil)
			})

			Context("and fetching via B succeeds", func() {
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
						Platform:  "some-platform",
						RootfsURI: "some-image",
						Params:    map[string]string{"PARAM": "B"},
						Run: atc.TaskRunConfig{
							Path: "echo",
							Args: []string{"bananapants"},
						},
					}))
				})

				Context("but B defines a param not in A", func() {
					BeforeEach(func() {
						configB.Params["EXTRA_PARAM"] = "EXTRA_PARAM isn't defined in task file"
					})

					It("should have a warning", func() {
						Expect(configSource.Warnings()).To(HaveLen(1))
						Expect(configSource.Warnings()[0]).To(ContainSubstring("EXTRA_PARAM was defined in pipeline but missing from task file"))
					})

					It("should not fail", func() {
						Expect(fetchErr).ToNot(HaveOccurred())
					})
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

	Describe("ValidatingConfigSource", func() {
		var (
			fakeConfigSource *execfakes.FakeTaskConfigSource

			configSource TaskConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			fakeConfigSource = new(execfakes.FakeTaskConfigSource)

			configSource = ValidatingConfigSource{fakeConfigSource}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when the config is valid", func() {
			config := atc.TaskConfig{
				Platform:  "some-platform",
				RootfsURI: "some-image",
				Params:    map[string]string{"PARAM": "A"},
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}

			BeforeEach(func() {
				fakeConfigSource.FetchConfigReturns(config, nil)
			})

			It("returns the config and no error", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetchedConfig).To(Equal(config))
			})
		})

		Context("when the config is invalid", func() {
			BeforeEach(func() {
				fakeConfigSource.FetchConfigReturns(atc.TaskConfig{
					RootfsURI: "some-image",
					Params:    map[string]string{"PARAM": "A"},
					Run: atc.TaskRunConfig{
						Args: []string{"bananapants"},
					},
				}, nil)
			})

			It("returns the validation error", func() {
				Expect(fetchErr).To(HaveOccurred())
			})
		})

		Context("when fetching the config fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeConfigSource.FetchConfigReturns(atc.TaskConfig{}, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})
		})
	})
})
