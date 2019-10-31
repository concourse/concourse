package algorithm

import (
	"sort"
	"strconv"

	"github.com/concourse/concourse/atc/db"
)

type versionCandidate struct {
	Version             db.ResourceVersion
	VouchedForBy        map[int]bool
	SourceBuildIds      []int
	HasNextEveryVersion bool
}

func newCandidateVersion(version db.ResourceVersion) *versionCandidate {
	return &versionCandidate{
		Version:        version,
		VouchedForBy:   map[int]bool{},
		SourceBuildIds: []int{},
	}
}

type groupResolver struct {
	vdb          db.VersionsDB
	inputConfigs InputConfigs
	candidates   []*versionCandidate

	lastUsedPassedBuilds map[int]int

	debug debugger
}

func NewGroupResolver(vdb db.VersionsDB, inputConfigs InputConfigs) Resolver {
	return &groupResolver{
		vdb:          vdb,
		inputConfigs: inputConfigs,
		candidates:   make([]*versionCandidate, len(inputConfigs)),
		debug:        debugger{},
	}
}

func (r *groupResolver) InputConfigs() InputConfigs {
	return r.inputConfigs
}

func (r *groupResolver) Resolve(depth int) (map[string]*versionCandidate, db.ResolutionFailure, error) {
	defer func() { r.debug.depth-- }()

	for i, inputConfig := range r.inputConfigs {
		r.debug.reset(depth, inputConfig.Name)

		// deterministically order the passed jobs for this input
		orderedJobs := r.orderJobs(inputConfig.Passed)

		for _, jobID := range orderedJobs {
			if r.candidates[i] != nil {
				r.debug.log(i, "has a candidate")

				// coming from recursive call; we've already got a candidate
				if r.candidates[i].VouchedForBy[jobID] {
					r.debug.log("job", jobID, i, "already vouched for", r.candidates[i].Version)
					// we've already been here; continue to the next job
					continue
				} else {
					r.debug.log("job", jobID, i, "has not vouched for", r.candidates[i].Version)
				}
			} else {
				r.debug.log(i, "has no candidate yet")
			}

			paginatedBuilds, skip, err := r.paginatedBuilds(inputConfig, r.candidates[i], inputConfig.JobID, jobID)
			if err != nil {
				return nil, "", err
			}

			if skip {
				continue
			}

			for {
				buildID, ok, err := paginatedBuilds.Next()
				if err != nil {
					return nil, "", err
				}

				if !ok {
					r.debug.log("reached end")
					break
				}

				r.debug.log("job", jobID, "trying build", buildID)

				outputs, err := r.vdb.SuccessfulBuildOutputs(buildID)
				if err != nil {
					return nil, "", err
				}

				restore := map[int]*versionCandidate{}
				var mismatch bool

				// loop over the resource versions that came out of this build set
			outputs:
				for _, output := range outputs {
					r.debug.log("build", buildID, "output", output.ResourceID, output.Version)

					// try to pin each candidate to the versions from this build
					for c, candidate := range r.candidates {
						if _, ok := restore[c]; ok {
							// have already set a new version for this candidate within this build, so skip
							r.debug.log("have already set this candidate", c, "with a version", candidate.Version)
							continue
						}

						var satisfied bool
						var resolveErr db.ResolutionFailure
						satisfied, mismatch, resolveErr, err = r.outputSatisfyCandidateConstraints(output, r.inputConfigs[c], candidate, jobID)
						if err != nil {
							return nil, "", err
						}

						if resolveErr != "" {
							return nil, resolveErr, nil
						}

						if mismatch {
							// if mismatch is returned, that means this build contained a
							// different version than the one we already have for that
							// candidate, so let's try a different build
							break outputs
						} else if !satisfied {
							continue
						}

						// if this doesn't work out, restore it to either nil or the
						// candidate *without* the job vouching for it
						restore[c] = candidate

						r.debug.log("setting candidate", c, "to", output.Version, "vouched resource", output.ResourceID, "by job", jobID)
						r.candidates[c] = r.vouchForCandidate(candidate, output.Version, jobID, buildID, paginatedBuilds.HasNext())
					}
				}

				// we found a candidate for ourselves and the rest are OK too - recurse
				if r.candidates[i] != nil && r.candidates[i].VouchedForBy[jobID] && !mismatch {
					r.debug.log("recursing")

					candidates, _, err := r.Resolve(depth + 1)
					if err != nil {
						return nil, "", err
					}

					if len(candidates) != 0 {
						// we've attempted to resolve all of the inputs!
						return candidates, "", nil
					}
				}

				r.debug.log("restoring")

				for c, candidate := range restore {
					// either there was a mismatch or resolving didn't work; go on to the
					// next output set
					r.debug.log("restoring candidate", c, "to", candidate)
					r.candidates[c] = candidate
				}
			}

			// reached the end of the builds
			r.debug.log("returning with no satisfiable builds")
			return nil, db.NoSatisfiableBuilds, nil
		}
	}

	finalCandidates := map[string]*versionCandidate{}
	for i, input := range r.inputConfigs {
		finalCandidates[input.Name] = r.candidates[i]
	}

	// go to the end of all the inputs
	return finalCandidates, "", nil
}

