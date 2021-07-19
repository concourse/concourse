package builds_test

import (
	"encoding/json"
	"testing"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/db"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PlannerSuite struct {
	suite.Suite
	*require.Assertions
}

func TestPlanner(t *testing.T) {
	suite.Run(t, &PlannerSuite{
		Assertions: require.New(t),
	})
}

type PlannerTest struct {
	Title string

	Config atc.StepConfig
	Inputs []db.BuildInput

	CompareIDs bool
	PlanJSON   string
	Err        error
}

var resources = db.SchedulerResources{
	db.SchedulerResource{
		Name:   "some-resource",
		Type:   "some-resource-type",
		Source: atc.Source{"some": "source"},
	},
	db.SchedulerResource{
		Name:   "some-base-resource",
		Type:   "some-base-resource-type",
		Source: atc.Source{"some": "source"},
	},
}

var resourceTypes = atc.VersionedResourceTypes{
	{
		ResourceType: atc.ResourceType{
			Name:     "some-resource-type",
			Type:     "some-base-resource-type",
			Source:   atc.Source{"some": "type-source"},
			Defaults: atc.Source{"default-key": "default-value"},
		},
		Version: atc.Version{"some": "type-version"},
	},
}

var prototypes = atc.Prototypes{
	{
		Name: "some-prototype",
		Type: "some-base-resource-type",
		Source: atc.Source{
			"some": "prototype-source",
		},
		Defaults: atc.Source{
			"default-key":       "default-value",
			"other-default-key": "other-default-value",
		},
	},
}

var baseResourceTypeDefaults = map[string]atc.Source{
	"some-base-resource-type": {"default-key": "default-value"},
}

