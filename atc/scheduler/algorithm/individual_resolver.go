package algorithm

import (
	"github.com/concourse/concourse/atc/db"
)

type individualResolver struct {
	vdb         *db.VersionsDB
	inputConfig InputConfig

	debug debugger
}

func NewIndividualResolver(vdb *db.VersionsDB, inputConfig InputConfig) Resolver {
	return &individualResolver{
		vdb:         vdb,
		inputConfig: inputConfig,
		debug:       debugger{},
	}
}

func (r *individualResolver) InputConfigs() InputConfigs {
	return InputConfigs{r.inputConfig}
}

// Handles the three different configurations of a resource without passed
// constraints: pinned, every and latest
func (r *individualResolver) Resolve(depth int) (map[string]*versionCandidate, db.ResolutionFailure, error) {
	r.debug.reset(0, r.inputConfig.Name)

	var version db.ResourceVersion
	if r.inputConfig.UseEveryVersion {
		buildID, found, err := r.vdb.LatestBuildID(r.inputConfig.JobID)
		if err != nil {
			return nil, "", err
		}

		if found {
			version, found, err = r.vdb.NextEveryVersion(buildID, r.inputConfig.ResourceID)
			if err != nil {
				return nil, "", err
			}

			if !found {
				return nil, db.VersionNotFound, nil
			}
		} else {
			version, found, err = r.vdb.LatestVersionOfResource(r.inputConfig.ResourceID)
			if err != nil {
				return nil, "", err
			}

			if !found {
				return nil, db.LatestVersionNotFound, nil
			}
		}

		r.debug.log("setting candidate", r.inputConfig.Name, "to version for version every", version, " resource ", r.inputConfig.ResourceID)
	} else {
		// there are no passed constraints, so just take the latest version
		var err error
		var found bool
		version, found, err = r.vdb.LatestVersionOfResource(r.inputConfig.ResourceID)
		if err != nil {
			return nil, "", err
		}

		if !found {
			return nil, db.LatestVersionNotFound, nil
		}

		r.debug.log("setting candidate", r.inputConfig.Name, "to version for latest", version)
	}

	versionCandidate := map[string]*versionCandidate{
		r.inputConfig.Name: newCandidateVersion(version),
	}

	return versionCandidate, "", nil
}

type debugger struct {
	depth     int
	inputName string
}

func (d *debugger) reset(depth int, inputName string) {
	d.depth = depth
	d.inputName = inputName
}

func (d *debugger) log(messages ...interface{}) {
	// log.Println(
	// append(
	// 	[]interface{}{
	// 		strings.Repeat("-", d.depth) + fmt.Sprintf("[%s]", d.inputName),
	// 	},
	// 	messages...,
	// )...,
	// )
}
