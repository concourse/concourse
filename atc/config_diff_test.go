package atc_test

import (
	. "github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Config diff", func() {
	Describe("job config", func() {
		var job JobConfig
		BeforeEach(func() {
			job = JobConfig{
				Name: "some-job",
				PlanSequence: []Step{
					Step{
						Config: &GetStep{
							Name:     "some-name",
							Resource: "some-resource",
							Trigger:  true,
						},
					},
				},
			}
		})

		Context("when there are no jobs", func() {
			It("does not print anything about jobs", func() {
				buffer := NewBuffer()
				diff := Config{}.Diff(buffer, Config{})
				Expect(diff).To(BeFalse())
				Consistently(buffer).ShouldNot(Say("jobs"))
			})
		})

		Context("when a job is added", func() {
			It("says config has been added", func() {
				buffer := NewBuffer()
				newConfig := Config{
					Jobs: []JobConfig{job},
				}
				diff := Config{}.Diff(buffer, newConfig)
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("job some-job has been added:"))
				Eventually(buffer).Should(Say(`\+.*name: some-job`))
				Eventually(buffer).Should(Say(`\+.*plan:`))
				Eventually(buffer).Should(Say(`\+.*- get: some-name`))
				Eventually(buffer).Should(Say(`\+.*  resource: some-resource`))
				Eventually(buffer).Should(Say(`\+.*  trigger: true`))
			})
		})

		Context("when display config is removed", func() {
			It("says config has been removed", func() {
				buffer := NewBuffer()
				oldConfig := Config{
					Jobs: []JobConfig{job},
				}
				diff := oldConfig.Diff(buffer, Config{})
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("job some-job has been removed:"))
				Eventually(buffer).Should(Say(`-.*name: some-job`))
				Eventually(buffer).Should(Say(`-.*plan:`))
				Eventually(buffer).Should(Say(`-.*- get: some-name`))
				Eventually(buffer).Should(Say(`-.*  resource: some-resource`))
				Eventually(buffer).Should(Say(`-.*  trigger: true`))
			})
		})

		Context("when there are no jobs to change", func() {
			It("says there are no changes to apply", func() {
				oldConfig := Config{
					Jobs: []JobConfig{job},
				}
				newConfig := Config{
					Jobs: []JobConfig{job},
				}

				diff := oldConfig.Diff(GinkgoWriter, newConfig)
				Expect(diff).To(BeFalse())
			})
		})

		Context("when a single field changes", func() {
			It("removes the field if it has a default value", func() {
				oldConfig := Config{
					Jobs: []JobConfig{job},
				}
				newJob := JobConfig{
					Name: "some-job",
					PlanSequence: []Step{
						Step{
							Config: &GetStep{
								Name:     "some-name",
								Resource: "some-resource",
								Trigger:  false,
							},
						},
					},
				}
				newConfig := Config{
					Jobs: []JobConfig{newJob},
				}

				buffer := NewBuffer()
				diff := oldConfig.Diff(buffer, newConfig)
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("job some-job has changed:"))
				Eventually(buffer).Should(Say(`-.* trigger: true`))
			})

			It("replaces the field if it does not have a default value", func() {
				oldConfig := Config{
					Jobs: []JobConfig{job},
				}
				newJob := JobConfig{
					Name: "some-job",
					PlanSequence: []Step{
						Step{
							Config: &GetStep{
								Name:     "some-name",
								Resource: "some-other-resource",
								Trigger:  true,
							},
						},
					},
				}
				newConfig := Config{
					Jobs: []JobConfig{newJob},
				}

				buffer := NewBuffer()
				diff := oldConfig.Diff(buffer, newConfig)
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("job some-job has changed:"))
				Eventually(buffer).Should(Say(`-.* resource: some-resource`))
				Eventually(buffer).Should(Say(`\+.* resource: some-other-resource`))
			})
		})
	})

	Describe("display config", func() {
		var display DisplayConfig
		BeforeEach(func() {
			display = DisplayConfig{
				BackgroundImage: "some-background.jpg",
			}
		})
		Context("when there is no display config", func() {
			It("does not print anything about display config", func() {
				buffer := NewBuffer()
				diff := Config{}.Diff(buffer, Config{})
				Expect(diff).To(BeFalse())
				Consistently(buffer).ShouldNot(Say("display"))
			})
		})

		Context("when display config is added", func() {
			It("says config has been added", func() {
				buffer := NewBuffer()
				newConfig := Config{
					Display: &display,
				}
				diff := Config{}.Diff(buffer, newConfig)
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("display configuration has been added:"))
				Eventually(buffer).Should(Say(`\+.*background_image: some-background.jpg`))
			})
		})

		Context("when display config is removed", func() {
			It("says config has been removed", func() {
				buffer := NewBuffer()
				oldConfig := Config{
					Display: &display,
				}
				diff := oldConfig.Diff(buffer, Config{})
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("display configuration has been removed:"))
				Eventually(buffer).Should(Say(`-.*background_image: some-background.jpg`))
			})
		})

		Context("when there is no display config to change", func() {
			It("says there are no changes to apply", func() {
				oldConfig := Config{
					Display: &display,
				}
				newConfig := Config{
					Display: &display,
				}

				diff := oldConfig.Diff(GinkgoWriter, newConfig)
				Expect(diff).To(BeFalse())
			})
		})

		Context("when the background changes", func() {
			It("replaces the background", func() {
				oldConfig := Config{
					Display: &display,
				}
				newConfig := Config{
					Display: &DisplayConfig{
						BackgroundImage: "some-other-background.jpg",
					},
				}

				buffer := NewBuffer()
				diff := oldConfig.Diff(buffer, newConfig)
				Expect(diff).To(BeTrue())
				Eventually(buffer).Should(Say("display configuration has changed:"))
				Eventually(buffer).Should(Say("-.*background_image: some-background.jpg"))
				Eventually(buffer).Should(Say(`\+.*background_image: some-other-background.jpg`))
			})
		})
	})
})
