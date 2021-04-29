package atc

type VersionedResourceType struct {
	ResourceType

	Version Version `json:"version"`
}

type VersionedResourceTypes []VersionedResourceType

func (types VersionedResourceTypes) Lookup(name string) (VersionedResourceType, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return VersionedResourceType{}, false
}

func (types VersionedResourceTypes) Without(name string) VersionedResourceTypes {
	newTypes := VersionedResourceTypes{}
	for _, t := range types {
		if t.Name != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

func (types VersionedResourceTypes) Base(name string) string {
	base := name
	for {
		resourceType, found := types.Lookup(base)
		if !found {
			break
		}

		types = types.Without(base)
		base = resourceType.Type
	}

	return base
}

func (types VersionedResourceTypes) ImageForType(planID PlanID, resourceType string, stepTags Tags) TypeImage {
	// Check if resource type is a custom type
	parent, found := types.Lookup(resourceType)
	if !found {
		// If it is not a custom type, return back the image as a base type
		return TypeImage{
			BaseType: resourceType,
		}
	}

	tags := parent.Tags
	if len(parent.Tags) == 0 {
		tags = stepTags
	}

	checkPlan := types.createImageCheckPlan(planID, parent, tags)
	getPlan := types.createImageGetPlan(planID, parent, tags, &checkPlan.ID)

	return TypeImage{
		// Set the base type as the base type of its parent. The value of the base
		// type will always be the base type at the bottom of the dependency chain.
		//
		// For example, if there is a resource that depends on a custom type that
		// depends on a git base resource type, the BaseType value of the resource's
		// TypeImage will be git.
		BaseType: getPlan.Get.TypeImage.BaseType,

		Privileged: parent.Privileged,

		// GetPlan for fetching the custom type's image and CheckPlan
		// for checking the version of the custom type.
		GetPlan:   getPlan,
		CheckPlan: checkPlan,
	}
}

func (types VersionedResourceTypes) createImageCheckPlan(planID PlanID, parent VersionedResourceType, tags Tags) *Plan {
	checkPlanID := planID + "/image-check"
	return &Plan{
		ID: checkPlanID,
		Check: &CheckPlan{
			Name:      parent.Name,
			Type:      parent.Type,
			Source:    parent.Source,
			TypeImage: types.Without(parent.Name).ImageForType(checkPlanID, parent.Type, tags),
			Tags:      tags,
		},
	}
}

func (types VersionedResourceTypes) createImageGetPlan(planID PlanID, parent VersionedResourceType, tags Tags, checkPlanID *PlanID) *Plan {
	getPlanID := planID + "/image-get"
	return &Plan{
		ID: getPlanID,
		Get: &GetPlan{
			Name:        parent.Name,
			Type:        parent.Type,
			Source:      parent.Source,
			Params:      parent.Params,
			VersionFrom: checkPlanID,
			TypeImage:   types.Without(parent.Name).ImageForType(getPlanID, parent.Type, tags),
			Tags:        tags,
		},
	}
}

