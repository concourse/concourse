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
	PinnedVersionID int
	ResourceID      int
	JobID           int
}

//go:generate counterfeiter . InputMapper

type InputMapper interface {
	MapInputs(db *db.VersionsDB, job db.Job, resources db.Resources) (db.InputMapping, bool, error)
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

type version struct {
	ID             int
	VouchedForBy   map[int]bool
	SourceBuildIds []int
	ResolveError   error
}

func newCandidateVersion(id int) *version {
	return &version{
		ID:             id,
		VouchedForBy:   map[int]bool{},
		SourceBuildIds: []int{},
		ResolveError:   nil,
	}
}

func newCandidateError(err error) *version {
	return &version{
		ID:             0,
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
			id, found, err := versions.FindVersionOfResource(inputConfig.ResourceID, pinnedVersion)
			if err != nil {
				return nil, false, err
			}

			if !found {
				return nil, false, PinnedVersionNotFoundError{pinnedVersion}
			}

			inputConfig.PinnedVersionID = id
		}

		inputConfigs = append(inputConfigs, inputConfig)
	}

	return im.computeNextInputs(inputConfigs, versions, job.ID())
}

func (im *inputMapper) computeNextInputs(configs InputConfigs, versionsDB *db.VersionsDB, currentJobID int) (db.InputMapping, bool, error) {
	mapping := db.InputMapping{}

	versions, err := im.resolve(versionsDB, configs)
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

	latestBuildOutputs := map[string]int{}
	for _, o := range outputs {
		latestBuildOutputs[o.InputName] = o.VersionID
	}

	valid := true
	for i, config := range configs {
		inputResult := db.InputResult{}

		if versions[i] == nil {
			inputResult.ResolveSkipped = true
			valid = false

		} else if versions[i].ResolveError != nil {
			inputResult.ResolveError = versions[i].ResolveError
			valid = false

		} else {
			inputResult = db.InputResult{
				Input: &db.AlgorithmInput{
					AlgorithmVersion: db.AlgorithmVersion{
						ResourceID: config.ResourceID,
						VersionID:  versions[i].ID,
					},
					FirstOccurrence: !found || latestBuildOutputs[config.Name] != versions[i].ID,
				},
				PassedBuildIDs: versions[i].SourceBuildIds,
			}
		}

		mapping[config.Name] = inputResult
	}

	return mapping, valid, nil
}

func (im *inputMapper) resolve(db *db.VersionsDB, inputConfigs InputConfigs) ([]*version, error) {
	candidates := make([]*version, len(inputConfigs))
	unresolvedCandidates := make([]*version, len(inputConfigs))

	resolved, err := im.tryResolve(0, db, inputConfigs, candidates, unresolvedCandidates)
	if err != nil {
		return nil, err
	}

	if !resolved {
		return unresolvedCandidates, nil
	}

	return candidates, nil
}

