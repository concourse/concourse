package execv3engine_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/engine/execv3engine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Planner", func() {
	var planner execv3engine.Planner

	BeforeEach(func() {
		planner = execv3engine.Planner{}
	})

	Context("when atc plan has get step", func() {
		It("generates execution plan with get step parameters and name as an output with rootfs source", func() {
			plan := atc.Plan{
				Get: &atc.GetPlan{
					Type:     "git",
					Name:     "my-resource-name",
					Resource: "my-resource",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Version:  atc.Version{"some": "version"},
					Tags:     atc.Tags{"some", "tags"},
				},
			}

			execPlan := planner.Generate(plan)
			Expect(execPlan).To(Equal(execv3engine.Plan{
				Get: &execv3engine.GetPlan{
					Type:     "git",
					Name:     "my-resource-name",
					Resource: "my-resource",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Version:  atc.Version{"some": "version"},
					Tags:     atc.Tags{"some", "tags"},
					Outputs:  []string{"my-resource-name"},
					RootFSSource: execv3engine.RootFSSource{
						Base: &execv3engine.BaseResourceTypeRootFSSource{
							Name: "git",
						},
					},
				},
			}))
		})

		Context("when the resource type is in the list of custom resource types", func() {
			It("generates output resource type rootfs source", func() {
				plan := atc.Plan{
					Get: &atc.GetPlan{
						Type:     "git",
						Name:     "my-resource-name",
						Resource: "my-resource",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
						Version:  atc.Version{"some": "version"},
						Tags:     atc.Tags{"some", "tags"},
						VersionedResourceTypes: atc.VersionedResourceTypes{
							{
								ResourceType: atc.ResourceType{Name: "git"},
							},
						},
					},
				}

				execPlan := planner.Generate(plan)
				Expect(execPlan).To(Equal(execv3engine.Plan{
					Get: &execv3engine.GetPlan{
						Type:     "git",
						Name:     "my-resource-name",
						Resource: "my-resource",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
						Version:  atc.Version{"some": "version"},
						Tags:     atc.Tags{"some", "tags"},
						Outputs:  []string{"my-resource-name"},
						RootFSSource: execv3engine.RootFSSource{
							Output: &execv3engine.OutputRootFSSource{
								Name: "git",
							},
						},
					},
				}))
			})
		})
	})
})
