package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	. "github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/turbine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory", func() {
	var (
		factory *BuildFactory

		job       atc.JobConfig
		resources atc.ResourceConfigs

		expectedBuildPlan engine.BuildPlan
	)

	BeforeEach(func() {
		factory = &BuildFactory{}

		job = atc.JobConfig{
			Name: "some-job",

			BuildConfig: turbine.Config{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
				Run: turbine.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

			Privileged: true,

			BuildConfigPath: "some-input/build.yml",

			Inputs: []atc.InputConfig{
				{
					RawName:  "some-input",
					Resource: "some-resource",
					Params:   atc.Params{"some": "params"},
				},
			},

			Outputs: []atc.OutputConfig{
				{
					Resource:     "some-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.OutputCondition{"success"},
				},
				{
					Resource:     "some-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.OutputCondition{"failure"},
				},
				{
					Resource:     "some-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.OutputCondition{},
				},
			},
		}

		expectedBuildPlan = engine.BuildPlan{
			Config: turbine.Config{
				Image: "some-image",

				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},

				Run: turbine.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

			Inputs: []turbine.Input{
				{
					Name:       "some-input",
					Resource:   "some-resource",
					Type:       "git",
					Source:     turbine.Source{"uri": "git://some-resource"},
					Params:     turbine.Params{"some": "params"},
					ConfigPath: "build.yml",
				},
			},

			Outputs: []turbine.Output{
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []turbine.OutputCondition{turbine.OutputConditionSuccess},
					Params: turbine.Params{"foo": "bar"},
					Source: turbine.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []turbine.OutputCondition{turbine.OutputConditionFailure},
					Params: turbine.Params{"foo": "bar"},
					Source: turbine.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []turbine.OutputCondition{},
					Params: turbine.Params{"foo": "bar"},
					Source: turbine.Source{"uri": "git://some-resource"},
				},
			},

			Privileged: true,
		}

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
			{
				Name:   "some-dependant-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-dependant-resource"},
			},
			{
				Name:   "some-output-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-output-resource"},
			},
			{
				Name:   "some-resource-with-longer-name",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource-with-longer-name"},
			},
			{
				Name:   "some-named-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-named-resource"},
			},
		}
	})

	It("creates a turbine build based on the job's configuration", func() {
		buildPlan, err := factory.Create(job, resources, nil)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(buildPlan).Should(Equal(expectedBuildPlan))
	})

	Context("when an input has an explicit name", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, atc.InputConfig{
				RawName:  "some-named-input",
				Resource: "some-named-resource",
				Params:   atc.Params{"some": "named-params"},
			})

			expectedBuildPlan.Inputs = append(expectedBuildPlan.Inputs, turbine.Input{
				Name:     "some-named-input",
				Resource: "some-named-resource",
				Type:     "git",
				Source:   turbine.Source{"uri": "git://some-named-resource"},
				Params:   turbine.Params{"some": "named-params"},
			})
		})

		It("uses it as the name for the input", func() {
			buildPlan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(buildPlan.Inputs).Should(Equal(expectedBuildPlan.Inputs))
		})
	})

	Context("when an explicitly named input is the source of the config", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, atc.InputConfig{
				RawName:  "some-named-input",
				Resource: "some-named-resource",
				Params:   atc.Params{"some": "named-params"},
			})

			job.BuildConfigPath = "some-named-input/build.yml"

			expectedBuildPlan.Inputs[0].ConfigPath = ""

			expectedBuildPlan.Inputs = append(expectedBuildPlan.Inputs, turbine.Input{
				Name:       "some-named-input",
				Resource:   "some-named-resource",
				Type:       "git",
				Source:     turbine.Source{"uri": "git://some-named-resource"},
				Params:     turbine.Params{"some": "named-params"},
				ConfigPath: "build.yml",
			})
		})

		It("uses the explicit name to match the config path", func() {
			buildPlan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(buildPlan.Inputs).Should(Equal(expectedBuildPlan.Inputs))
		})
	})

	Context("when two inputs have overlappying names for the config path", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, atc.InputConfig{
				Resource: "some-resource-with-longer-name",
			})

			job.BuildConfigPath = "some-resource-with-longer-name/build.yml"

			expectedBuildPlan.Inputs[0].ConfigPath = ""

			expectedBuildPlan.Inputs = append(expectedBuildPlan.Inputs, turbine.Input{
				Name:       "some-resource-with-longer-name",
				Resource:   "some-resource-with-longer-name",
				Type:       "git",
				Source:     turbine.Source{"uri": "git://some-resource-with-longer-name"},
				ConfigPath: "build.yml",
			})
		})

		It("chooses the correct input path", func() {
			buildPlan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(buildPlan.Inputs).Should(Equal(expectedBuildPlan.Inputs))
		})
	})

	Context("when inputs with versions are specified", func() {
		It("uses them for the build's inputs", func() {
			buildPlan, err := factory.Create(job, resources, []db.BuildInput{
				{
					Name: "some-input",
					VersionedResource: db.VersionedResource{
						Resource: "some-resource",
						Type:     "git-ng",
						Version:  db.Version{"version": "1"},
						Source:   db.Source{"uri": "git://some-provided-uri"},
					},
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(buildPlan.Inputs).Should(Equal([]turbine.Input{
				{
					Name:       "some-input",
					Resource:   "some-resource",
					Type:       "git-ng",
					Source:     turbine.Source{"uri": "git://some-provided-uri"},
					Params:     turbine.Params{"some": "params"},
					Version:    turbine.Version{"version": "1"},
					ConfigPath: "build.yml",
				},
			}))
		})
	})

	Context("when the job's input is not found", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, atc.InputConfig{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			_, err := factory.Create(job, resources, nil)
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when the job's output is not found", func() {
		BeforeEach(func() {
			job.Outputs = append(job.Outputs, atc.OutputConfig{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			_, err := factory.Create(job, resources, nil)
			Ω(err).Should(HaveOccurred())
		})
	})
})
