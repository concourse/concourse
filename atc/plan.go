package atc

// // current
// atc.Plan {
// 	Task: TaskPlan {
// 		// custom resource type
// 		// getvar
// 	}
// }

// // a
// Plan: atc.Plan {
// 	InParallel: {
// 		Plan {
// 			Check: // resource type
// 		},
// 		Plan {
// 			GetVar: // getvar
// 		},
// 	},
// 	Plan {
// 		Task: // references the Check and GetVar
// 	}
// }

// resource_types:
// - name: a
// 	type: registry-image
// - name: b
// 	type a

// resources:
// - name: c
// 	type b

// jobs:
// - name: foo
// 	plan:
// 		- get: c

// Plan: {
// 	OnSuccess {
// 		Step: OnSuccess {
// 			Step: OnSuccess {
// 				Step: OnSuccess{
// 					Step: Check {
// 						ID: 1
// 						Name: "a"
// 						BaseImageType: "registry-image"
// 					}
// 					Next: Get {
// 						ID: 2
// 						Name: "a"
// 						BaseImageType: "registry-image"
// 					}
// 				}
// 				Next: Check {
// 					ID: 3
// 					Name: "b"
// 					ImageSpecFrom: 2
// 				}
// 			}
// 			Next: Get {
// 				ID: 4
// 				Name: "b"
// 				ImageSpecFrom: 2
// 			}
// 		}
// 		Next: Get {
// 			ID: 5
// 			Name: "c"
// 			ImageSpecFrom: 4
// 		}
// 	}
// }

// resource_types:
// - name: a
// 	type: registry-image

// resources:
// - name: b
// 	type: a

// jobs:
// - name: foo
// 	plan:
// 		- get: b

// Plan: {
// 	Get: {
// 		Name: b
// 		ImageCheckPlan: {
// 			OnSuccess: {
//				ID: 0
// 				Step: Check {
// 					ID: 1
// 					Name: a
// 					BaseImageType: registry-image
// 				}
// 				Next: Get {
// 					ID: 2
// 					Name: a
// 					BaseImageType: registry-image
// 					VersionFrom: 1
// 				}
// 			}
// 		}
// 	}
// }

// resource_types:
// - name: a
// 	type: registry-image
// - name: b
// 	type: a

// resources:
// - name: c
// 	type: b

// jobs:
// - name: foo
// 	plan:
// 		- get: c

// Plan: {
// 	Get: {
// 		Name: c
// 		ImageCheckPlan: {
//				ID: 0
// 				Check {
// 					Name: b
// 					ImageCheckPlan: {
// 						ID:
// 						Check: {
//							Name: a
//							BaseImageType: registry-image
//						}
// 					}
// 					ImageGetPlan: {
// 						ID:
// 						Get: {
//							Name: a
//							BaseImageType: registry-image
//						}
// 					}
// 				}
// 			}
// 		ImageGetPlan: {
//				ID: 0
// 				Get {
// 					Name: b
// 					ImageCheckPlan: {
// 						ID:
// 						Check: {
//							Name: a
//							BaseImageType: registry-image
//						}
// 					}
// 					ImageGetPlan: {
// 						ID:
// 						Get: {
//							Name: a
//							BaseImageType: registry-image
//						}
// 					}
// 				}
// 			}
// 		}
// 	}

// Plan: {
// 	Check {
// 		Name: a-check
// 		ImageSpec: // default registry-image
// 	} // produces "abc" and stores in runState["a-check"]
// 	Get: {
// 		Name: a-get
// 		Version: [[a-check]]	// "abc"
// 		Vars: map[string]string {
// 			version: "((a-check))"	// lookup runState.GetVar("a-check") and store resulte in Version
// 		}
// 	}
// 	Check {
// 		Name: b-check
// 		ImageSpec:
// 		Vars: map[string]string {
// 			imagespec: a-get
// 		}
// 	}
// 	Get: {
// 		Name: b-get
// 		Version: b-check
// 	}
// 	Check {
// 		Name: c-check
// 	}
// 	Get: {
// 		Name: c-get
// 		Version: c-check
// 	}
// }

