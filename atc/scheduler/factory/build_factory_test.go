package factory_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/yaml"
)

type BuildFactorySuite struct {
	suite.Suite
	*require.Assertions
}

type FactoryTest struct {
	Title string

	ConfigYAML string
	Inputs     []db.BuildInput

	CompareIDs bool
	PlanJSON   string
}

var resources = db.SchedulerResources{
	db.SchedulerResource{
		Name:   "some-resource",
		Type:   "some-resource-type",
		Source: atc.Source{"some": "source"},
	},
}

var resourceTypes = atc.VersionedResourceTypes{
	{
		ResourceType: atc.ResourceType{
			Name:   "some-resource-type",
			Type:   "some-base-resource-type",
			Source: atc.Source{"some": "type-source"},
		},
		Version: atc.Version{"some": "type-version"},
	},
}

var factoryTests = []FactoryTest{
	{
		Title: "get step",
		ConfigYAML: `
			get: some-name
			resource: some-resource
			params: {some: params}
			version: {doesnt: matter}
			tags: [tag-1, tag-2]
		`,
		Inputs: []db.BuildInput{
			{
				Name:    "some-name",
				Version: atc.Version{"some": "version"},
			},
		},
		PlanJSON: `{
			"get": {
				"name": "some-name",
				"type": "some-resource-type",
				"resource": "some-resource",
				"source": {"some":"source"},
				"params": {"some":"params"},
				"version": {"some":"version"},
				"tags": ["tag-1", "tag-2"],
				"resource_types": [
					{
						"name": "some-resource-type",
						"type": "some-base-resource-type",
						"source": {"some": "type-source"},
						"version": {"some": "type-version"}
					}
				]
			}
		}`,
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
						"source": {"some":"source"},
						"params": {"some":"params"},
						"tags": ["tag-1", "tag-2"],
						"resource_types": [
							{
								"name": "some-resource-type",
								"type": "some-base-resource-type",
								"source": {"some": "type-source"},
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
						"source": {"some":"source"},
						"params": {"some":"get-params"},
						"tags": ["tag-1", "tag-2"],
						"version_from": "1",
						"resource_types": [
							{
								"name": "some-resource-type",
								"type": "some-base-resource-type",
								"source": {"some": "type-source"},
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

		PlanJSON: `{
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
				"resource_types": [
					{
						"name": "some-resource-type",
						"type": "some-base-resource-type",
						"source": {"some": "type-source"},
						"version": {"some": "type-version"}
					}
				]
			}
		}`,
	},
	{
		Title: "set_pipeline step",

		ConfigYAML: `
			set_pipeline: some-pipeline
			file: some-pipeline-file
			vars: {some: vars}
			var_files: [file-1, file-2]
		`,

		PlanJSON: `{
			"set_pipeline": {
				"name": "some-pipeline",
				"file": "some-pipeline-file",
				"vars": {"some": "vars"},
				"var_files": ["file-1", "file-2"]
			}
		}`,
	},
	{
		Title: "load_var step",

		ConfigYAML: `
			load_var: some-var
			file: some-pipeline-file
			format: raw
			reveal: true
		`,

		PlanJSON: `{
			"load_var": {
				"name": "some-var",
				"file": "some-pipeline-file",
				"format": "raw",
				"reveal": true
			}
		}`,
	},
	{
		Title: "try step",

		ConfigYAML: `
			try:
			  load_var: some-var
			  file: some-file
		`,

		PlanJSON: `{
			"try": {
				"step": {
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

		ConfigYAML: `
			do:
			- load_var: some-var
			  file: some-file
			- load_var: some-other-var
			  file: some-other-file
		`,

		PlanJSON: `{
			"do": [
				{
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				{
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			]
		}`,
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

		PlanJSON: `{
			"in_parallel": {
				"steps": [
					{
						"load_var": {
							"name": "some-var",
							"file": "some-file"
						}
					},
					{
						"load_var": {
							"name": "some-other-var",
							"file": "some-other-file"
						}
					}
				]
			}
		}`,
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

		PlanJSON: `{
			"in_parallel": {
				"steps": [
					{
						"load_var": {
							"name": "some-var",
							"file": "some-file"
						}
					},
					{
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
		Title: "aggregate step",

		ConfigYAML: `
			aggregate:
			- load_var: some-var
			  file: some-file
			- load_var: some-other-var
			  file: some-other-file
		`,

		PlanJSON: `{
			"aggregate": [
				{
					"load_var": {
						"name": "some-var",
						"file": "some-file"
					}
				},
				{
					"load_var": {
						"name": "some-other-var",
						"file": "some-other-file"
					}
				}
			]
		}`,
	},
	{
		Title: "timeout modifier",

		ConfigYAML: `
			load_var: some-var
			file: some-file
			timeout: 1h
		`,

		PlanJSON: `{
			"timeout": {
				"step": {
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

		ConfigYAML: `
			load_var: some-var
			file: some-file
			attempts: 3
		`,

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
		Title: "timeout and attempts modifier",

		ConfigYAML: `
			load_var: some-var
			file: some-file
			timeout: 1h
			attempts: 3
		`,

		PlanJSON: `{
			"retry": [
				{
					"timeout": {
						"step": {
							"load_var": {
								"name": "some-var",
								"file": "some-file"
							}
						},
						"duration": "1h"
					}
				},
				{
					"timeout": {
						"step": {
							"load_var": {
								"name": "some-var",
								"file": "some-file"
							}
						},
						"duration": "1h"
					}
				},
				{
					"timeout": {
						"step": {
							"load_var": {
								"name": "some-var",
								"file": "some-file"
							}
						},
						"duration": "1h"
					}
				}
			]
		}`,
	},
}

func init() {
	for _, hookType := range []string{"on_success", "on_failure", "on_abort", "ensure"} {
		factoryTests = append(factoryTests, FactoryTest{
			Title: hookType + " hook",

			ConfigYAML: fmt.Sprintf(`
				load_var: some-var
				file: some-file
				%s:
				  load_var: some-hook-var
				  file: some-hook-file
			`, hookType),

			PlanJSON: fmt.Sprintf(`{
				"%s": {
					"step": {
						"load_var": {
							"name": "some-var",
							"file": "some-file"
						}
					},
					"%s": {
						"load_var": {
							"name": "some-hook-var",
							"file": "some-hook-file"
						}
					}
				}
			}`, hookType, hookType),
		}, FactoryTest{
			Title: hookType + " hook with timeout",

			ConfigYAML: fmt.Sprintf(`
				load_var: some-var
				file: some-file
				timeout: 1h
				%s:
				  load_var: some-hook-var
				  file: some-hook-file
			`, hookType),

			// timeout applies to inner step, not hook
			PlanJSON: fmt.Sprintf(`{
				"%s": {
					"step": {
						"timeout": {
							"step": {
								"load_var": {
									"name": "some-var",
									"file": "some-file"
								}
							},
							"duration": "1h"
						}
					},
					"%s": {
						"load_var": {
							"name": "some-hook-var",
							"file": "some-hook-file"
						}
					}
				}
			}`, hookType, hookType),
		}, FactoryTest{
			Title: hookType + " hook with attempts",

			ConfigYAML: fmt.Sprintf(`
				load_var: some-var
				file: some-file
				attempts: 3
				%s:
				  load_var: some-hook-var
				  file: some-hook-file
			`, hookType),

			// timeout applies to inner step, not hook
			PlanJSON: fmt.Sprintf(`{
				"%s": {
					"step": {
						"retry": [
							{
								"load_var": {
									"name": "some-var",
									"file": "some-file"
								}
							},
							{
								"load_var": {
									"name": "some-var",
									"file": "some-file"
								}
							},
							{
								"load_var": {
									"name": "some-var",
									"file": "some-file"
								}
							}
						]
					},
					"%s": {
						"load_var": {
							"name": "some-hook-var",
							"file": "some-hook-file"
						}
					}
				}
			}`, hookType, hookType),
		})
	}

	for _, hookType := range []string{"on_success", "on_failure", "on_abort"} {
		factoryTests = append(factoryTests, FactoryTest{
			Title: hookType + " hook with ensure",

			ConfigYAML: fmt.Sprintf(`
				load_var: some-var
				file: some-file
				%s:
				  load_var: some-hook-var
				  file: some-hook-file
				ensure:
				  load_var: some-ensure-var
				  file: some-ensure-file
			`, hookType),

			PlanJSON: fmt.Sprintf(`{
				"ensure": {
					"step": {
						"%s": {
							"step": {
								"load_var": {
									"name": "some-var",
									"file": "some-file"
								}
							},
							"%s": {
								"load_var": {
									"name": "some-hook-var",
									"file": "some-hook-file"
								}
							}
						}
					},
					"ensure": {
						"load_var": {
							"name": "some-ensure-var",
							"file": "some-ensure-file"
						}
					}
				}
			}`, hookType, hookType),
		})
	}
}

func (test FactoryTest) Run(s *BuildFactorySuite) {
	factory := factory.NewBuildFactory(atc.NewPlanFactory(0))

	// thank goodness gofmt makes this a reasonable assumption
	cleanIndents := strings.ReplaceAll(test.ConfigYAML, "\t", "")

	var config atc.PlanConfig
	err := yaml.UnmarshalStrict([]byte(cleanIndents), &config)
	s.NoError(err)

	actualPlan, actualErr := factory.Create(config, resources, resourceTypes, test.Inputs)
	s.NoError(actualErr)

	seenIDs := map[atc.PlanID]bool{}
	actualPlan.Each(func(p *atc.Plan) {
		s.NotEmpty(p.ID)

		// make sure all IDs are unique
		s.False(seenIDs[p.ID], "duplicate plan id: %s", p.ID)
		seenIDs[p.ID] = true

		// strip out the IDs, we don't really care what their value is
		if !test.CompareIDs {
			p.ID = ""
		}
	})
	s.NotEmpty(seenIDs)

	actualJSON, err := json.Marshal(actualPlan)
	s.NoError(err)

	s.JSONEq(test.PlanJSON, string(actualJSON))
}

func (s *BuildFactorySuite) TestFactory() {
	for _, test := range factoryTests {
		s.Run(test.Title, func() {
			test.Run(s)
		})
	}
}
