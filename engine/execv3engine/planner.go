package execv3engine

import "github.com/concourse/atc"

type Planner struct {
}

func (planner *Planner) Generate(plan atc.Plan) Plan {
	p := Plan{}

	if plan.Get != nil {
		var rootFSSource RootFSSource

		_, customResourceTypeFound := plan.Get.VersionedResourceTypes.Lookup(plan.Get.Type)
		if customResourceTypeFound {
			rootFSSource = RootFSSource{
				Output: &OutputRootFSSource{
					Name: plan.Get.Type, // TODO: randomly generate
				},
			}
		} else {
			rootFSSource = RootFSSource{
				Base: &BaseResourceTypeRootFSSource{
					Name: plan.Get.Type,
				},
			}
		}

		p.Get = &GetPlan{
			Type:         plan.Get.Type,
			Name:         plan.Get.Name,
			Resource:     plan.Get.Resource,
			Source:       plan.Get.Source,
			Params:       plan.Get.Params,
			Version:      plan.Get.Version,
			Tags:         plan.Get.Tags,
			RootFSSource: rootFSSource,
			Outputs:      []string{plan.Get.Name},
		}
	}

	return p
}