// resource_types:
// - name: vault
// 	type: registry-image
// 	source:
// 		repository: hashicorp/vault

// var_sources:
// - name: z
// - name: a
// 	type: vault
// 	source:
// 		team: a
// 		token: Bearer ((z:token))

// plan:
// - get_var: path
// 	source: a
// - get: foo
// 	params:
// 	- foo: ((.:path))

// Plan: {
// 	GetVar: {
// 		PlanID: "z-get-var"
// 		Name: "z",
// 		Path: "token",
// 	},
// 	Check: {
// 		PlanID: "a-check"
// 	},
// 	Get: {
// 		PlanID: "a-get"
// 	},
// 	GetVar: {
// 		PlanID: "a-get-var"
// 		Name: "a",
// 		Path: "path",
// 		ImageSpec: [[a-get]],	// filled from "a-get"
// 		Source: {	// filled in from global creds
// 			"team": "a",
// 			"token": "Bearer [[z-get-var]]"
// 		},
// 	},	// 1. store result in runState["a:path"]
// 	GetVar: {
// 		PlanID: "b-get-var"
// 		Name: "b",
// 		Path: "path",
// 		Source: atc.Source {	// 2. Source.Evaulate(runState)
// 			"foo": "((a:path))",	// 3. go through and template (()) by looking up runState.GetVar(...)
// 		}
// 	},	// 4. store result in runState["b:path"]
// 	Get: {
// 		PlanID: "get-foo"
// 		Name: "foo",
// 		Params: atc.Params{
// 			"foo": "((.:path))",
// 		}
// 	}
// }

// // b
// atc.PlanContext {
// 	ResourceTypeConfig:
// 	VarSourceConfig:

// 	Plan: atc.Plan {
// 		Task: TaskPlan {
// 			// ResourceTypeConfig and VarSourceConfig gets passed around and dynamically creates new steps
// 		}
// 	}
// }
type Plan struct {
	ID       PlanID `json:"id"`
	Attempts []int  `json:"attempts,omitempty"`

	Get         *GetPlan         `json:"get,omitempty"`
	Put         *PutPlan         `json:"put,omitempty"`
	Check       *CheckPlan       `json:"check,omitempty"`
	Task        *TaskPlan        `json:"task,omitempty"`
	SetPipeline *SetPipelinePlan `json:"set_pipeline,omitempty"`
	LoadVar     *LoadVarPlan     `json:"load_var,omitempty"`

	Do         *DoPlan         `json:"do,omitempty"`
	InParallel *InParallelPlan `json:"in_parallel,omitempty"`
	Across     *AcrossPlan     `json:"across,omitempty"`

	OnSuccess *OnSuccessPlan `json:"on_success,omitempty"`
	OnFailure *OnFailurePlan `json:"on_failure,omitempty"`
	OnAbort   *OnAbortPlan   `json:"on_abort,omitempty"`
	OnError   *OnErrorPlan   `json:"on_error,omitempty"`
	Ensure    *EnsurePlan    `json:"ensure,omitempty"`

	Try     *TryPlan     `json:"try,omitempty"`
	Timeout *TimeoutPlan `json:"timeout,omitempty"`
	Retry   *RetryPlan   `json:"retry,omitempty"`

	// used for 'fly execute'
	ArtifactInput  *ArtifactInputPlan  `json:"artifact_input,omitempty"`
	ArtifactOutput *ArtifactOutputPlan `json:"artifact_output,omitempty"`

	// deprecated, kept for backwards compatibility to be able to show old builds
	DependentGet *DependentGetPlan `json:"dependent_get,omitempty"`
}

