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

	// Construct the TypeImage as an image plan with a custom type. This means it
	// will need a GetPlan for fetching the custom type's image.
	getPlanID := planID + "/image-get"
	typeImage := TypeImage{
		Privileged: parent.Privileged,

		GetPlan: &Plan{
			ID: getPlanID,
			Get: &GetPlan{
				Name:   parent.Name,
				Type:   parent.Type,
				Source: parent.Source,
				Params: parent.Params,

				TypeImage: types.Without(resourceType).ImageForType(getPlanID, parent.Type, tags),

				Tags: tags,
			},
		},
	}

	// Set the base type as the base type of its parent. The value of the base
	// type will always be the base type at the bottom of the dependency chain.
	//
	// For example, if there is a resource that depends on a custom type that
	// depends on a git base resource type, the BaseType value of the resource's
	// TypeImage will be git.
	typeImage.BaseType = typeImage.GetPlan.Get.TypeImage.BaseType

	// If the parent resource type does not have a version, include a check plan
	// in the TypeImage
	resourceTypeVersion := parent.Version
	if resourceTypeVersion == nil {
		checkPlanID := planID + "/image-check"

		// don't know the version, need to do a Check before the Get
		typeImage.CheckPlan = &Plan{
			ID: checkPlanID,
			Check: &CheckPlan{
				Name:   parent.Name,
				Type:   parent.Type,
				Source: parent.Source,

				TypeImage: types.Without(resourceType).ImageForType(checkPlanID, parent.Type, tags),

				Tags: tags,
			},
		}

		typeImage.GetPlan.Get.VersionFrom = &typeImage.CheckPlan.ID
	} else {
		// version is already provided, only need to do Get step
		typeImage.GetPlan.Get.Version = &resourceTypeVersion
	}

	return typeImage
}
