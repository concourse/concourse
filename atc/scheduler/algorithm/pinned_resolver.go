package algorithm

import "github.com/concourse/concourse/atc/db"

type pinnedResolver struct {
	vdb         db.VersionsDB
	inputConfig InputConfig

	debug debugger
}

func NewPinnedResolver(vdb db.VersionsDB, inputConfig InputConfig) Resolver {
	return &pinnedResolver{
		vdb:         vdb,
		inputConfig: inputConfig,
		debug:       debugger{},
	}
}

func (r *pinnedResolver) InputConfigs() InputConfigs {
	return InputConfigs{r.inputConfig}
}

func (r *pinnedResolver) Resolve(depth int) (map[string]*versionCandidate, db.ResolutionFailure, error) {
	version, found, err := r.vdb.FindVersionOfResource(r.inputConfig.ResourceID, r.inputConfig.PinnedVersion)
	if err != nil {
		return nil, "", err
	}

	if !found {
		return nil, db.PinnedVersionNotFound{PinnedVersion: r.inputConfig.PinnedVersion}.String(), nil
	}

	versionCandidate := map[string]*versionCandidate{
		r.inputConfig.Name: newCandidateVersion(version),
	}

	return versionCandidate, "", nil
}
