package algorithm

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type InputConfigs []InputConfig

type InputConfig struct {
	Name            string
	Passed          db.JobSet
	UseEveryVersion bool
	PinnedVersion   db.ResourceVersion
	ResourceID      int
	JobID           int
}

//go:generate counterfeiter . InputMapper

type InputMapper interface {
	MapInputs(*db.VersionsDB, db.Job, db.Resources) (db.InputMapping, bool, error)
}

// NOTE: we're effectively ignoring check_order here and relying on
// build history - be careful when doing #413 that we don't go
// 'back in time'
//
// QUESTION: is version_md5 worth it? might it be surprising
// that all it takes is (resource name, version identifier)
// to consider something 'passed'?

var ErrLatestVersionNotFound = errors.New("latest version of resource not found")
var ErrVersionNotFound = errors.New("version of resource not found")

type PinnedVersionNotFoundError struct {
	PinnedVersion atc.Version
}

func (e PinnedVersionNotFoundError) Error() string {
	var text string
	for k, v := range e.PinnedVersion {
		text += fmt.Sprintf(" %s:%s", k, v)
	}
	return fmt.Sprintf("pinned version%s not found", text)
}

type NoSatisfiableBuildsForPassedJobError struct {
	JobName string
}

func (e NoSatisfiableBuildsForPassedJobError) Error() string {
	return fmt.Sprintf("passed job '%s' does not have a build that satisfies the constraints", e.JobName)
}

type versionCandidate struct {
	Version        db.ResourceVersion
	VouchedForBy   map[int]bool
	SourceBuildIds []int
	ResolveError   error
}

func newCandidateVersion(version db.ResourceVersion) *versionCandidate {
	return &versionCandidate{
		Version:        version,
		VouchedForBy:   map[int]bool{},
		SourceBuildIds: []int{},
		ResolveError:   nil,
	}
}

func newCandidateError(err error) *versionCandidate {
	return &versionCandidate{
		Version:        db.ResourceVersion{},
		VouchedForBy:   map[int]bool{},
		SourceBuildIds: []int{},
		ResolveError:   err,
	}
}

func NewInputMapper() *inputMapper {
	return &inputMapper{}
}

type inputMapper struct{}

func (im *inputMapper) MapInputs(
	versions *db.VersionsDB,
	job db.Job,
	resources db.Resources,
) (db.InputMapping, bool, error) {
	inputConfigs := InputConfigs{}

	for _, input := range job.Config().Inputs() {
		resource, found := resources.Lookup(input.Resource)
		if !found {
			return nil, false, errors.New("input resource not found")
		}

		jobs := db.JobSet{}
		for _, passedJobName := range input.Passed {
			jobs[versions.JobIDs[passedJobName]] = true
		}

		inputConfig := InputConfig{
			Name:       input.Name,
			ResourceID: versions.ResourceIDs[input.Resource],
			Passed:     jobs,
			JobID:      job.ID(),
		}

		var pinnedVersion atc.Version
		if resource.CurrentPinnedVersion() != nil {
			pinnedVersion = resource.CurrentPinnedVersion()
		}

		if input.Version != nil {
			inputConfig.UseEveryVersion = input.Version.Every

			if input.Version.Pinned != nil {
				pinnedVersion = input.Version.Pinned
			}
		}

		if pinnedVersion != nil {
			version, found, err := versions.FindVersionOfResource(inputConfig.ResourceID, pinnedVersion)
			if err != nil {
				return nil, false, err
			}

			if !found {
				return nil, false, PinnedVersionNotFoundError{pinnedVersion}
			}

			inputConfig.PinnedVersion = version
		}

		inputConfigs = append(inputConfigs, inputConfig)
	}

	return im.computeNextInputs(inputConfigs, versions, job.ID())
}

func (im *inputMapper) computeNextInputs(configs InputConfigs, versionsDB *db.VersionsDB, currentJobID int) (db.InputMapping, bool, error) {
	mapping := db.InputMapping{}

	candidates, err := im.resolve(versionsDB, configs)
	if err != nil {
		return nil, false, err
	}

	latestBuildID, found, err := versionsDB.LatestBuildID(currentJobID)
	if err != nil {
		return nil, false, err
	}

	outputs, err := versionsDB.BuildOutputs(latestBuildID)
	if err != nil {
		return nil, false, err
	}

	latestBuildOutputs := map[string]db.ResourceVersion{}
	for _, o := range outputs {
		latestBuildOutputs[o.InputName] = o.Version
	}

	valid := true
	for i, config := range configs {
		inputResult := db.InputResult{}

		if candidates[i] == nil {
			inputResult.ResolveSkipped = true
			valid = false

		} else if candidates[i].ResolveError != nil {
			inputResult.ResolveError = candidates[i].ResolveError
			valid = false

		} else {
			inputResult = db.InputResult{
				Input: &db.AlgorithmInput{
					AlgorithmVersion: db.AlgorithmVersion{
						ResourceID: config.ResourceID,
						Version:    candidates[i].Version,
					},
					FirstOccurrence: !found || latestBuildOutputs[config.Name].ID != candidates[i].Version.ID,
				},
				PassedBuildIDs: candidates[i].SourceBuildIds,
			}
		}

		mapping[config.Name] = inputResult
	}

	return mapping, valid, nil
}

