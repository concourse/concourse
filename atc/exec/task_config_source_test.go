package exec_test

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"sigs.k8s.io/yaml"
)

var _ = Describe("TaskConfigSource", func() {
	var (
		taskConfig atc.TaskConfig
		taskVars   atc.Params
		repo       *build.Repository
		logger     *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("task-config-source-test")
		repo = build.NewRepository()
		taskConfig = atc.TaskConfig{
			Platform:  "some-platform",
			RootfsURI: "some-image",
			ImageResource: &atc.ImageResource{
				Type: "docker",
				Source: atc.Source{
					"a":               "b",
					"evaluated-value": "((task-variable-name))",
				},
				Params: atc.Params{
					"some":            "params",
					"evaluated-value": "((task-variable-name))",
				},
				Version: atc.Version{"some": "version"},
			},
			Params: atc.TaskEnv{
				"key1": "key1-((task-variable-name))",
				"key2": "key2-((task-variable-name))",
			},
			Run: atc.TaskRunConfig{
				Path: "ls",
				Args: []string{"-al", "((task-variable-name))"},
				Dir:  "some/dir",
				User: "some-user",
			},
			Inputs: []atc.TaskInputConfig{
				{Name: "some-input", Path: "some-path"},
			},
		}
		taskVars = atc.Params{
			"task-variable-name": "task-variable-value",
		}
	})

	Describe("StaticConfigSource", func() {

		It("fetches task config successfully", func() {
			configSource := StaticConfigSource{Config: &taskConfig}
			fetchedConfig, fetchErr := configSource.FetchConfig(context.TODO(), logger, repo)
			Expect(fetchErr).ToNot(HaveOccurred())
			Expect(fetchedConfig).To(Equal(taskConfig))
		})

		It("fetches config of nil task successfully", func() {
			configSource := StaticConfigSource{Config: nil}
			fetchedConfig, fetchErr := configSource.FetchConfig(context.TODO(), logger, repo)
			Expect(fetchErr).ToNot(HaveOccurred())
			Expect(fetchedConfig).To(Equal(atc.TaskConfig{}))
		})
	})

	Describe("FileConfigSource", func() {
		var (
			configSource         FileConfigSource
			fakeArtifactStreamer *workerfakes.FakeArtifactStreamer
			fetchErr             error
			artifactName         string
		)

		BeforeEach(func() {

			artifactName = "some-artifact-name"
			fakeArtifactStreamer = new(workerfakes.FakeArtifactStreamer)
			configSource = FileConfigSource{
				ConfigPath: artifactName + "/build.yml",
				Streamer:   fakeArtifactStreamer,
			}
		})

		JustBeforeEach(func() {
			_, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
		})

		Context("when the path does not indicate an artifact source", func() {
			BeforeEach(func() {
				configSource.ConfigPath = "foo-bar.yml"
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(UnspecifiedArtifactSourceError{"foo-bar.yml"}))
			})
		})

		Context("when the file's artifact can be found in the repository", func() {
			var fakeArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				fakeArtifact = new(runtimefakes.FakeArtifact)
				repo.RegisterArtifact(build.ArtifactName(artifactName), fakeArtifact)
			})

			Context("when the artifact provides a proper file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					marshalled, err := yaml.Marshal(taskConfig)
					Expect(err).NotTo(HaveOccurred())

					streamedOut = gbytes.BufferWithBytes(marshalled)
					fakeArtifactStreamer.StreamFileFromArtifactReturns(streamedOut, nil)
				})

				It("fetches the file via the correct artifact & path", func() {
					_, artifact, dest := fakeArtifactStreamer.StreamFileFromArtifactArgsForCall(0)
					Expect(artifact).To(Equal(fakeArtifact))
					Expect(dest).To(Equal("build.yml"))
				})

				It("succeeds", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
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
					fakeArtifactStreamer.StreamFileFromArtifactReturns(streamedOut, nil)
				})

				It("returns an error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})

			Context("when the artifact source provides a malformed file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					streamedOut = gbytes.BufferWithBytes([]byte("bogus"))
					fakeArtifactStreamer.StreamFileFromArtifactReturns(streamedOut, nil)
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
					fakeArtifactStreamer.StreamFileFromArtifactReturns(streamedOut, nil)
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
					fakeArtifactStreamer.StreamFileFromArtifactReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})

			Context("when the file task is not found", func() {
				BeforeEach(func() {
					fakeArtifactStreamer.StreamFileFromArtifactReturns(nil, baggageclaim.ErrFileNotFound)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(HaveOccurred())
					Expect(fetchErr.Error()).To(Equal(fmt.Sprintf("task config '%s/build.yml' not found", artifactName)))
				})
			})
		})

		Context("when the file's artifact source cannot be found in the repository", func() {
			It("returns an UnknownArtifactSourceError", func() {
				Expect(fetchErr).To(Equal(UnknownArtifactSourceError{SourceName: build.ArtifactName(artifactName), ConfigPath: artifactName + "/build.yml"}))
			})
		})
	})

	Describe("OverrideParamsConfigSource", func() {
		var (
			config       atc.TaskConfig
			configSource TaskConfigSource

			overrideParams atc.TaskEnv

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			config = atc.TaskConfig{
				Platform:  "some-platform",
				RootfsURI: "some-image",
				Params:    atc.TaskEnv{"PARAM": "A", "ORIG_PARAM": "D"},
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}

			overrideParams = atc.TaskEnv{"PARAM": "B", "EXTRA_PARAM": "C"}
		})

		Context("when there are no params to override", func() {
			BeforeEach(func() {
				configSource = &OverrideParamsConfigSource{
					ConfigSource: StaticConfigSource{Config: &config},
				}
			})

			JustBeforeEach(func() {
				fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
			})

			It("succeeds", func() {
				Expect(fetchErr).NotTo(HaveOccurred())
			})

			It("returns the same config", func() {
				Expect(fetchedConfig).To(Equal(config))
			})

			It("returns no warnings", func() {
				Expect(configSource.Warnings()).To(HaveLen(0))
			})
		})

		Context("when override params are specified", func() {
			BeforeEach(func() {
				configSource = &OverrideParamsConfigSource{
					ConfigSource: StaticConfigSource{Config: &config},
					Params:       overrideParams,
				}
			})

			JustBeforeEach(func() {
				fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
			})

			It("succeeds", func() {
				Expect(fetchErr).NotTo(HaveOccurred())
			})

			It("returns the config with overridden parameters", func() {
				Expect(fetchedConfig.Params).To(Equal(atc.TaskEnv{
					"ORIG_PARAM":  "D",
					"PARAM":       "B",
					"EXTRA_PARAM": "C",
				}))
			})

			It("returns a deprecation warning", func() {
				Expect(configSource.Warnings()).To(HaveLen(1))
				Expect(configSource.Warnings()[0]).To(ContainSubstring("EXTRA_PARAM was defined in pipeline but missing from task file"))
			})
		})
	})

	Describe("OverrideContainerLimitsSource", func() {
		var (
			config         atc.TaskConfig
			noLimitsConfig atc.TaskConfig
			configSource   TaskConfigSource

			overrideLimits atc.ContainerLimits

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			config = atc.TaskConfig{
				Platform:  "some-platform",
				RootfsURI: "some-image",
				Limits:    &atc.ContainerLimits{CPU: newCPULimit(1024), Memory: newMemoryLimit(209715200)},
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}

			noLimitsConfig = atc.TaskConfig{
				Platform:  "some-platform",
				RootfsURI: "some-image",
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}

			overrideLimits = atc.ContainerLimits{CPU: newCPULimit(2048), Memory: newMemoryLimit(209715200)}
		})

		Context("when there are no container limits to override", func() {
			BeforeEach(func() {
				configSource = &OverrideContainerLimitsSource{
					ConfigSource: StaticConfigSource{Config: &config},
				}
			})

			JustBeforeEach(func() {
				fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
			})

			It("succeeds", func() {
				Expect(fetchErr).NotTo(HaveOccurred())
			})

			It("returns the same config", func() {
				Expect(fetchedConfig).To(Equal(config))
			})
		})

		Context("when override container limits are specified but no configuration container limits", func() {
			BeforeEach(func() {
				configSource = &OverrideContainerLimitsSource{
					ConfigSource: StaticConfigSource{Config: &noLimitsConfig},
					Limits:       &overrideLimits,
				}
			})

			JustBeforeEach(func() {
				fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
			})

			It("succeeds", func() {
				Expect(fetchErr).NotTo(HaveOccurred())
			})

			It("returns the config with overridden limits", func() {
				Expect(fetchedConfig).NotTo(BeNil())
				Expect(*fetchedConfig.Limits).To(Equal(atc.ContainerLimits{
					CPU:    newCPULimit(2048),
					Memory: newMemoryLimit(209715200),
				}))
			})
		})

		Context("when override container limits are specified", func() {
			BeforeEach(func() {
				configSource = &OverrideContainerLimitsSource{
					ConfigSource: StaticConfigSource{Config: &config},
					Limits:       &overrideLimits,
				}
			})

			JustBeforeEach(func() {
				fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
			})

			It("succeeds", func() {
				Expect(fetchErr).NotTo(HaveOccurred())
			})

			It("returns the config with overridden limits", func() {
				Expect(fetchedConfig).NotTo(BeNil())
				Expect(*fetchedConfig.Limits).To(Equal(atc.ContainerLimits{
					CPU:    newCPULimit(2048),
					Memory: newMemoryLimit(209715200),
				}))
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
			fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
		})

		Context("when the config is valid", func() {
			config := atc.TaskConfig{
				Platform:  "some-platform",
				RootfsURI: "some-image",
				Params:    atc.TaskEnv{"PARAM": "A"},
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
					Params:    atc.TaskEnv{"PARAM": "A"},
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

	Describe("InterpolateTemplateConfigSource", func() {
		var (
			configSource  TaskConfigSource
			fetchedConfig atc.TaskConfig
			fetchErr      error
			expectAllKeys bool
		)

		JustBeforeEach(func() {
			configSource = StaticConfigSource{Config: &taskConfig}
			configSource = InterpolateTemplateConfigSource{
				ConfigSource:  configSource,
				Vars:          []vars.Variables{vars.StaticVariables(taskVars)},
				ExpectAllKeys: expectAllKeys,
			}
			fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
		})

		Context("when expect all keys", func() {
			BeforeEach(func() {
				expectAllKeys = true
			})

			It("fetches task config successfully", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
			})

			It("resolves task config parameters successfully", func() {
				Expect(fetchedConfig.Run.Args).To(Equal([]string{"-al", "task-variable-value"}))
				Expect(fetchedConfig.Params).To(Equal(atc.TaskEnv{
					"key1": "key1-task-variable-value",
					"key2": "key2-task-variable-value",
				}))
				Expect(fetchedConfig.ImageResource.Source).To(Equal(atc.Source{
					"a":               "b",
					"evaluated-value": "task-variable-value",
				}))
			})
		})

		Context("when not expect all keys", func() {
			BeforeEach(func() {
				expectAllKeys = false
				taskVars = atc.Params{}
			})

			It("fetches task config successfully", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
			})

			It("resolves task config parameters successfully", func() {
				Expect(fetchedConfig.Run.Args).To(Equal([]string{"-al", "((task-variable-name))"}))
				Expect(fetchedConfig.Params).To(Equal(atc.TaskEnv{
					"key1": "key1-((task-variable-name))",
					"key2": "key2-((task-variable-name))",
				}))
				Expect(fetchedConfig.ImageResource.Source).To(Equal(atc.Source{
					"a":               "b",
					"evaluated-value": "((task-variable-name))",
				}))
			})
		})
	})

	Context("BaseResourceTypeDefaultsApplySource", func() {
		var (
			configSource  TaskConfigSource
			resourceTypes atc.VersionedResourceTypes
			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		JustBeforeEach(func() {
			configSource = StaticConfigSource{Config: &taskConfig}
			configSource = BaseResourceTypeDefaultsApplySource{
				ConfigSource:  configSource,
				ResourceTypes: resourceTypes,
			}
			fetchedConfig, fetchErr = configSource.FetchConfig(context.TODO(), logger, repo)
		})

		Context("resourceTypes is empty, and no base resource type defaults configured", func() {
			It("fetchedConfig should be identical to the original", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetchedConfig).To(Equal(taskConfig))
			})
		})

		Context("resourceTypes is empty, and base resource type defaults configured", func() {
			BeforeEach(func() {
				atc.LoadBaseResourceTypeDefaults(map[string]atc.Source{"docker": atc.Source{"some-key": "some-value"}})
			})
			AfterEach(func() {
				atc.LoadBaseResourceTypeDefaults(map[string]atc.Source{})
			})

			It("defaults should be added to image source", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetchedConfig.ImageResource.Source).To(Equal(atc.Source{
					"a":               "b",
					"evaluated-value": "((task-variable-name))",
					"some-key":        "some-value",
				}))
			})
		})

		Context("resourceTypes contains image source type", func() {
			BeforeEach(func() {
				resourceTypes = atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name:     "docker",
							Defaults: atc.Source{"some-key": "some-value"},
						},
					},
				}
			})

			It("defaults should be added to image source", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetchedConfig.ImageResource.Source).To(Equal(atc.Source{
					"a":               "b",
					"evaluated-value": "((task-variable-name))",
					"some-key":        "some-value",
				}))
			})
		})
	})
})

func newCPULimit(cpuLimit uint64) *atc.CPULimit {
	limit := atc.CPULimit(cpuLimit)
	return &limit
}

func newMemoryLimit(memoryLimit uint64) *atc.MemoryLimit {
	limit := atc.MemoryLimit(memoryLimit)
	return &limit
}
