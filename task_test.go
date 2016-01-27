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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'concourse'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'concourse'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'path'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'concourse'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'concourse'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'path'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'concourse'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'garden'")))
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
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'path'")))
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

					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'path'")))
					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'jettison'")))

					lines := strings.Split(err.Error(), "\n")
					Expect(lines).To(HaveLen(3)) // 2 errors + header
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

					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'foo'")))
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

					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'foo'")))
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

					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'foo'")))
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

					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'foo'")))
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

					Expect(err).To(MatchError(ContainSubstring("  inputs and outputs have overlapping path: 'foo1'")))
				})
			})
		})
	})

	Describe("merging", func() {
		It("merges params while preserving other properties", func() {
			Expect(TaskConfig{
				Image: "some-image",
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
					Image: "some-image",
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
				Image: "some-image",
			}.Merge(TaskConfig{
				Image: "better-image",
			})).To(

				Equal(TaskConfig{
					Image: "better-image",
				}))

		})

		It("overrides the run config", func() {
			Expect(TaskConfig{
				Run: TaskRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(TaskConfig{
				Image: "some-image",
				Run: TaskRunConfig{
					Path: "better-path",
					Args: []string{"better-arg1", "better-arg2"},
				},
			})).To(

				Equal(TaskConfig{
					Image: "some-image",
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
				Image: "some-image",
				Run: TaskRunConfig{
					Path: "better-path",
				},
			})).To(

				Equal(TaskConfig{
					Image: "some-image",
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
