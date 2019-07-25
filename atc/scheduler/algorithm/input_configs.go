package algorithm

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

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
	PassedJobID int
	JobIDs      map[string]int
}

func (e NoSatisfiableBuildsForPassedJobError) Error() string {
	var jobName string
	for jName, jID := range e.JobIDs {
		if jID == e.PassedJobID {
			jobName = jName
		}
	}
	return fmt.Sprintf("passed job '%s' does not have a build that satisfies the constraints", jobName)
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
				mapping := db.InputMapping{}
				for _, innerInput := range job.Config().Inputs() {
					if innerInput.Name == input.Name {
						mapping[innerInput.Name] = db.InputResult{
							ResolveError: PinnedVersionNotFoundError{pinnedVersion},
						}
					}
				}

				return mapping, false, nil
			}

			inputConfig.PinnedVersion = version
		}

		inputConfigs = append(inputConfigs, inputConfig)
	}

	return im.computeNextInputs(inputConfigs, versions, job.ID())
}

func (im *inputMapper) computeNextInputs(configs InputConfigs, versionsDB *db.VersionsDB, currentJobID int) (db.InputMapping, bool, error) {
	mapping := db.InputMapping{}

	candidates, resolved, err := im.resolve(versionsDB, configs)
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

	for i, config := range configs {
		if candidates[i] != nil {
			if candidates[i].ResolveError != nil {
				mapping[config.Name] = db.InputResult{
					ResolveError: candidates[i].ResolveError,
				}
			} else {
				mapping[config.Name] = db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							ResourceID: config.ResourceID,
							Version:    candidates[i].Version,
						},
						FirstOccurrence: !found || latestBuildOutputs[config.Name] != candidates[i].Version,
					},
					PassedBuildIDs: candidates[i].SourceBuildIds,
				}
			}
		}
	}

	return mapping, resolved, nil
}

type resolver struct {
	vdb          *db.VersionsDB
	inputConfigs InputConfigs
	candidates   []*versionCandidate

	// These candidates are used for holding partially resolved candidates,
	// meaning that it contains both valid versions for candidates and also
	// errors. It is used for the preparation pending state within a build.
	partialCandidates []*versionCandidate

	debug debugger
}

func (im *inputMapper) resolve(vdb *db.VersionsDB, inputConfigs InputConfigs) ([]*versionCandidate, bool, error) {
	resolver := resolver{
		vdb:               vdb,
		inputConfigs:      inputConfigs,
		candidates:        make([]*versionCandidate, len(inputConfigs)),
		partialCandidates: make([]*versionCandidate, len(inputConfigs)),
		debug:             debugger{},
	}

	resolved, err := resolver.tryResolve(0)
	if err != nil {
		return nil, false, err
	}

	if !resolved {
		return resolver.partialCandidates, resolved, nil
	}

	return resolver.candidates, resolved, nil
}

