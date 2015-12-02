package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskConfig", func() {
	Describe("validating", func() {
		validConfig := TaskConfig{
			Platform: "linux",
			Run: TaskRunConfig{
				Path: "reboot",
			},
		}

		var invalidConfig TaskConfig

		BeforeEach(func() {
			invalidConfig = validConfig
		})

		Context("when platform is missing", func() {
			BeforeEach(func() {
				invalidConfig.Platform = ""
			})

			It("returns an error", func() {
				Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("missing 'platform'")))
			})
		})

		Context("when run is missing", func() {
			BeforeEach(func() {
				invalidConfig.Run.Path = ""
			})

			It("returns an error", func() {
				Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("missing path to executable to run")))
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
