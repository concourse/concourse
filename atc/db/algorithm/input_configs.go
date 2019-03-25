package algorithm

type InputConfigs []InputConfig

type InputConfig struct {
	Name            string
	JobName         string
	Passed          JobSet
	UseEveryVersion bool
	PinnedVersionID int
	ResourceID      int
	JobID           int
}

func (configs InputConfigs) Resolve(db *VersionsDB) (InputMapping, bool, error) {
	mapping := InputMapping{}

	versions, ok, err := Resolve(db, configs)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return nil, false, nil
	}

	for i, v := range versions {
		mapping[configs[i].Name] = InputVersion{
			ResourceID:      configs[i].ResourceID,
			VersionID:       v.ID,
			FirstOccurrence: false, // TODO
		}
	}

	return mapping, true, nil
}

// func (configs InputConfigs) Resolve(db *VersionsDB) (InputMapping, bool, error) {
// jobs := JobSet{}
// inputCandidates := InputCandidates{}

// for _, inputConfig := range configs {
// 	var versionCandidates VersionCandidates
// 	var err error

// 	if len(inputConfig.Passed) == 0 {
// 		if inputConfig.PinnedVersionID != 0 {
// 			versionCandidates, err = db.FindVersionOfResource(inputConfig.ResourceID, inputConfig.PinnedVersionID)
// 		} else {
// 			versionCandidates, err = db.AllVersionsOfResource(inputConfig.ResourceID)
// 		}
// 	} else {
// 		jobs = jobs.Union(inputConfig.Passed)

// 		versionCandidates, err = db.VersionsOfResourcePassedJobs(
// 			inputConfig.ResourceID,
// 			inputConfig.Passed,
// 		)
// 	}
// 	if err != nil {
// 		return nil, false, err
// 	}

// 	if inputConfig.UseEveryVersion {
// 		versionCandidates, err = versionCandidates.ConsecutiveVersions(inputConfig.JobID, inputConfig.ResourceID)
// 		if err != nil {
// 			return nil, false, err
// 		}
// 	}

// 	inputCandidates = append(inputCandidates, InputVersionCandidates{
// 		Input:             inputConfig.Name,
// 		Passed:            inputConfig.Passed,
// 		UseEveryVersion:   inputConfig.UseEveryVersion,
// 		PinnedVersionID:   inputConfig.PinnedVersionID,
// 		VersionCandidates: versionCandidates,
// 	})
// }

// basicMapping, ok, err := inputCandidates.Reduce(0, jobs)
// if err != nil {
// 	return nil, false, err
// }

// if !ok {
// 	return nil, false, nil
// }

// mapping := InputMapping{}
// for _, inputConfig := range configs {
// 	inputName := inputConfig.Name
// 	inputVersionID := basicMapping[inputName]
// 	firstOccurrence, err := db.IsVersionFirstOccurrence(inputVersionID, inputConfig.JobID, inputName)
// 	if err != nil {
// 		return nil, false, err
// 	}

// 	mapping[inputName] = InputVersion{
// 		ResourceID:      inputConfig.ResourceID,
// 		VersionID:       inputVersionID,
// 		FirstOccurrence: firstOccurrence,
// 	}
// }

// return mapping, true, nil
// }