func (r *resolver) tryResolve(depth int) (bool, error) {
	// NOTE 3: maybe also select distinct build outputs so we don't waste time on
	// the same thing (i.e. constantly re-triggered build)
	//
	// NOTE : make sure everything is deterministically ordered
	defer func() { r.debug.depth-- }()

	for i, inputConfig := range r.inputConfigs {
		r.debug.reset(depth, inputConfig.Name)

		if len(inputConfig.Passed) == 0 {
			// coming from recursive call; already set to the latest version
			if r.candidates[i] != nil {
				continue
			}

			candidate, found, err := r.computeVersionWithoutPassed(inputConfig)
			if err != nil {
				return false, err
			}

			r.partialCandidates[i] = candidate

			if found {
				r.candidates[i] = candidate
			} else {
				return false, nil
			}

			continue
		}

		orderedJobs := []int{}
		if len(inputConfig.Passed) != 0 {
			var err error
			orderedJobs, err = r.vdb.OrderPassedJobs(inputConfig.JobID, inputConfig.Passed)
			if err != nil {
				return false, err
			}
		}

		lastUsedBuildIDs := map[int]int{}
		if inputConfig.UseEveryVersion {
			buildID, found, err := r.vdb.LatestBuildID(inputConfig.JobID)
			if err != nil {
				return false, err
			}

			if found {
				lastUsedBuildIDs, err = r.vdb.LatestBuildPipes(buildID, inputConfig.Passed)
				if err != nil {
					return false, err
				}
			}
		}

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

			if r.candidates[i] == nil && inputConfig.UseEveryVersion && len(lastUsedBuildIDs) > 0 && lastUsedBuildIDs[jobID] == 0 {
				// we've run with version: every and passed: before, just not with this
				// job, and there's no candidate yet, so skip it for now and let the
				// algorithm continue from where the other jobs left off rather than
				// starting from 'latest'
				//
				// this job will eventually vouch for it during the recursive resolve
				// call
				continue
			}

			paginatedBuilds, err := r.paginatedBuilds(inputConfig, r.candidates[i], lastUsedBuildIDs, jobID)
			if err != nil {
				return false, err
			}

			for {
				buildID, ok, err := paginatedBuilds.Next()
				if err != nil {
					return false, err
				}

				if !ok {
					r.debug.log("reached end")
					break
				}

				r.debug.log("job", jobID, "trying build", jobID, buildID)

				outputs, err := r.vdb.SuccessfulBuildOutputs(buildID)
				if err != nil {
					return false, err
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
						satisfied, mismatch = r.outputSatisfyCandidateConstraints(output, r.inputConfigs[c], candidate, jobID)
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
						r.candidates[c] = r.vouchForCandidate(candidate, output.Version, jobID, buildID)

						allVouchedFor := true
						for passedJob, _ := range r.inputConfigs[c].Passed {
							if !r.candidates[c].VouchedForBy[passedJob] {
								allVouchedFor = false
							}
						}

						if allVouchedFor {
							r.partialCandidates[c] = r.candidates[c]
						}
					}
				}

				// we found a candidate for ourselves and the rest are OK too - recurse
				if r.candidates[i] != nil && r.candidates[i].VouchedForBy[jobID] && !mismatch {
					r.debug.log("recursing")

					resolved, err := r.tryResolve(depth + 1)
					if err != nil {
						return false, err
					}

					if resolved {
						// we've attempted to resolve all of the inputs!
						return true, nil
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

			// we've exhausted all the builds and never found a matching input set;
			// give up on this input
			if r.partialCandidates[i] == nil {
				r.debug.log("setting candidate", i, "to no satisfiable builds error")
				r.partialCandidates[i] = newCandidateError(NoSatisfiableBuildsForPassedJobError{PassedJobID: jobID, JobIDs: r.vdb.JobIDs})
			}

			// reached the end of the builds
			return false, nil
		}
	}

	// go to the end of all the inputs
	return true, nil
}

func (r *resolver) computeVersionWithoutPassed(inputConfig InputConfig) (*versionCandidate, bool, error) {
	var version db.ResourceVersion

	if inputConfig.PinnedVersion != "" {
		version = inputConfig.PinnedVersion
		r.debug.log("setting candidate", inputConfig.Name, "to unconstrained version", version)
	} else if inputConfig.UseEveryVersion {
		buildID, found, err := r.vdb.LatestBuildID(inputConfig.JobID)
		if err != nil {
			return nil, false, err
		}

		if found {
			version, found, err = r.vdb.NextEveryVersion(buildID, inputConfig.ResourceID)
			if err != nil {
				return nil, false, err
			}

			if !found {
				return newCandidateError(ErrVersionNotFound), false, nil
			}
		} else {
			version, found, err = r.vdb.LatestVersionOfResource(inputConfig.ResourceID)
			if err != nil {
				return nil, false, err
			}

			if !found {
				return newCandidateError(ErrLatestVersionNotFound), false, nil
			}
		}

		r.debug.log("setting candidate", inputConfig.Name, "to version for version every", version, " resource ", inputConfig.ResourceID)
	} else {
		// there are no passed constraints, so just take the latest version
		var err error
		var found bool
		version, found, err = r.vdb.LatestVersionOfResource(inputConfig.ResourceID)
		if err != nil {
			return nil, false, nil
		}

		if !found {
			return newCandidateError(ErrLatestVersionNotFound), false, nil
		}

		r.debug.log("setting candidate", inputConfig.Name, "to version for latest", version)
	}

	return newCandidateVersion(version), true, nil
}

func (r *resolver) paginatedBuilds(currentInputConfig InputConfig, currentCandidate *versionCandidate, lastUsedBuildIDs map[int]int, passedJobID int) (db.PaginatedBuilds, error) {
	constraints := r.constrainingCandidates(passedJobID)

	if currentInputConfig.UseEveryVersion {
		lastUsedBuildID, hasUsedJob := lastUsedBuildIDs[passedJobID]
		if hasUsedJob {
			if currentCandidate != nil {
				return r.vdb.UnusedBuildsVersionConstrained(lastUsedBuildID, passedJobID, constraints)
			} else {
				return r.vdb.UnusedBuilds(lastUsedBuildID, passedJobID)
			}
		} else if currentCandidate != nil {
			return r.vdb.SuccessfulBuildsVersionConstrained(passedJobID, constraints)
		}
	} else if currentCandidate != nil {
		return r.vdb.SuccessfulBuildsVersionConstrained(passedJobID, constraints)
	}

	return r.vdb.SuccessfulBuilds(passedJobID), nil
}

func (r *resolver) constrainingCandidates(passedJobID int) map[string][]string {
	constrainingCandidates := map[string][]string{}
	for passedIndex, passedInput := range r.inputConfigs {
		if passedInput.Passed[passedJobID] && r.candidates[passedIndex] != nil {
			resID := strconv.Itoa(passedInput.ResourceID)
			constrainingCandidates[resID] = append(constrainingCandidates[resID], string(r.candidates[passedIndex].Version))
		}
	}

	return constrainingCandidates
}

func (r *resolver) outputSatisfyCandidateConstraints(output db.AlgorithmVersion, inputConfig InputConfig, candidate *versionCandidate, passedJobID int) (bool, bool) {
	if inputConfig.ResourceID != output.ResourceID {
		// unrelated to this output because it is a different resource
		return false, false
	}

	if !inputConfig.Passed[passedJobID] {
		// this candidate is unaffected by the current job
		r.debug.log("independent", inputConfig.Passed, passedJobID)
		return false, false
	}

	if r.vdb.VersionIsDisabled(output.ResourceID, output.Version) {
		// this version is disabled so it cannot be used
		r.debug.log("disabled", output.Version, passedJobID)
		return false, true
	}

	if inputConfig.PinnedVersion != "" && inputConfig.PinnedVersion != output.Version {
		// this input has a pinned constraint and this version does not match the
		// required pinned version
		r.debug.log("mismatch pinned version", output.Version, passedJobID)
		return false, true
	}

	if candidate != nil && candidate.Version != output.Version {
		// don't return here! just try the next output set. it's possible
		// we just need to use an older output set.
		r.debug.log("mismatch candidate version", candidate.Version, "comparing to output version", output.Version)
		return false, true
	}

	return true, false
}

func (r *resolver) vouchForCandidate(oldCandidate *versionCandidate, version db.ResourceVersion, passedJobID int, passedBuildID int) *versionCandidate {
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
	}

	// vouch for the version with this new passed job and append the passed build
	// that we used the outputs of to satisfy the input constraints. The source
	// build IDs are used for the build pipes table.
	newCandidate.VouchedForBy[passedJobID] = true
	newCandidate.SourceBuildIds = append(newCandidate.SourceBuildIds, passedBuildID)

	return newCandidate
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
	log.Println(
		append(
			[]interface{}{
				strings.Repeat("-", d.depth) + fmt.Sprintf("[%s]", d.inputName),
			},
			messages...,
		)...,
	)
}
