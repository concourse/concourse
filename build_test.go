package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildConfig", func() {
	Describe("merging", func() {
		It("merges params while preserving other properties", func() {
			Ω(BuildConfig{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
			}.Merge(BuildConfig{
				Params: map[string]string{
					"FOO": "3",
					"BAZ": "4",
				},
			})).Should(Equal(BuildConfig{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "3",
					"BAR": "2",
					"BAZ": "4",
				},
			}))
		})

		It("overrides the image", func() {
			Ω(BuildConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(BuildConfig{
				Image: "better-image",
			})).Should(Equal(BuildConfig{
				Image: "better-image",
				Run: BuildRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}))
		})

		It("overrides the run config", func() {
			Ω(BuildConfig{
				Run: BuildRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(BuildConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
					Args: []string{"better-arg1", "better-arg2"},
				},
			})).Should(Equal(BuildConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
					Args: []string{"better-arg1", "better-arg2"},
				},
			}))
		})

		It("overrides the run config even with no args", func() {
			Ω(BuildConfig{
				Run: BuildRunConfig{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				},
			}.Merge(BuildConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
				},
			})).Should(Equal(BuildConfig{
				Image: "some-image",
				Run: BuildRunConfig{
					Path: "better-path",
				},
			}))
		})

		It("overrides input configuration", func() {
			Ω(BuildConfig{
				Inputs: []BuildInputConfig{
					{Name: "some-input", Path: "some-destination"},
				},
			}.Merge(BuildConfig{
				Inputs: []BuildInputConfig{
					{Name: "another-input", Path: "another-destination"},
				},
			})).Should(Equal(BuildConfig{
				Inputs: []BuildInputConfig{
					{Name: "another-input", Path: "another-destination"},
				},
			}))
		})
	})
})