func (r *groupResolver) paginatedBuilds(currentInputConfig InputConfig, currentCandidate *versionCandidate, currentJobID int, passedJobID int) (db.PaginatedBuilds, bool, error) {
	constraints := r.constrainingCandidates(passedJobID)

	if currentInputConfig.UseEveryVersion {
		if r.lastUsedPassedBuilds == nil {
			lastUsedBuildIDs := map[int]int{}

			r.debug.log("querying for latest build for job", currentJobID)
			buildID, found, err := r.vdb.LatestBuildID(currentJobID)
			if err != nil {
				return db.PaginatedBuilds{}, false, err
			}

			if found {
				r.debug.log("querying for latest build pipes for job", currentJobID)
				lastUsedBuildIDs, err = r.vdb.LatestBuildPipes(buildID)
				if err != nil {
					return db.PaginatedBuilds{}, false, err
				}

				r.lastUsedPassedBuilds = lastUsedBuildIDs
			}
		}

		relatedPassedBuilds := map[int]int{}
		for jobID, buildID := range r.lastUsedPassedBuilds {
			if currentInputConfig.Passed[jobID] {
				relatedPassedBuilds[jobID] = buildID
			}
		}

		lastUsedBuildID, hasUsedJob := relatedPassedBuilds[passedJobID]
		if hasUsedJob {
			var paginatedBuilds db.PaginatedBuilds
			var err error

			if currentCandidate != nil {
				paginatedBuilds, err = r.vdb.UnusedBuildsVersionConstrained(lastUsedBuildID, passedJobID, constraints)
			} else {
				paginatedBuilds, err = r.vdb.UnusedBuilds(lastUsedBuildID, passedJobID)
			}

			return paginatedBuilds, false, err
		} else if currentCandidate == nil && len(relatedPassedBuilds) > 0 {
			// we've run with version: every and passed: before, just not with this
			// job, and there's no candidate yet, so skip it for now and let the
			// algorithm continue from where the other jobs left off rather than
			// starting from 'latest'
			//
			// this job will eventually vouch for it during the recursive resolve
			// call
			r.debug.log("skipping passed job", passedJobID)
			return db.PaginatedBuilds{}, true, nil
		}
	}

	var paginatedBuilds db.PaginatedBuilds
	var err error
	if currentCandidate != nil {
		paginatedBuilds, err = r.vdb.SuccessfulBuildsVersionConstrained(passedJobID, constraints)
	} else {
		paginatedBuilds = r.vdb.SuccessfulBuilds(passedJobID)
	}

	return paginatedBuilds, false, err
}

