package factory_test

import (
	tbuilds "github.com/concourse/turbine/api/builds"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory", func() {
	var (
		factory *BuildFactory

		job config.Job

		expectedTurbineBuild tbuilds.Build
	)

	BeforeEach(func() {
		factory = &BuildFactory{
			Resources: config.Resources{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: config.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-dependant-resource",
					Type:   "git",
					Source: config.Source{"uri": "git://some-dependant-resource"},
				},
				{
					Name:   "some-output-resource",
					Type:   "git",
					Source: config.Source{"uri": "git://some-output-resource"},
				},
				{
					Name:   "some-resource-with-longer-name",
					Type:   "git",
					Source: config.Source{"uri": "git://some-resource-with-longer-name"},
				},
				{
					Name:   "some-named-resource",
					Type:   "git",
					Source: config.Source{"uri": "git://some-named-resource"},
				},
			},
		}

		job = config.Job{
			Name: "some-job",

			BuildConfig: tbuilds.Config{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
				Run: tbuilds.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

			Privileged: true,

			BuildConfigPath: "some-resource/build.yml",

			Inputs: []config.Input{
				{
					Resource: "some-resource",
					Params:   config.Params{"some": "params"},
				},
			},

			Outputs: []config.Output{
				{
					Resource: "some-resource",
					Params:   config.Params{"foo": "bar"},
				},
				{
					Resource:  "some-resource",
					Params:    config.Params{"foo": "bar"},
					PerformOn: []config.OutputCondition{"failure"},
				},
				{
					Resource:  "some-resource",
					Params:    config.Params{"foo": "bar"},
					PerformOn: []config.OutputCondition{},
				},
			},
		}

		expectedTurbineBuild = tbuilds.Build{
			Config: tbuilds.Config{
				Image: "some-image",

				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},

				Run: tbuilds.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

			Inputs: []tbuilds.Input{
				{
					Name:       "some-resource",
					Resource:   "some-resource",
					Type:       "git",
					Source:     tbuilds.Source{"uri": "git://some-resource"},
					Params:     tbuilds.Params{"some": "params"},
					ConfigPath: "build.yml",
				},
			},

			Outputs: []tbuilds.Output{
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []tbuilds.OutputCondition{tbuilds.OutputConditionSuccess},
					Params: tbuilds.Params{"foo": "bar"},
					Source: tbuilds.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []tbuilds.OutputCondition{tbuilds.OutputConditionFailure},
					Params: tbuilds.Params{"foo": "bar"},
					Source: tbuilds.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []tbuilds.OutputCondition{},
					Params: tbuilds.Params{"foo": "bar"},
					Source: tbuilds.Source{"uri": "git://some-resource"},
				},
			},

			Privileged: true,
		}
	})

	It("creates a turbine build based on the job's configuration", func() {
		turbineBuild, err := factory.Create(job, nil)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(turbineBuild).Should(Equal(expectedTurbineBuild))
	})

	Context("when an input has an explicit name", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, config.Input{
				Name:     "some-named-input",
				Resource: "some-named-resource",
				Params:   config.Params{"some": "named-params"},
			})

			expectedTurbineBuild.Inputs = append(expectedTurbineBuild.Inputs, tbuilds.Input{
				Name:     "some-named-input",
				Resource: "some-named-resource",
				Type:     "git",
				Source:   tbuilds.Source{"uri": "git://some-named-resource"},
				Params:   tbuilds.Params{"some": "named-params"},
			})
		})

		It("uses it as the name for the input", func() {
			turbineBuild, err := factory.Create(job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineBuild.Inputs).Should(Equal(expectedTurbineBuild.Inputs))
		})
	})

	Context("when an explicitly named input is the source of the config", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, config.Input{
				Name:     "some-named-input",
				Resource: "some-named-resource",
				Params:   config.Params{"some": "named-params"},
			})

			job.BuildConfigPath = "some-named-input/build.yml"

			expectedTurbineBuild.Inputs[0].ConfigPath = ""

			expectedTurbineBuild.Inputs = append(expectedTurbineBuild.Inputs, tbuilds.Input{
				Name:       "some-named-input",
				Resource:   "some-named-resource",
				Type:       "git",
				Source:     tbuilds.Source{"uri": "git://some-named-resource"},
				Params:     tbuilds.Params{"some": "named-params"},
				ConfigPath: "build.yml",
			})
		})

		It("uses the explicit name to match the config path", func() {
			turbineBuild, err := factory.Create(job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineBuild.Inputs).Should(Equal(expectedTurbineBuild.Inputs))
		})
	})

	Context("when two inputs have overlappying names for the config path", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, config.Input{
				Resource: "some-resource-with-longer-name",
			})

			job.BuildConfigPath = "some-resource-with-longer-name/build.yml"

			expectedTurbineBuild.Inputs[0].ConfigPath = ""

			expectedTurbineBuild.Inputs = append(expectedTurbineBuild.Inputs, tbuilds.Input{
				Name:       "some-resource-with-longer-name",
				Resource:   "some-resource-with-longer-name",
				Type:       "git",
				Source:     tbuilds.Source{"uri": "git://some-resource-with-longer-name"},
				ConfigPath: "build.yml",
			})
		})

		It("chooses the correct input path", func() {
			turbineBuild, err := factory.Create(job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineBuild.Inputs).Should(Equal(expectedTurbineBuild.Inputs))
		})
	})

	Context("when versioned resources are specified", func() {
		It("uses them for the build's inputs", func() {
			turbineBuild, err := factory.Create(job, builds.VersionedResources{
				{
					Name:    "some-resource",
					Type:    "git-ng",
					Version: builds.Version{"version": "1"},
					Source:  builds.Source{"uri": "git://some-provided-uri"},
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineBuild.Inputs).Should(Equal([]tbuilds.Input{
				{
					Name:       "some-resource",
					Resource:   "some-resource",
					Type:       "git-ng",
					Source:     tbuilds.Source{"uri": "git://some-provided-uri"},
					Params:     tbuilds.Params{"some": "params"},
					Version:    tbuilds.Version{"version": "1"},
					ConfigPath: "build.yml",
				},
			}))
		})
	})

	Context("when the job's input is not found", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, config.Input{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			_, err := factory.Create(job, nil)
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when the job's output is not found", func() {
		BeforeEach(func() {
			job.Outputs = append(job.Outputs, config.Output{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			_, err := factory.Create(job, nil)
			Ω(err).Should(HaveOccurred())
		})
	})
})