func (im *inputMapper) tryResolve(depth int, db *db.VersionsDB, inputConfigs InputConfigs, candidates []*version, unresolvedCandidates []*version) (bool, error) {
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

			var versionID int
			if inputConfig.PinnedVersionID != 0 {
				// pinned
				versionID = inputConfig.PinnedVersionID
				debug("setting candidate", i, "to unconstrained version", versionID)
			} else if inputConfig.UseEveryVersion {
				buildID, found, err := db.LatestBuildID(inputConfig.JobID)
				if err != nil {
					return false, err
				}

				if found {
					versionID, found, err = db.NextEveryVersion(buildID, inputConfig.ResourceID)
					if err != nil {
						return false, err
					}

					if !found {
						unresolvedCandidates[i] = newCandidateError(ErrVersionNotFound)
						return false, nil
					}
				} else {
					versionID, found, err = db.LatestVersionOfResource(inputConfig.ResourceID)
					if err != nil {
						return false, err
					}

					if !found {
						unresolvedCandidates[i] = newCandidateError(ErrLatestVersionNotFound)
						return false, nil
					}
				}

				debug("setting candidate", i, "to version for version every", versionID, " resource ", inputConfig.ResourceID)
			} else {
				// there are no passed constraints, so just take the latest version
				var err error
				var found bool
				versionID, found, err = db.LatestVersionOfResource(inputConfig.ResourceID)
				if err != nil {
					return false, nil
				}

				if !found {
					unresolvedCandidates[i] = newCandidateError(ErrLatestVersionNotFound)
					return false, nil
				}

				debug("setting candidate", i, "to version for latest", versionID)
			}

			candidates[i] = newCandidateVersion(versionID)
			unresolvedCandidates[i] = newCandidateVersion(versionID)
			continue
		}

		orderedJobs := []int{}
		if len(inputConfig.Passed) != 0 {
			var err error
			orderedJobs, err = db.OrderPassedJobs(inputConfig.JobID, inputConfig.Passed)
			if err != nil {
				return false, err
			}
		}

		for _, jobID := range orderedJobs {
			if candidates[i] != nil {
				debug(i, "has a candidate")

				// coming from recursive call; we've already got a candidate
				if candidates[i].VouchedForBy[jobID] {
					debug("job", jobID, i, "already vouched for", candidates[i].ID)
					// we've already been here; continue to the next job
					continue
				} else {
					debug("job", jobID, i, "has not vouched for", candidates[i].ID)
				}
			} else {
				debug(i, "has no candidate yet")
			}

			// loop over previous output sets, latest first
			var builds []int

			if inputConfig.UseEveryVersion {
				buildID, found, err := db.LatestBuildID(inputConfig.JobID)
				if err != nil {
					return false, err
				}

				if found {
					constraintBuildID, found, err := db.LatestConstraintBuildID(buildID, jobID)
					if err != nil {
						return false, err
					}

					if found {
						if candidates[i] != nil {
							builds, err = db.UnusedBuildsVersionConstrained(constraintBuildID, jobID, candidates[i].ID)
						} else {
							builds, err = db.UnusedBuilds(constraintBuildID, jobID)
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
					builds, err = db.SuccessfulBuildsVersionConstrained(jobID, candidates[i].ID)
					debug("found", len(builds), "builds for candidate", candidates[i].ID)
				} else {
					builds, err = db.SuccessfulBuilds(jobID)
					debug("found", len(builds), "builds no candidate")
				}
				if err != nil {
					return false, err
				}
			}

			for _, buildID := range builds {
				outputs, err := db.BuildOutputs(buildID)
				if err != nil {
					return false, err
				}

				debug("job", jobID, "trying build", jobID, buildID)

				restore := map[int]*version{}

				var mismatch bool

				// loop over the resource versions that came out of this build set
			outputs:
				for _, output := range outputs {
					debug("build", buildID, "output", output.ResourceID, output.VersionID)

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

						if db.DisabledVersionIDs[output.VersionID] {
							debug("disabled", output.VersionID, jobID)
							mismatch = true
							break outputs
						}

						if inputConfigs[c].PinnedVersionID != 0 && inputConfigs[c].PinnedVersionID != output.VersionID {
							debug("mismatch pinned version", output.VersionID, jobID)
							mismatch = true
							break outputs
						}

						if candidate != nil && candidate.ID != output.VersionID {
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

							debug("setting candidate", c, "to", output.VersionID)
							candidates[c] = newCandidateVersion(output.VersionID)
						}

						debug("job", jobID, "vouching for", output.ResourceID, "version", output.VersionID)
						candidates[c].VouchedForBy[jobID] = true
						candidates[c].SourceBuildIds = append(candidates[c].SourceBuildIds, buildID)

						allVouchedFor := true
						for passedJobID, _ := range inputConfigs[c].Passed {
							allVouchedFor = allVouchedFor && candidates[c].VouchedForBy[passedJobID]
						}

						if allVouchedFor && (unresolvedCandidates[i] == nil || (unresolvedCandidates[i] != nil && unresolvedCandidates[i].ResolveError != nil)) {
							unresolvedCandidates[i] = newCandidateVersion(output.VersionID)
						}
					}
				}

				// we found a candidate for ourselves and the rest are OK too - recurse
				if candidates[i] != nil && candidates[i].VouchedForBy[jobID] && !mismatch {
					debug("recursing")

					resolved, err := im.tryResolve(depth+1, db, inputConfigs, candidates, unresolvedCandidates)
					if err != nil {
						return false, err
					}

					if resolved {
						// we've attempted to resolve all of the inputs!
						return true, nil
					}
				}

				debug("restoring")

				for c, version := range restore {
					// either there was a mismatch or resolving didn't work; go on to the
					// next output set
					debug("restoring candidate", c, "to", version)
					candidates[c] = version
				}
			}

			// we've exhausted all the builds and never found a matching input set;
			// give up on this input
			if unresolvedCandidates[i] == nil {
				var jobName string
				for jName, jID := range db.JobIDs {
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