func (r *groupResolver) constrainingCandidates(passedJobID int) map[string][]string {
	constrainingCandidates := map[string][]string{}
	for passedIndex, passedInput := range r.inputConfigs {
		if passedInput.Passed[passedJobID] && r.candidates[passedIndex] != nil {
			resID := strconv.Itoa(passedInput.ResourceID)
			constrainingCandidates[resID] = append(constrainingCandidates[resID], string(r.candidates[passedIndex].Version))
		}
	}

	return constrainingCandidates
}

func (r *groupResolver) outputSatisfyCandidateConstraints(output db.AlgorithmVersion, inputConfig InputConfig, candidate *versionCandidate, passedJobID int) (bool, bool, db.ResolutionFailure, error) {
	if inputConfig.ResourceID != output.ResourceID {
		// unrelated to this output because it is a different resource
		return false, false, "", nil
	}

	if !inputConfig.Passed[passedJobID] {
		// this candidate is unaffected by the current job
		r.debug.log("independent", inputConfig.Passed, passedJobID)
		return false, false, "", nil
	}

	disabled, err := r.vdb.VersionIsDisabled(output.ResourceID, output.Version)
	if err != nil {
		return false, false, "", err
	}

	if disabled {
		// this version is disabled so it cannot be used
		r.debug.log("disabled", output.Version, passedJobID)
		return false, false, "", nil
	}

	if inputConfig.PinnedVersion != nil {
		version, found, err := r.vdb.FindVersionOfResource(inputConfig.ResourceID, inputConfig.PinnedVersion)
		if err != nil {
			return false, false, "", err
		}

		if !found {
			return false, false, db.PinnedVersionNotFound{PinnedVersion: inputConfig.PinnedVersion}.String(), nil
		}

		if version != output.Version {
			// this input has a pinned constraint and this version does not match the
			// required pinned version
			r.debug.log("mismatch pinned version", output.Version, passedJobID)
			return false, true, "", nil
		}
	}

	if candidate != nil && candidate.Version != output.Version {
		// don't return here! just try the next output set. it's possible
		// we just need to use an older output set.
		r.debug.log("mismatch candidate version", candidate.Version, "comparing to output version", output.Version)
		return false, true, "", nil
	}

	return true, false, "", nil
}

func (r *groupResolver) vouchForCandidate(oldCandidate *versionCandidate, version db.ResourceVersion, passedJobID int, passedBuildID int, hasNext bool) *versionCandidate {
	// create a new candidate with the new version
	newCandidate := newCandidateVersion(version)

	// carry over the vouchers from the previous state of the candidate
	if oldCandidate != nil {
		for vJobID := range oldCandidate.VouchedForBy {
			newCandidate.VouchedForBy[vJobID] = true
		}

		if len(oldCandidate.SourceBuildIds) != 0 {
			for _, sourceBuildId := range oldCandidate.SourceBuildIds {
				newCandidate.SourceBuildIds = append(newCandidate.SourceBuildIds, sourceBuildId)
			}
		}

		newCandidate.HasNextEveryVersion = oldCandidate.HasNextEveryVersion
	}

	// vouch for the version with this new passed job and append the passed build
	// that we used the outputs of to satisfy the input constraints. The source
	// build IDs are used for the build pipes table.
	newCandidate.VouchedForBy[passedJobID] = true
	newCandidate.SourceBuildIds = append(newCandidate.SourceBuildIds, passedBuildID)
	newCandidate.HasNextEveryVersion = newCandidate.HasNextEveryVersion || hasNext

	return newCandidate
}

func (r *groupResolver) orderJobs(jobIDs map[int]bool) []int {
	orderedJobs := []int{}
	for id, _ := range jobIDs {
		orderedJobs = append(orderedJobs, id)
	}

	sort.Ints(orderedJobs)

	return orderedJobs
}