func (im *inputMapper) resolve(vdb *db.VersionsDB, inputConfigs InputConfigs) ([]*versionCandidate, error) {
	candidates := make([]*versionCandidate, len(inputConfigs))
	unresolvedCandidates := make([]*versionCandidate, len(inputConfigs))

	resolved, err := im.tryResolve(0, vdb, inputConfigs, candidates, unresolvedCandidates)
	if err != nil {
		return nil, err
	}

	if !resolved {
		return unresolvedCandidates, nil
	}

	return candidates, nil
}

func (im *inputMapper) tryResolve(depth int, vdb *db.VersionsDB, inputConfigs InputConfigs, candidates []*versionCandidate, unresolvedCandidates []*versionCandidate) (bool, error) {
	// NOTE: this is probably made most efficient by doing it in order of inputs
	// with jobs that have the broadest output sets, so that we can pin the most
	// at once
	//
	// NOTE 3: maybe also select distinct build outputs so we don't waste time on
	// the same thing (i.e. constantly re-triggered build)
	//
	// NOTE : make sure everything is deterministically ordered

	for i, inputConfig := range inputConfigs {
		debug := func(messages ...interface{}) {
			// 	log.Println(
			// 		append(
			// 			[]interface{}{
			// 				strings.Repeat("-", depth) + fmt.Sprintf("[%s]", inputConfig.Name),
			// 			},
			// 			messages...,
			// 		)...,
			// 	)
		}

		if len(inputConfig.Passed) == 0 {
			// coming from recursive call; already set to the latest version
			if candidates[i] != nil {
				continue
			}

			var version db.ResourceVersion
			if inputConfig.PinnedVersion.ID != 0 {
				// pinned
				version = inputConfig.PinnedVersion
				debug("setting candidate", i, "to unconstrained version", version.ID)
			} else if inputConfig.UseEveryVersion {
				buildID, found, err := vdb.LatestBuildID(inputConfig.JobID)
				if err != nil {
					return false, err
				}

				if found {
					version, found, err = vdb.NextEveryVersion(buildID, inputConfig.ResourceID)
					if err != nil {
						return false, err
					}

					if !found {
						unresolvedCandidates[i] = newCandidateError(ErrVersionNotFound)
						return false, nil
					}
				} else {
					version, found, err = vdb.LatestVersionOfResource(inputConfig.ResourceID)
					if err != nil {
						return false, err
					}

					if !found {
						unresolvedCandidates[i] = newCandidateError(ErrLatestVersionNotFound)
						return false, nil
					}
				}

				debug("setting candidate", i, "to version for version every", version.ID, " resource ", inputConfig.ResourceID)
			} else {
				// there are no passed constraints, so just take the latest version
				var err error
				var found bool
				version, found, err = vdb.LatestVersionOfResource(inputConfig.ResourceID)
				if err != nil {
					return false, nil
				}

				if !found {
					unresolvedCandidates[i] = newCandidateError(ErrLatestVersionNotFound)
					return false, nil
				}

				debug("setting candidate", i, "to version for latest", version.ID)
			}

			candidates[i] = newCandidateVersion(version)
			unresolvedCandidates[i] = newCandidateVersion(version)
			continue
		}

		orderedJobs := []int{}
		if len(inputConfig.Passed) != 0 {
			var err error
			orderedJobs, err = vdb.OrderPassedJobs(inputConfig.JobID, inputConfig.Passed)
			if err != nil {
				return false, err
			}
		}

		for _, jobID := range orderedJobs {
			if candidates[i] != nil {
				debug(i, "has a candidate")

				// coming from recursive call; we've already got a candidate
				if candidates[i].VouchedForBy[jobID] {
					debug("job", jobID, i, "already vouched for", candidates[i].Version.ID)
					// we've already been here; continue to the next job
					continue
				} else {
					debug("job", jobID, i, "has not vouched for", candidates[i].Version.ID)
				}
			} else {
				debug(i, "has no candidate yet")
			}

			// loop over previous output sets, latest first
			var builds []int

			if inputConfig.UseEveryVersion {
				buildID, found, err := vdb.LatestBuildID(inputConfig.JobID)
				if err != nil {
					return false, err
				}

				if found {
					constraintBuildID, found, err := vdb.LatestConstraintBuildID(buildID, jobID)
					if err != nil {
						return false, err
					}

					if found {
						if candidates[i] != nil {
							builds, err = vdb.UnusedBuildsVersionConstrained(constraintBuildID, jobID, candidates[i].Version)
						} else {
							builds, err = vdb.UnusedBuilds(constraintBuildID, jobID)
						}
						if err != nil {
							return false, err
						}
					}
				}
			}

			var err error
			if len(builds) == 0 {
				if candidates[i] != nil {
					builds, err = vdb.SuccessfulBuildsVersionConstrained(jobID, candidates[i].Version)
					debug("found", len(builds), "builds for candidate", candidates[i].Version.ID)
				} else {
					builds, err = vdb.SuccessfulBuilds(jobID)
					debug("found", len(builds), "builds no candidate")
				}
				if err != nil {
					return false, err
				}
			}

			for _, buildID := range builds {
				outputs, err := vdb.BuildOutputs(buildID)
				if err != nil {
					return false, err
				}

				debug("job", jobID, "trying build", jobID, buildID)

				restore := map[int]*versionCandidate{}

				var mismatch bool

				// loop over the resource versions that came out of this build set
			outputs:
				for _, output := range outputs {
					debug("build", buildID, "output", output.ResourceID, output.Version.ID)

					// try to pin each candidate to the versions from this build
					for c, candidate := range candidates {
						if inputConfigs[c].ResourceID != output.ResourceID {
							// unrelated to this output
							continue
						}

						if !inputConfigs[c].Passed[jobID] {
							// this candidate is unaffected by the current job
							debug("independent", inputConfigs[c].Passed, jobID)
							continue
						}

						if vdb.DisabledVersionIDs[output.Version.ID] {
							debug("disabled", output.Version.ID, jobID)
							mismatch = true
							break outputs
						}

						if inputConfigs[c].PinnedVersion.ID != 0 && inputConfigs[c].PinnedVersion.ID != output.Version.ID {
							debug("mismatch pinned version", output.Version.ID, jobID)
							mismatch = true
							break outputs
						}

						if candidate != nil && candidate.Version.ID != output.Version.ID {
							// don't return here! just try the next output set. it's possible
							// we just need to use an older output set.
							debug("mismatch")
							mismatch = true
							break outputs
						}

						// if this doesn't work out, restore it to either nil or the
						// candidate *without* the job vouching for it
						if candidate == nil {
							restore[c] = candidate

							debug("setting candidate", c, "to", output.Version.ID)
							candidates[c] = newCandidateVersion(output.Version)
						}

						debug("job", jobID, "vouching for", output.ResourceID, "version", output.Version.ID)
						candidates[c].VouchedForBy[jobID] = true
						candidates[c].SourceBuildIds = append(candidates[c].SourceBuildIds, buildID)

						allVouchedFor := true
						for passedJobID, _ := range inputConfigs[c].Passed {
							allVouchedFor = allVouchedFor && candidates[c].VouchedForBy[passedJobID]
						}

						if allVouchedFor && (unresolvedCandidates[i] == nil || (unresolvedCandidates[i] != nil && unresolvedCandidates[i].ResolveError != nil)) {
							unresolvedCandidates[i] = newCandidateVersion(output.Version)
						}
					}
				}

				// we found a candidate for ourselves and the rest are OK too - recurse
				if candidates[i] != nil && candidates[i].VouchedForBy[jobID] && !mismatch {
					debug("recursing")

					resolved, err := im.tryResolve(depth+1, vdb, inputConfigs, candidates, unresolvedCandidates)
					if err != nil {
						return false, err
					}

					if resolved {
						// we've attempted to resolve all of the inputs!
						return true, nil
					}
				}

				debug("restoring")

				for c, candidate := range restore {
					// either there was a mismatch or resolving didn't work; go on to the
					// next output set
					debug("restoring candidate", c, "to", candidate)
					candidates[c] = candidate
				}
			}

			// we've exhausted all the builds and never found a matching input set;
			// give up on this input
			if unresolvedCandidates[i] == nil {
				var jobName string
				for jName, jID := range vdb.JobIDs {
					if jID == jobID {
						jobName = jName
					}
				}
				unresolvedCandidates[i] = newCandidateError(NoSatisfiableBuildsForPassedJobError{jobName})
			}
			return false, nil
		}
	}

	// go to the end of all the inputs
	return true, nil
}
