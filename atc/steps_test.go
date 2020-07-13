package atc_test

import (
	"encoding/json"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/yaml"
)

type StepsSuite struct {
	suite.Suite
	*require.Assertions
}

type StepTest struct {
	Title string

	ConfigYAML string
	StepConfig atc.StepConfig

	UnknownFields map[string]*json.RawMessage
}

var factoryTests = []StepTest{
	{
		Title: "get step",
		ConfigYAML: `
			get: some-name
			resource: some-resource
			params: {some: params}
			version: {some: version}
			tags: [tag-1, tag-2]
		`,
		StepConfig: &atc.GetStep{
			Name:     "some-name",
			Resource: "some-resource",
			Params:   atc.Params{"some": "params"},
			Version:  &atc.VersionConfig{Pinned: atc.Version{"some": "version"}},
			Tags:     []string{"tag-1", "tag-2"},
		},
	},
	{
		Title: "put step",

		ConfigYAML: `
			put: some-name
			resource: some-resource
			params: {some: params}
			tags: [tag-1, tag-2]
			inputs: all
			get_params: {some: get-params}
		`,
		StepConfig: &atc.PutStep{
			Name:      "some-name",
			Resource:  "some-resource",
			Params:    atc.Params{"some": "params"},
			Tags:      []string{"tag-1", "tag-2"},
			Inputs:    &atc.InputsConfig{All: true},
			GetParams: atc.Params{"some": "get-params"},
		},
	},
	{
		Title: "task step",

		ConfigYAML: `
			task: some-task
			privileged: true
			config:
			  platform: linux
			  run: {path: hello}
			file: some-task-file
			vars: {some: vars}
			params: {SOME: PARAMS}
			tags: [tag-1, tag-2]
			input_mapping: {generic: specific}
			output_mapping: {specific: generic}
			image: some-image
		`,

		StepConfig: &atc.TaskStep{
			Name:       "some-task",
			Privileged: true,
			Config: &atc.TaskConfig{
				Platform: "linux",
				Run:      atc.TaskRunConfig{Path: "hello"},
			},
			ConfigPath:        "some-task-file",
			Vars:              atc.Params{"some": "vars"},
			Params:            atc.Params{"SOME": "PARAMS"},
			Tags:              []string{"tag-1", "tag-2"},
			InputMapping:      map[string]string{"generic": "specific"},
			OutputMapping:     map[string]string{"specific": "generic"},
			ImageArtifactName: "some-image",
		},
	},
	{
		Title: "set_pipeline step",

		ConfigYAML: `
			set_pipeline: some-pipeline
			file: some-pipeline-file
			vars: {some: vars}
			var_files: [file-1, file-2]
		`,

		StepConfig: &atc.SetPipelineStep{
			Name:     "some-pipeline",
			File:     "some-pipeline-file",
			Vars:     atc.Params{"some": "vars"},
			VarFiles: []string{"file-1", "file-2"},
		},
	},
	{
		Title: "load_var step",

		ConfigYAML: `
			load_var: some-var
			file: some-var-file
			format: raw
			reveal: true
		`,

		StepConfig: &atc.LoadVarStep{
			Name:   "some-var",
			File:   "some-var-file",
			Format: "raw",
			Reveal: true,
		},
	},
	{
		Title: "try step",

		ConfigYAML: `
			try:
			  load_var: some-var
			  file: some-file
		`,

		StepConfig: &atc.TryStep{
			Step: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-var",
					File: "some-file",
				},
			},
		},
	},
	{
		Title: "do step",

		ConfigYAML: `
			do:
			- load_var: some-var
			  file: some-file
			- load_var: some-other-var
			  file: some-other-file
		`,

		StepConfig: &atc.DoStep{
			Steps: []atc.Step{
				{
					Config: &atc.LoadVarStep{
						Name: "some-var",
						File: "some-file",
					},
				},
				{
					Config: &atc.LoadVarStep{
						Name: "some-other-var",
						File: "some-other-file",
					},
				},
			},
		},
	},
	{
		Title: "in_parallel step with simple list",

		ConfigYAML: `
			in_parallel:
			- load_var: some-var
			  file: some-file
			- load_var: some-other-var
			  file: some-other-file
		`,

		StepConfig: &atc.InParallelStep{
			Config: atc.InParallelConfig{
				Steps: []atc.Step{
					{
						Config: &atc.LoadVarStep{
							Name: "some-var",
							File: "some-file",
						},
					},
					{
						Config: &atc.LoadVarStep{
							Name: "some-other-var",
							File: "some-other-file",
						},
					},
				},
			},
		},
	},
	{
		Title: "in_parallel step with config",

		ConfigYAML: `
			in_parallel:
			  steps:
			  - load_var: some-var
			    file: some-file
			  - load_var: some-other-var
			    file: some-other-file
			  limit: 3
			  fail_fast: true
		`,

		StepConfig: &atc.InParallelStep{
			Config: atc.InParallelConfig{
				Steps: []atc.Step{
					{
						Config: &atc.LoadVarStep{
							Name: "some-var",
							File: "some-file",
						},
					},
					{
						Config: &atc.LoadVarStep{
							Name: "some-other-var",
							File: "some-other-file",
						},
					},
				},
				Limit:    3,
				FailFast: true,
			},
		},
	},
	{
		Title: "aggregate step",

		ConfigYAML: `
			aggregate:
			- load_var: some-var
			  file: some-file
			- load_var: some-other-var
			  file: some-other-file
		`,

		StepConfig: &atc.AggregateStep{
			Steps: []atc.Step{
				{
					Config: &atc.LoadVarStep{
						Name: "some-var",
						File: "some-file",
					},
				},
				{
					Config: &atc.LoadVarStep{
						Name: "some-other-var",
						File: "some-other-file",
					},
				},
			},
		},
	},
	{
		Title: "timeout modifier",

		ConfigYAML: `
			load_var: some-var
			file: some-file
			timeout: 1h
		`,

		StepConfig: &atc.TimeoutStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Duration: "1h",
		},
	},
	{
		Title: "attempts modifier",

		ConfigYAML: `
			load_var: some-var
			file: some-file
			attempts: 3
		`,

		StepConfig: &atc.RetryStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Attempts: 3,
		},
	},
	{
		Title: "precedence of all hooks and modifiers",

		ConfigYAML: `
			load_var: some-var
			file: some-file
			timeout: 1h
			attempts: 3
			on_success:
			  load_var: success-var
			  file: success-file
			on_failure:
			  load_var: failure-var
			  file: failure-file
			on_abort:
			  load_var: abort-var
			  file: abort-file
			on_error:
			  load_var: error-var
			  file: error-file
			ensure:
			  load_var: ensure-var
			  file: ensure-file
		`,

		StepConfig: &atc.EnsureStep{
			Step: &atc.OnErrorStep{
				Step: &atc.OnAbortStep{
					Step: &atc.OnFailureStep{
						Step: &atc.OnSuccessStep{
							Step: &atc.RetryStep{
								Step: &atc.TimeoutStep{
									Step: &atc.LoadVarStep{
										Name: "some-var",
										File: "some-file",
									},
									Duration: "1h",
								},
								Attempts: 3,
							},
							Hook: atc.Step{
								Config: &atc.LoadVarStep{
									Name: "success-var",
									File: "success-file",
								},
							},
						},
						Hook: atc.Step{
							Config: &atc.LoadVarStep{
								Name: "failure-var",
								File: "failure-file",
							},
						},
					},
					Hook: atc.Step{
						Config: &atc.LoadVarStep{
							Name: "abort-var",
							File: "abort-file",
						},
					},
				},
				Hook: atc.Step{
					Config: &atc.LoadVarStep{
						Name: "error-var",
						File: "error-file",
					},
				},
			},
			Hook: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "ensure-var",
					File: "ensure-file",
				},
			},
		},
	},
	{
		Title: "unknown field with get step",

		ConfigYAML: `
			get: some-name
			bogus: foo
		`,

		StepConfig: &atc.GetStep{
			Name: "some-name",
		},

		UnknownFields: map[string]*json.RawMessage{"bogus": rawMessage(`"foo"`)},
	},
	{
		Title: "multiple steps defined",

		ConfigYAML: `
			put: some-name
			get: some-other-name
		`,

		StepConfig: &atc.PutStep{
			Name: "some-name",
		},

		UnknownFields: map[string]*json.RawMessage{"get": rawMessage(`"some-other-name"`)},
	},
}

func (test StepTest) Run(s *StepsSuite) {
	cleanIndents := strings.ReplaceAll(test.ConfigYAML, "\t", "")

	var step atc.Step
	actualErr := yaml.Unmarshal([]byte(cleanIndents), &step)
	s.NoError(actualErr)

	s.Equal(test.StepConfig, step.Config)
	s.Equal(test.UnknownFields, step.UnknownFields)

	remarshalled, err := json.Marshal(step)
	s.NoError(err)

	var reStep atc.Step
	err = yaml.Unmarshal(remarshalled, &reStep)
	s.NoError(err)

	s.Equal(test.StepConfig, reStep.Config)
}

func (s *StepsSuite) TestFactory() {
	for _, test := range factoryTests {
		s.Run(test.Title, func() {
			test.Run(s)
		})
	}
}

func rawMessage(s string) *json.RawMessage {
	raw := json.RawMessage(s)
	return &raw
}