func (plan *Plan) Each(f func(*Plan)) {
	f(plan)

	if plan.Do != nil {
		for i, p := range *plan.Do {
			p.Each(f)
			(*plan.Do)[i] = p
		}
	}

	if plan.InParallel != nil {
		for i, p := range plan.InParallel.Steps {
			p.Each(f)
			plan.InParallel.Steps[i] = p
		}
	}

	if plan.Across != nil {
		for i, p := range plan.Across.Steps {
			p.Step.Each(f)
			plan.Across.Steps[i] = p
		}
	}

	if plan.OnSuccess != nil {
		plan.OnSuccess.Step.Each(f)
		plan.OnSuccess.Next.Each(f)
	}

	if plan.OnFailure != nil {
		plan.OnFailure.Step.Each(f)
		plan.OnFailure.Next.Each(f)
	}

	if plan.OnAbort != nil {
		plan.OnAbort.Step.Each(f)
		plan.OnAbort.Next.Each(f)
	}

	if plan.OnError != nil {
		plan.OnError.Step.Each(f)
		plan.OnError.Next.Each(f)
	}

	if plan.Ensure != nil {
		plan.Ensure.Step.Each(f)
		plan.Ensure.Next.Each(f)
	}

	if plan.Try != nil {
		plan.Try.Step.Each(f)
	}

	if plan.Timeout != nil {
		plan.Timeout.Step.Each(f)
	}

	if plan.Retry != nil {
		for i, p := range *plan.Retry {
			p.Each(f)
			(*plan.Retry)[i] = p
		}
	}
}

type PlanID string

func (id PlanID) String() string {
	return string(id)
}

type ArtifactInputPlan struct {
	ArtifactID int    `json:"artifact_id"`
	Name       string `json:"name"`
}

type ArtifactOutputPlan struct {
	Name string `json:"name"`
}

type OnAbortPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_abort"`
}

type OnErrorPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_error"`
}

type OnFailurePlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_failure"`
}

type EnsurePlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"ensure"`
}

type OnSuccessPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_success"`
}

type TimeoutPlan struct {
	Step     Plan   `json:"step"`
	Duration string `json:"duration"`
}

type TryPlan struct {
	Step Plan `json:"step"`
}

type InParallelPlan struct {
	Steps    []Plan `json:"steps"`
	Limit    int    `json:"limit,omitempty"`
	FailFast bool   `json:"fail_fast,omitempty"`
}

type AcrossPlan struct {
	Vars     []AcrossVar     `json:"vars"`
	Steps    []VarScopedPlan `json:"steps"`
	FailFast bool            `json:"fail_fast,omitempty"`
}

type AcrossVar struct {
	Var         string             `json:"name"`
	Values      []interface{}      `json:"values"`
	MaxInFlight *MaxInFlightConfig `json:"max_in_flight,omitempty"`
}

type VarScopedPlan struct {
	Step   Plan          `json:"step"`
	Values []interface{} `json:"values"`
}

type DoPlan []Plan

type GetPlan struct {
	// The name of the step.
	Name string `json:"name,omitempty"`

	// The resource config to fetch from.
	Type                   string                 `json:"type"`
	Source                 Source                 `json:"source"`
	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`

	// Image of the container. One of these must be specified.
	ImageCheckPlan *Plan `json:"image_check_plan,omitempty"`
	ImageGetPlan   *Plan `json:"image_get_plan,omitempty"`

	// TODO:
	BaseType string `json:"base_type,omitempty"`

	// The version of the resource to fetch. One of these must be specified.
	Version     *Version `json:"version,omitempty"`
	VersionFrom *PlanID  `json:"version_from,omitempty"`

	// Params to pass to the get operation.
	Params Params `json:"params,omitempty"`

	// A pipeline resource to update with metadata.
	Resource string `json:"resource,omitempty"`

	// Worker tags to influence placement of the container.
	Tags Tags `json:"tags,omitempty"`

	// A timeout to enforce on the resource `get` process. Note that fetching the
	// resource's image does not count towards the timeout.
	Timeout string `json:"timeout,omitempty"`

	// Privileged indicates whether the parent resource type is privileged.
	Privileged bool `json:"privileged,omitempty"`
}

type PutPlan struct {
	// The name of the step.
	Name string `json:"name"`

	// The resource config to push to.
	Type                   string                 `json:"type"`
	Source                 Source                 `json:"source"`
	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`

	// TODO:
	BaseType string `json:"base_type,omitempty"`

	// Image of the container. One of these must be specified.
	ImageCheckPlan *Plan `json:"image_check_plan,omitempty"`
	ImageGetPlan   *Plan `json:"image_get_plan,omitempty"`

	// Params to pass to the put operation.
	Params Params `json:"params,omitempty"`

	// Inputs to pass to the put operation.
	Inputs *InputsConfig `json:"inputs,omitempty"`

	// A pipeline resource to save the versions onto.
	Resource string `json:"resource,omitempty"`

	// Worker tags to influence placement of the container.
	Tags Tags `json:"tags,omitempty"`

	// A timeout to enforce on the resource `put` process. Note that fetching the
	// resource's image does not count towards the timeout.
	Timeout string `json:"timeout,omitempty"`

	// If or not expose BUILD_CREATED_BY to build metadata
	ExposeBuildCreatedBy bool `json:"expose_build_created_by,omitempty"`

	// Privileged indicates whether the parent resource type is privileged.
	Privileged bool `json:"privileged,omitempty"`
}