var factoryTests = []PlannerTest{
	{
		Title: "get step",
		Config: &atc.GetStep{
			Name:         "some-name",
			Resource:     "some-resource",
			Params:       atc.Params{"some": "params"},
			Version:      &atc.VersionConfig{Pinned: atc.Version{"doesnt": "matter"}},
			Tags:         atc.Tags{"tag-1", "tag-2"},
			Timeout:      "1h",
			ForceRefresh: true,
		},
		Inputs: []db.BuildInput{
			{
				Name:    "some-name",
				Version: atc.Version{"some": "version"},
			},
		},
		PlanJSON: `{
			"id": "(unique)",
			"get": {
				"name": "some-name",
				"type": "some-resource-type",
				"resource": "some-resource",
				"source": {"some":"source","default-key":"default-value"},
				"params": {"some":"params"},
				"version": {"some":"version"},
				"tags": ["tag-1", "tag-2"],
				"timeout": "1h",
				"force_refresh": true,
				"resource_types": [
					{
						"name": "some-resource-type",
						"type": "some-base-resource-type",
						"source": {"some": "type-source"},
						"defaults": {"default-key":"default-value"},
						"version": {"some": "type-version"}
					}
				]
			}
		}`,
	},
	{
		Title: "get step with base resource type",
		Config: &atc.GetStep{
			Name:         "some-name",
			Resource:     "some-base-resource",
			Params:       atc.Params{"some": "params"},
			Version:      &atc.VersionConfig{Pinned: atc.Version{"doesnt": "matter"}},
			Tags:         atc.Tags{"tag-1", "tag-2"},
			ForceRefresh: true,
		},
		Inputs: []db.BuildInput{
			{
				Name:    "some-name",
				Version: atc.Version{"some": "version"},
			},
		},
		PlanJSON: `{
			"id": "(unique)",
			"get": {
				"name": "some-name",
				"type": "some-base-resource-type",
				"resource": "some-base-resource",
				"source": {"some":"source","default-key":"default-value"},
				"params": {"some":"params"},
				"version": {"some":"version"},
				"tags": ["tag-1", "tag-2"],
				"force_refresh": true,
				"resource_types": [
					{
						"name": "some-resource-type",
						"type": "some-base-resource-type",
						"source": {"some": "type-source"},
						"defaults": {"default-key":"default-value"},
						"version": {"some": "type-version"}
					}
				]
			}
		}`,
	},
	{
		Title: "get step with unknown resource",
		Config: &atc.GetStep{
			Name:     "some-name",
			Resource: "bogus-resource",
		},
		Err: builds.UnknownResourceError{Resource: "bogus-resource"},
	},
	{
		Title: "get step with no available version",
		Config: &atc.GetStep{
			Name:     "some-name",
			Resource: "some-resource",
		},
		Err: builds.VersionNotProvidedError{Input: "some-name"},
	},
	{
		Title: "put step",
		Config: &atc.PutStep{
			Name:      "some-name",
			Resource:  "some-resource",
			Params:    atc.Params{"some": "params"},
			Tags:      atc.Tags{"tag-1", "tag-2"},
			Inputs:    &atc.InputsConfig{All: true},
			GetParams: atc.Params{"some": "get-params"},
			Timeout:   "1h",
		},
		Inputs: []db.BuildInput{
			{
				Name:    "some-name",
				Version: atc.Version{"some": "version"},
			},
		},

		// the ids are significant for versioned_from
		CompareIDs: true,
		PlanJSON: `{
			"id": "3",
			"on_success": {
				"step": {
					"id": "1",
					"put": {
						"name": "some-name",
						"type": "some-resource-type",
						"resource": "some-resource",
						"inputs": "all",
						"source": {"some":"source","default-key":"default-value"},
						"params": {"some":"params"},
						"tags": ["tag-1", "tag-2"],
						"timeout": "1h",
						"resource_types": [
							{
								"name": "some-resource-type",
								"type": "some-base-resource-type",
								"source": {"some": "type-source"},
								"defaults": {"default-key":"default-value"},
								"version": {"some": "type-version"}
							}
						]
					}
				},
				"on_success": {
					"id": "2",
					"get": {
						"name": "some-name",
						"type": "some-resource-type",
						"resource": "some-resource",
						"source": {"some":"source","default-key":"default-value"},
						"params": {"some":"get-params"},
						"tags": ["tag-1", "tag-2"],
						"version_from": "1",
						"timeout": "1h",
						"resource_types": [
							{
								"name": "some-resource-type",
								"type": "some-base-resource-type",
								"source": {"some": "type-source"},
								"defaults": {"default-key":"default-value"},
								"version": {"some": "type-version"}
							}
						]
					}
				}
			}
		}`,
	},
	{
		Title: "task step",

		Config: &atc.TaskStep{
			Name:       "some-task",
			Privileged: true,
			Config: &atc.TaskConfig{
				Platform: "linux",
				Run:      atc.TaskRunConfig{Path: "hello"},
			},
			ConfigPath:        "some-task-file",
			Vars:              atc.Params{"some": "vars"},
			Params:            atc.TaskEnv{"SOME": "PARAMS"},
			Tags:              atc.Tags{"tag-1", "tag-2"},
			InputMapping:      map[string]string{"generic": "specific"},
			OutputMapping:     map[string]string{"specific": "generic"},
			ImageArtifactName: "some-image",
			Timeout:           "1h",
		},

		PlanJSON: `{
			"id": "(unique)",
			"task": {
				"name": "some-task",
				"privileged": true,
				"config": {
					"platform": "linux",
					"run": {"path": "hello"}
				},
				"config_path": "some-task-file",
				"vars": {"some": "vars"},
				"params": {"SOME": "PARAMS"},
				"tags": ["tag-1", "tag-2"],
				"input_mapping": {"generic": "specific"},
				"output_mapping": {"specific": "generic"},
				"image": "some-image",
				"timeout": "1h",
				"resource_types": [
					{
						"name": "some-resource-type",
						"type": "some-base-resource-type",
						"source": {"some": "type-source"},
						"defaults": {"default-key":"default-value"},
						"version": {"some": "type-version"}
					}
				]
			}
		}`,
	},
	{
		Title: "task step with top level container limits",

		Config: &atc.TaskStep{
			Name:       "some-task",
			Privileged: true,
			Config: &atc.TaskConfig{
				Platform: "linux",
				Run:      atc.TaskRunConfig{Path: "hello"},
			},
			Limits: &atc.ContainerLimits{
				CPU:    newCPULimit(456),
				Memory: newMemoryLimit(2048),
			},
			ConfigPath:        "some-task-file",
			Vars:              atc.Params{"some": "vars"},
			Params:            atc.TaskEnv{"SOME": "PARAMS"},
			Tags:              atc.Tags{"tag-1", "tag-2"},
			InputMapping:      map[string]string{"generic": "specific"},
			OutputMapping:     map[string]string{"specific": "generic"},
			ImageArtifactName: "some-image",
			Timeout:           "1h",
		},

		PlanJSON: `{
			"id": "(unique)",
			"task": {
				"name": "some-task",
				"privileged": true,
				"config": {
					"platform": "linux",
					"run": {"path": "hello"}
				},
				"config_path": "some-task-file",
				"vars": {"some": "vars"},
				"container_limits": {"cpu": 456, "memory": 2048},
				"params": {"SOME": "PARAMS"},
				"tags": ["tag-1", "tag-2"],
				"input_mapping": {"generic": "specific"},
				"output_mapping": {"specific": "generic"},
				"image": "some-image",
				"timeout": "1h",
				"resource_types": [
					{
						"name": "some-resource-type",
						"type": "some-base-resource-type",
						"source": {"some": "type-source"},
						"defaults": {"default-key":"default-value"},
						"version": {"some": "type-version"}
					}
				]
			}
		}`,
	},
	{
		Title: "run step",

		Config: &atc.RunStep{
			Message: "some-message",
			Type:    "some-prototype",
			Params: atc.Params{
				"some-param":        "some-val",
				"other-default-key": "override-defaults",
			},
			Privileged: true,
			Tags:       atc.Tags{"tag-1", "tag-2"},
			Limits: &atc.ContainerLimits{
				CPU:    newCPULimit(456),
				Memory: newMemoryLimit(2048),
			},
			Timeout: "1h",
		},

		PlanJSON: `{
			"id": "(unique)",
			"run": {
				"message": "some-message",
				"type": "some-prototype",
				"object": {
					"some-param": "some-val",
					"default-key": "default-value",
					"other-default-key": "override-defaults"
				},
				"privileged": true,
				"tags": ["tag-1", "tag-2"],
				"container_limits": {"cpu": 456, "memory": 2048},
				"timeout": "1h"
			}
		}`,
	},
	{
		Title: "set_pipeline step",

		Config: &atc.SetPipelineStep{
			Name:         "some-pipeline",
			File:         "some-pipeline-file",
			Vars:         atc.Params{"some": "vars"},
			VarFiles:     []string{"file-1", "file-2"},
			InstanceVars: atc.InstanceVars{"branch": "feature/foo"},
		},

		PlanJSON: `{
			"id": "(unique)",
			"set_pipeline": {
				"name": "some-pipeline",
				"file": "some-pipeline-file",
				"vars": {"some": "vars"},
				"var_files": ["file-1", "file-2"],
				"instance_vars": {"branch": "feature/foo"}
			}
		}`,
	},
	{
		Title: "load_var step",

		Config: &atc.LoadVarStep{
			Name:   "some-var",
			File:   "some-var-file",
			Format: "raw",
			Reveal: true,
		},

		PlanJSON: `{
			"id": "(unique)",
			"load_var": {
				"name": "some-var",
				"file": "some-var-file",
				"format": "raw",
				"reveal": true
			}
		}`,
	},
	{
		Title: "try step",

		Config: &atc.TryStep{
			Step: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-var",
					File: "some-file",
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"try": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				}
			}
		}`,
	},
	{
		Title: "do step",

		Config: &atc.DoStep{
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

		PlanJSON: `{
			"id": "(unique)",
			"do": [
				{
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				{
					"id": "(unique)",
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			]
		}`,
	},
	{
		Title: "in_parallel step",

		Config: &atc.InParallelStep{
			Config: atc.InParallelConfig{
				Limit:    3,
				FailFast: true,
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

		PlanJSON: `{
			"id": "(unique)",
			"in_parallel": {
				"steps": [
					{
						"id": "(unique)",
						"load_var": {
							"name": "some-var",
							"file": "some-file"
						}
					},
					{
						"id": "(unique)",
						"load_var": {
							"name": "some-other-var",
							"file": "some-other-file"
						}
					}
				],
				"limit": 3,
				"fail_fast": true
			}
		}`,
	},
	{
		Title: "across step",

		Config: &atc.AcrossStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Vars: []atc.AcrossVarConfig{
				{
					Var:         "var1",
					Values:      []interface{}{"a1", "a2"},
					MaxInFlight: &atc.MaxInFlightConfig{All: true},
				},
				{
					Var:         "var2",
					Values:      "((dynamic))",
					MaxInFlight: &atc.MaxInFlightConfig{Limit: 1},
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"across": {
				"vars": [
					{
						"name": "var1",
						"values": ["a1", "a2"],
						"max_in_flight": "all"
					},
					{
						"name": "var2",
						"values": "((dynamic))",
						"max_in_flight": 1
					}
				],
				"substep_template": "{\"id\":\"ACROSS_SUBSTEP_TEMPLATE\",\"load_var\":{\"name\":\"some-var\",\"file\":\"some-file\"}}"
			}
		}`,
	},
	{
		Title: "timeout modifier",

		Config: &atc.TimeoutStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Duration: "1h",
		},

		PlanJSON: `{
			"id": "(unique)",
			"timeout": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				"duration": "1h"
			}
		}`,
	},
	{
		Title: "attempts modifier",

		Config: &atc.RetryStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Attempts: 3,
		},

		CompareIDs: true,
		PlanJSON: `{
			"id": "4",
			"retry": [
				{
					"id": "1",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				{
					"id": "2",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				{
					"id": "3",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				}
			]
		}`,
	},
	{
		Title: "on_success step",

		Config: &atc.OnSuccessStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Hook: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-other-var",
					File: "some-other-file",
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"on_success": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				"on_success": {
					"id": "(unique)",
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			}
		}`,
	},
	{
		Title: "on_failure step",

		Config: &atc.OnFailureStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Hook: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-other-var",
					File: "some-other-file",
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"on_failure": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				"on_failure": {
					"id": "(unique)",
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			}
		}`,
	},
	{
		Title: "on_error step",

		Config: &atc.OnErrorStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Hook: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-other-var",
					File: "some-other-file",
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"on_error": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				"on_error": {
					"id": "(unique)",
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			}
		}`,
	},
	{
		Title: "on_abort step",

		Config: &atc.OnAbortStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Hook: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-other-var",
					File: "some-other-file",
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"on_abort": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				"on_abort": {
					"id": "(unique)",
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			}
		}`,
	},
	{
		Title: "ensure step",

		Config: &atc.EnsureStep{
			Step: &atc.LoadVarStep{
				Name: "some-var",
				File: "some-file",
			},
			Hook: atc.Step{
				Config: &atc.LoadVarStep{
					Name: "some-other-var",
					File: "some-other-file",
				},
			},
		},

		PlanJSON: `{
			"id": "(unique)",
			"ensure": {
				"step": {
					"id": "(unique)",
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				"ensure": {
					"id": "(unique)",
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			}
		}`,
	},
}

