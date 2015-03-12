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
			Run: BuildRunConfig{
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
				Ω(invalidConfig.Validate()).Should(MatchError(ContainSubstring("missing 'platform'")))
			})
		})

		Context("when run is missing", func() {
			BeforeEach(func() {
				invalidConfig.Run.Path = ""
			})

			It("returns an error", func() {
				Ω(invalidConfig.Validate()).Should(MatchError(ContainSubstring("missing path to executable to run")))
			})
		})
	})

	Describe("merging", func() {
		It("merges params while preserving other properties", func() {
			Ω(TaskConfig{
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
			})).Should(Equal(TaskConfig{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "3",
					"BAR": "2",
					"BAZ": "4",
				},
			}))
		})

		It("merges tags", func() {
			Ω(TaskConfig{
				Tags: []string{"a", "b"},
			}.Merge(TaskConfig{
				Tags: []string{"b", "c", "d"},
			}).Tags).Should(ConsistOf("a", "b", "c", "d"))
		})

		It("overrides the platform", func() {
			Ω(TaskConfig{
				Platform: "platform-a",
			}.Merge(TaskConfig{
				Platform: "platform-b",
			})).Should(Equal(TaskConfig{
				Platform: "platform-b",
			}))
		})

		It("overrides the image", func() {
			Ω(TaskConfig{
				Image: "some-image",
			}.Merge(TaskConfig{
				Image: "better-image",
			})).Should(Equal(TaskConfig{
				Image: "better-image",
			}))
		})

		It("overrides the run config", func() {
			Ω(TaskConfig{
				Run: BuildRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(TaskConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
					Args: []string{"better-arg1", "better-arg2"},
				},
			})).Should(Equal(TaskConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
					Args: []string{"better-arg1", "better-arg2"},
				},
			}))
		})

		It("overrides the run config even with no args", func() {
			Ω(TaskConfig{
				Run: BuildRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(TaskConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
				},
			})).Should(Equal(TaskConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
				},
			}))
		})

		It("overrides input configuration", func() {
			Ω(TaskConfig{
				Inputs: []BuildInputConfig{
					{Name: "some-input", Path: "some-destination"},
				},
			}.Merge(TaskConfig{
				Inputs: []BuildInputConfig{
					{Name: "another-input", Path: "another-destination"},
				},
			})).Should(Equal(TaskConfig{
				Inputs: []BuildInputConfig{
					{Name: "another-input", Path: "another-destination"},
				},
			}))
		})
	})
})
