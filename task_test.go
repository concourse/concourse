package atc_test

import (
	"strings"

	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskConfig", func() {
	Describe("validating", func() {
		var (
			invalidConfig TaskConfig
			validConfig   TaskConfig
		)

		BeforeEach(func() {
			validConfig = TaskConfig{
				Platform: "linux",
				Run: TaskRunConfig{
					Path: "reboot",
				},
			}

			invalidConfig = validConfig
		})

		Describe("decode task yaml", func() {
			Context("given a valid task config", func() {
				It("works", func() {
					data := []byte(`
platform: beos

inputs: []

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(task.Platform).To(Equal("beos"))
					Expect(task.Run.Path).To(Equal("a/file"))
				})

				It("converts yaml booleans to strings in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: true

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("true"))
				})

				It("converts yaml ints to the correct string in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: 1059262

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("1059262"))
				})

				It("converts yaml floats to the correct string in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: 1059262.123123123

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("1059262.123123123"))
				})

				It("converts maps to json in params", func() {
					data := []byte(`
platform: beos

params:
  testParam:
    foo: bar

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal(`{"foo":"bar"}`))
				})
			})

			Context("given a valid task config with numeric params", func() {
				It("works", func() {
					data := []byte(`
platform: beos

params:
  FOO: 1

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(task.Platform).To(Equal("beos"))
					Expect(task.Params).To(Equal(map[string]string{"FOO": "1"}))
				})
			})

			Context("given a valid task config with extra keys", func() {
				It("returns an error", func() {
					data := []byte(`
platform: beos

intputs: []

run: {path: a/file}
`)
					_, err := NewTaskConfig(data)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("given an invalid task config", func() {
				It("errors on validation", func() {
					data := []byte(`
platform: beos

inputs: ['a/b/c']
outputs: ['a/b/c']

run: {path: a/file}
`)
					_, err := NewTaskConfig(data)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when platform is missing", func() {
			BeforeEach(func() {
				invalidConfig.Platform = ""
			})

			It("returns an error", func() {
				Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  missing 'platform'")))
			})
		})

		Context("when the task has inputs", func() {
			BeforeEach(func() {
				validConfig.Inputs = append(validConfig.Inputs, TaskInputConfig{Name: "concourse"})
			})

			It("is valid", func() {
				Expect(validConfig.Validate()).ToNot(HaveOccurred())
			})

			Context("when input.name is missing", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(invalidConfig.Inputs, TaskInputConfig{Name: "concourse"}, TaskInputConfig{Name: ""})
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  input in position 1 is missing a name")))
				})
			})

			Context("when input.name is missing multiple times", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse"},
						TaskInputConfig{Name: ""},
						TaskInputConfig{Name: ""},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  input in position 1 is missing a name")))
					Expect(err).To(MatchError(ContainSubstring("  input in position 2 is missing a name")))
				})
			})
		})

		Context("when the task has outputs", func() {
			BeforeEach(func() {
				validConfig.Outputs = append(validConfig.Outputs, TaskOutputConfig{Name: "concourse"})
			})

			It("is valid", func() {
				Expect(validConfig.Validate()).ToNot(HaveOccurred())
			})

			Context("when output.name is missing", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(invalidConfig.Outputs, TaskOutputConfig{Name: "concourse"}, TaskOutputConfig{Name: ""})
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  output in position 1 is missing a name")))
				})
			})

			Context("when output.name is missing multiple times", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse"},
						TaskOutputConfig{Name: ""},
						TaskOutputConfig{Name: ""},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  output in position 1 is missing a name")))
					Expect(err).To(MatchError(ContainSubstring("  output in position 2 is missing a name")))
				})
			})
		})

		Context("when run is missing", func() {
			BeforeEach(func() {
				invalidConfig.Run.Path = ""
			})

			It("returns an error", func() {
				Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  missing path to executable to run")))
			})
		})

		Describe("input overlapping checks", func() {
			Context("when two inputs have the same name", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse"},
						TaskInputConfig{Name: "concourse"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have more than one input using the same path 'concourse'")))
				})
			})

			Context("when two inputs have the same path but not the same name", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse"},
						TaskInputConfig{Name: "garden", Path: "concourse"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have more than one input using the same path 'concourse'")))
				})
			})

			Context("when two inputs have the same path", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse", Path: "path"},
						TaskInputConfig{Name: "garden", Path: "path"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have more than one input using the same path 'path'")))
				})
			})
		})

		Describe("output overlapping checks", func() {
			Context("when two outputs have the same name", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse"},
						TaskOutputConfig{Name: "concourse"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have more than one output using the same path 'concourse'")))
				})
			})

			Context("when two outputs have the same path but not the same name", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse"},
						TaskOutputConfig{Name: "garden", Path: "concourse"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have more than one output using the same path 'concourse'")))
				})
			})

			Context("when two outputs have the same path", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse", Path: "path"},
						TaskOutputConfig{Name: "garden", Path: "path"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have more than one output using the same path 'path'")))
				})
			})
		})

		Describe("inputs and output overlapping checks", func() {
			Context("when an input has the same name as an output", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "garden"},
						TaskInputConfig{Name: "concourse"},
					)

					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have an input and output using the same path 'concourse'")))
				})
			})

			Context("when an inputs and an output have the same path but not the same name", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "garden"},
						TaskInputConfig{Name: "concourse"},
					)

					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "foo", Path: "garden"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have an input and output using the same path 'garden'")))
				})
			})

			Context("when an input and an output have the same path", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse", Path: "path"},
					)

					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "garden", Path: "path"},
					)
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  cannot have an input and output using the same path 'path'")))
				})
			})

			Context("when an input and an output have multiple conflicts", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "jettison"},
						TaskInputConfig{Name: "concourse", Path: "path"},
					)

					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "jettison"},
						TaskOutputConfig{Name: "jettison"},
						TaskOutputConfig{Name: "garden", Path: "path"},
						TaskOutputConfig{Name: "testflight", Path: "path"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot have an input and output using the same path 'jettison'")))
					Expect(err).To(MatchError(ContainSubstring("  cannot have more than one output using the same path 'jettison'")))
					Expect(err).To(MatchError(ContainSubstring("  cannot have more than one output using the same path 'path'")))
					Expect(err).To(MatchError(ContainSubstring("  cannot have an input and output using the same path 'path'")))

					lines := strings.Split(err.Error(), "\n")
					Expect(lines).To(HaveLen(5)) // 4 errors + header
				})
			})

			Context("when two inputs have a path conflict due to 1 path belonging to the subpath of the other", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "jettison", Path: "foo"},
						TaskInputConfig{Name: "concourse", Path: "foo/bar"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot nest inputs: 'foo/bar' is nested under input directory 'foo'")))
				})
			})

			Context("when two inputs have the same starting path but are not in the same directory", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "jettison", Path: "foo"},
						TaskInputConfig{Name: "concourse", Path: "foo-bar"},
					)
				})

				It("is not an error", func() {
					err := invalidConfig.Validate()

					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when two inputs have a name conflict due to 1 path belonging to the subpath of the other", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "foo/bar"},
						TaskInputConfig{Name: "foo"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot nest inputs: 'foo/bar' is nested under input directory 'foo'")))
				})
			})

			Context("when two outputs have a path conflict due to 1 path belonging to the subpath of the other", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "jettison", Path: "foo"},
						TaskOutputConfig{Name: "concourse", Path: "foo/bar"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot nest outputs: 'foo/bar' is nested under output directory 'foo'")))
				})
			})

			Context("when two outputs have the same starting path but are not in the same directory", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "jettison", Path: "foo"},
						TaskOutputConfig{Name: "concourse", Path: "foo-bar"},
					)
				})

				It("is not an error", func() {
					err := invalidConfig.Validate()

					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when two outputs have a name conflict due to 1 path belonging to the subpath of the other", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "foo/bar"},
						TaskOutputConfig{Name: "foo"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot nest outputs: 'foo/bar' is nested under output directory 'foo'")))
				})
			})

			Context("when two outputs have a path conflict due to 1 path belonging to the subpath of the other", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "jettison", Path: "foo1"},
						TaskOutputConfig{Name: "concourse", Path: "foo1/bar"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot nest outputs: 'foo1/bar' is nested under output directory 'foo1'")))
				})
			})

			Context("when an input path starts with ./", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "jettison", Path: "foo1"},
						TaskInputConfig{Name: "concourse", Path: "./foo2/bar"},
					)

					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "jettison", Path: "foo2"},
						TaskOutputConfig{Name: "concourse", Path: "./foo1/bar"},
					)
				})

				It("doesn't care and still returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  cannot nest outputs within inputs: 'foo1/bar' is nested under input directory 'foo1'")))
					Expect(err).To(MatchError(ContainSubstring("  cannot nest inputs within outputs: 'foo2/bar' is nested under output directory 'foo2'")))
				})
			})

			Context("when an input and output have the same starting path but are not in the same directory", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "jettison", Path: "foo"},
					)

					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse", Path: "foo-bar"},
					)
				})

				It("is not an error", func() {
					err := invalidConfig.Validate()

					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when there is only one input and it has path of '.'", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse", Path: "."},
					)
				})

				It("doesn't care and still returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when there is more than one input and one of them has path of '.'", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse", Path: "."},
						TaskInputConfig{Name: "testflight"},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("  you may not have more than one input or output when one of them has a path of '.'")))
				})
			})
		})
	})

	Describe("merging", func() {
		It("merges params while preserving other properties", func() {
			Expect(TaskConfig{
				RootfsURI: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
			}.Merge(TaskConfig{
				Params: map[string]string{
					"FOO": "3",
					"BAZ": "4",
				},
			})).To(

				Equal(TaskConfig{
					RootfsURI: "some-image",
					Params: map[string]string{
						"FOO": "3",
						"BAR": "2",
						"BAZ": "4",
					},
				}))

		})

		It("overrides the platform", func() {
			Expect(TaskConfig{
				Platform: "platform-a",
			}.Merge(TaskConfig{
				Platform: "platform-b",
			})).To(

				Equal(TaskConfig{
					Platform: "platform-b",
				}))

		})

		It("overrides the image", func() {
			Expect(TaskConfig{
				RootfsURI: "some-image",
			}.Merge(TaskConfig{
				RootfsURI: "better-image",
			})).To(

				Equal(TaskConfig{
					RootfsURI: "better-image",
				}))

		})

		It("overrides the run config", func() {
			Expect(TaskConfig{
				Run: TaskRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(TaskConfig{
				RootfsURI: "some-image",
				Run: TaskRunConfig{
					Path: "better-path",
					Args: []string{"better-arg1", "better-arg2"},
				},
			})).To(

				Equal(TaskConfig{
					RootfsURI: "some-image",
					Run: TaskRunConfig{
						Path: "better-path",
						Args: []string{"better-arg1", "better-arg2"},
					},
				}))

		})

		It("overrides the run config even with no args", func() {
			Expect(TaskConfig{
				Run: TaskRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(TaskConfig{
				RootfsURI: "some-image",
				Run: TaskRunConfig{
					Path: "better-path",
				},
			})).To(

				Equal(TaskConfig{
					RootfsURI: "some-image",
					Run: TaskRunConfig{
						Path: "better-path",
					},
				}))

		})

		It("overrides input configuration", func() {
			Expect(TaskConfig{
				Inputs: []TaskInputConfig{
					{Name: "some-input", Path: "some-destination"},
				},
			}.Merge(TaskConfig{
				Inputs: []TaskInputConfig{
					{Name: "another-input", Path: "another-destination"},
				},
			})).To(

				Equal(TaskConfig{
					Inputs: []TaskInputConfig{
						{Name: "another-input", Path: "another-destination"},
					},
				}))

		})
	})
})