func (test PlannerTest) Run(s *PlannerSuite) {
	factory := builds.NewPlanner(atc.NewPlanFactory(0))

	actualPlan, actualErr := factory.Create(test.Config, resources, resourceTypes, prototypes, test.Inputs)

	if test.Err != nil {
		s.Equal(test.Err, actualErr)
		return
	} else {
		s.NoError(actualErr)
	}

	seenIDs := map[atc.PlanID]bool{}
	actualPlan.Each(func(p *atc.Plan) {
		s.NotEmpty(p.ID)

		// make sure all IDs are unique
		s.False(seenIDs[p.ID], "duplicate plan id: %s", p.ID)
		seenIDs[p.ID] = true

		// strip out the IDs, we don't really care what their value is
		if !test.CompareIDs {
			p.ID = "(unique)"
		}
	})
	s.NotEmpty(seenIDs)

	actualJSON, err := json.Marshal(actualPlan)
	s.NoError(err)

	s.JSONEq(test.PlanJSON, string(actualJSON))
}

func (s *PlannerSuite) TestFactory() {
	atc.LoadBaseResourceTypeDefaults(baseResourceTypeDefaults)
	for _, test := range factoryTests {
		s.Run(test.Title, func() {
			test.Run(s)
		})
	}
	atc.LoadBaseResourceTypeDefaults(map[string]atc.Source{})
}

func newCPULimit(cpuLimit uint64) *atc.CPULimit {
	limit := atc.CPULimit(cpuLimit)
	return &limit
}

func newMemoryLimit(memoryLimit uint64) *atc.MemoryLimit {
	limit := atc.MemoryLimit(memoryLimit)
	return &limit
}
