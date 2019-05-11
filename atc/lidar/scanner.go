package lidar

// func (c *scanner) tryScan(ctx context.Context, resource db.Resource) error {

// 	resourceTypes, err := resource.ResourceTypes()
// 	if err != nil {
// 		c.logger.Error("failed-to-fetch-resource-types", err)
// 		return err
// 	}

// 	variables := creds.NewVariables(c.secrets, resource.PipelineName(), resource.TeamName())

// 	source, err := creds.NewSource(variables, resource.Source()).Evaluate()
// 	if err != nil {
// 		c.logger.Error("failed-to-evaluate-source", err)
// 		return err
// 	}

// 	versionedResourceTypes := creds.NewVersionedResourceTypes(variables, resourceTypes.Deserialize())

// 	// This could have changed based on new variable interpolation so update it
// 	resourceConfigScope, err := resource.SetResourceConfig(source, versionedResourceTypes)
// 	if err != nil {
// 		c.logger.Error("failed-to-update-resource-config", err)
// 		return err
// 	}

// }