type CheckPlan struct {
	// The name of the step.
	Name string `json:"name"`

	// The resource config to check.
	Type                   string                 `json:"type"`
	Source                 Source                 `json:"source"`
	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`

	// TODO:
	BaseType string `json:"base_type,omitempty"`

	// Image of the container. One of these must be specified.
	ImageCheckPlan *Plan `json:"image_check_plan,omitempty"`
	ImageGetPlan   *Plan `json:"image_get_plan,omitempty"`

	// The version to check from. If not specified, defaults to the latest
	// version of the config.
	FromVersion Version `json:"from_version,omitempty"`

	// A pipeline resource or resource type to assign the config to.
	Resource     string `json:"resource,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`

	// The interval on which to check - if it has not elapsed since the config
	// was last checked, and the build has not been manually triggered, the check
	// will be skipped.
	Interval string `json:"interval,omitempty"`

	// A timeout to enforce on the resource `check` process. Note that fetching
	// the resource's image does not count towards the timeout.
	Timeout string `json:"timeout,omitempty"`

	// Worker tags to influence placement of the container.
	Tags Tags `json:"tags,omitempty"`

	// Privileged indicates whether the parent resource type is privileged.
	Privileged bool `json:"privileged,omitempty"`
}

func (plan CheckPlan) IsPeriodic() bool {
	return plan.Resource != "" || plan.ResourceType != ""
}

type TaskPlan struct {
	// The name of the step.
	Name string `json:"name"`

	// Run the task in 'privileged' mode. What this means depends on the
	// platform, but typically you expose your workers to more risk by enabling
	// this.
	Privileged bool `json:"privileged"`

	// Worker tags to influence placement of the container.
	Tags Tags `json:"tags,omitempty"`

	// The task config to execute - either fetched from a path at runtime, or
	// provided statically.
	ConfigPath string      `json:"config_path,omitempty"`
	Config     *TaskConfig `json:"config,omitempty"`

	// Limits to set on the Task Container
	Limits *ContainerLimits `json:"container_limits,omitempty"`

	// An artifact in the build plan to use as the task's image. Overrides any
	// image set in the task's config.
	ImageArtifactName string `json:"image,omitempty"`

	// Vars to use to parameterize the task config.
	Vars Params `json:"vars,omitempty"`

	// Params to set in the task's environment.
	Params TaskEnv `json:"params,omitempty"`

	// Remap inputs and output artifacts from task names to other names in the
	// build plan.
	InputMapping  map[string]string `json:"input_mapping,omitempty"`
	OutputMapping map[string]string `json:"output_mapping,omitempty"`

	// A timeout to enforce on the task's process. Note that etching the task's
	// image does not count towards the timeout.
	Timeout string `json:"timeout,omitempty"`

	// Resource types to have available for use when fetching the task's image.
	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type SetPipelinePlan struct {
	Name         string                 `json:"name"`
	File         string                 `json:"file"`
	Team         string                 `json:"team,omitempty"`
	Vars         map[string]interface{} `json:"vars,omitempty"`
	VarFiles     []string               `json:"var_files,omitempty"`
	InstanceVars map[string]interface{} `json:"instance_vars,omitempty"`
}

type LoadVarPlan struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Format string `json:"format,omitempty"`
	Reveal bool   `json:"reveal,omitempty"`
}

type RetryPlan []Plan

type DependentGetPlan struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Resource string `json:"resource"`
}
