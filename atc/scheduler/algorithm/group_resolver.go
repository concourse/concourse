package algorithm

import (
	"context"
	"sort"
	"strconv"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"
	"google.golang.org/grpc/codes"
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
	inputConfigs db.InputConfigs

	pins        []db.ResourceVersion
	orderedJobs [][]int
	candidates  []*versionCandidate

	doomedCandidates []*versionCandidate

	lastUsedPassedBuilds map[int]db.BuildCursor
}

func NewGroupResolver(vdb db.VersionsDB, inputConfigs db.InputConfigs) Resolver {
	return &groupResolver{
		vdb:              vdb,
		inputConfigs:     inputConfigs,
		pins:             make([]db.ResourceVersion, len(inputConfigs)),
		orderedJobs:      make([][]int, len(inputConfigs)),
		candidates:       make([]*versionCandidate, len(inputConfigs)),
		doomedCandidates: make([]*versionCandidate, len(inputConfigs)),
	}
}

func (r *groupResolver) InputConfigs() db.InputConfigs {
	return r.inputConfigs
}

func (r *groupResolver) Resolve(ctx context.Context) (map[string]*versionCandidate, db.ResolutionFailure, error) {
	ctx, span := tracing.StartSpan(ctx, "groupResolver.Resolve", tracing.Attrs{
		"inputs": r.inputConfigs.String(),
	})
	defer span.End()

	for i, cfg := range r.inputConfigs {
		if cfg.PinnedVersion == nil {
			continue
		}

		version, found, err := r.vdb.FindVersionOfResource(ctx, cfg.ResourceID, cfg.PinnedVersion)
		if err != nil {
			tracing.End(span, err)
			return nil, "", err
		}

		if !found {
			notFoundErr := db.PinnedVersionNotFound{PinnedVersion: cfg.PinnedVersion}
			span.SetStatus(codes.InvalidArgument)
			return nil, notFoundErr.String(), nil
		}

		r.pins[i] = version
	}

	resolved, failure, err := r.tryResolve(ctx)
	if err != nil {
		tracing.End(span, err)
		return nil, "", err
	}

	if !resolved {
		span.SetAttributes(key.New("failure").String(string(failure)))
		span.SetStatus(codes.NotFound)
		return nil, failure, nil
	}

	finalCandidates := map[string]*versionCandidate{}
	for i, input := range r.inputConfigs {
		finalCandidates[input.Name] = r.candidates[i]
	}

	span.SetStatus(codes.OK)
	return finalCandidates, "", nil
}

func (r *groupResolver) tryResolve(ctx context.Context) (bool, db.ResolutionFailure, error) {
	ctx, span := tracing.StartSpan(ctx, "groupResolver.tryResolve", tracing.Attrs{
		"inputs": r.inputConfigs.String(),
	})
	defer span.End()

	for inputIndex := range r.inputConfigs {
		worked, failure, err := r.trySatisfyPassedConstraintsForInput(ctx, inputIndex)
		if err != nil {
			tracing.End(span, err)
			return false, "", err
		}

		if !worked {
			// input was not satisfiable
			span.SetStatus(codes.NotFound)
			return false, failure, nil
		}
	}

	// got to the end of all the inputs
	span.SetStatus(codes.OK)
	return true, "", nil
}

func (r *groupResolver) trySatisfyPassedConstraintsForInput(ctx context.Context, inputIndex int) (bool, db.ResolutionFailure, error) {
	inputConfig := r.inputConfigs[inputIndex]
	currentJobID := inputConfig.JobID

	ctx, span := tracing.StartSpan(ctx, "groupResolver.trySatisfyPassedConstraintsForInput", tracing.Attrs{
		"input": inputConfig.Name,
	})
	defer span.End()

	// current candidate, if coming from a recursive call
	currentCandidate := r.candidates[inputIndex]

	// deterministically order the passed jobs for this input
	orderedJobs := r.orderJobs(inputConfig.Passed)

	for _, passedJobID := range orderedJobs {
		if currentCandidate != nil {
			// coming from recursive call; we've already got a candidate
			if currentCandidate.VouchedForBy[passedJobID] {
				// we've already been here; continue to the next job
				continue
			}
		}

		builds, skip, err := r.paginatedBuilds(ctx, inputConfig, currentCandidate, currentJobID, passedJobID)
		if err != nil {
			tracing.End(span, err)
			return false, "", err
		}

		if skip {
			span.AddEvent(ctx, "deferring selection to other jobs", key.New("passedJobID").Int(passedJobID))
			continue
		}

		worked, err := r.tryJobBuilds(ctx, inputIndex, passedJobID, builds)
		if err != nil {
			tracing.End(span, err)
			return false, "", err
		}

		if worked {
			// resolving recursively worked!
			break
		} else {
			span.SetStatus(codes.NotFound)
			return false, db.NoSatisfiableBuilds, nil
		}
	}

	// all passed constraints were satisfied
	span.SetStatus(codes.OK)
	return true, "", nil
}

func (r *groupResolver) tryJobBuilds(ctx context.Context, inputIndex int, passedJobID int, builds db.PaginatedBuilds) (bool, error) {
	ctx, span := tracing.StartSpan(ctx, "groupResolver.tryJobBuilds", tracing.Attrs{})
	defer span.End()

	span.SetAttributes(key.New("passedJobID").Int(passedJobID))

	for {
		buildID, ok, err := builds.Next(ctx)
		if err != nil {
			tracing.End(span, err)
			return false, err
		}

		if !ok {
			// reached the end of the builds
			span.SetStatus(codes.ResourceExhausted)
			return false, nil
		}

		worked, err := r.tryBuildOutputs(ctx, inputIndex, passedJobID, buildID, builds.HasNext())
		if err != nil {
			tracing.End(span, err)
			return false, err
		}

		if worked {
			span.SetStatus(codes.OK)
			return true, nil
		}
	}
}

func (r *groupResolver) tryBuildOutputs(ctx context.Context, resolvingIdx, jobID, buildID int, hasNext bool) (bool, error) {
	ctx, span := tracing.StartSpan(ctx, "groupResolver.tryBuildOutputs", tracing.Attrs{})
	defer span.End()

	span.SetAttributes(key.New("buildID").Int(buildID))

	outputs, err := r.vdb.SuccessfulBuildOutputs(ctx, buildID)
	if err != nil {
		tracing.End(span, err)
		return false, err
	}

	restore := map[int]*versionCandidate{}
	var mismatch bool

	// loop over the resource versions that came out of this build set
outputs:
	for _, output := range outputs {
		// try to pin each candidate to the versions from this build
		for c, candidate := range r.candidates {
			if _, ok := restore[c]; ok {
				// we have already set a new version for this candidate within this
				// build, so continue attempting the existing version
				continue
			}

			var related bool
			related, mismatch, err = r.outputIsRelatedAndMatches(ctx, span, output, c, jobID)
			if err != nil {
				tracing.End(span, err)
				return false, err
			}

			if mismatch {
				// build contained a different version than the one we already have for
				// that candidate, so let's try a different build
				break outputs
			} else if !related {
				// output is not even relevant to this candidate; move on
				continue
			}

			if candidate == nil {
				exists, err := r.vdb.VersionExists(ctx, output.ResourceID, output.Version)
				if err != nil {
					tracing.End(span, err)
					return false, err
				}

				if !exists {
					break outputs
				}
			}

			// if this doesn't work out, restore it to either nil or the
			// candidate *without* the job vouching for it
			restore[c] = candidate

			span.AddEvent(
				ctx,
				"vouching for candidate",
				key.New("resourceID").Int(output.ResourceID),
				key.New("version").String(string(output.Version)),
			)

			r.candidates[c] = r.vouchForCandidate(candidate, output.Version, jobID, buildID, hasNext)
		}
	}

	// we found a candidate for ourselves and the rest are OK too - recurse
	if r.candidates[resolvingIdx] != nil && r.candidates[resolvingIdx].VouchedForBy[jobID] && !mismatch {
		if r.candidatesAreDoomed() {
			span.AddEvent(
				ctx,
				"candidates are doomed",
			)
		} else {
			worked, _, err := r.tryResolve(ctx)
			if err != nil {
				tracing.End(span, err)
				return false, err
			}

			if worked {
				// this build's candidates satisfied everything else!
				span.SetStatus(codes.OK)
				return true, nil
			}

			r.doomCandidates()
		}
	}

	for c, candidate := range restore {
		// either there was a mismatch or resolving didn't work; go on to the
		// next output set
		r.candidates[c] = candidate
	}

	span.SetStatus(codes.InvalidArgument)
	return false, nil
}

func (r *groupResolver) doomCandidates() {
	for i, c := range r.candidates {
		r.doomedCandidates[i] = c
	}
}

func (r *groupResolver) candidatesAreDoomed() bool {
	for i, c := range r.candidates {
		doomed := r.doomedCandidates[i]

		if c == nil && doomed == nil {
			continue
		}

		if c == nil && doomed != nil {
			return false
		}

		if c != nil && doomed == nil {
			return false
		}

		if doomed.Version != c.Version {
			return false
		}
	}

	return true
}

func (r *groupResolver) paginatedBuilds(ctx context.Context, currentInputConfig db.InputConfig, currentCandidate *versionCandidate, currentJobID int, passedJobID int) (db.PaginatedBuilds, bool, error) {
	constraints := r.constrainingCandidates(passedJobID)

	if currentInputConfig.UseEveryVersion {
		if r.lastUsedPassedBuilds == nil {
			lastUsedBuildIDs := map[int]db.BuildCursor{}

			buildID, found, err := r.vdb.LatestBuildUsingLatestVersion(ctx, currentJobID, currentInputConfig.ResourceID)
			if err != nil {
				return db.PaginatedBuilds{}, false, err
			}

			if found {
				lastUsedBuildIDs, err = r.vdb.LatestBuildPipes(ctx, buildID)
				if err != nil {
					return db.PaginatedBuilds{}, false, err
				}

				r.lastUsedPassedBuilds = lastUsedBuildIDs
			}
		}

		relatedPassedBuilds := map[int]db.BuildCursor{}
		for jobID, build := range r.lastUsedPassedBuilds {
			if currentInputConfig.Passed[jobID] {
				relatedPassedBuilds[jobID] = build
			}
		}

		lastUsedBuild, hasUsedJob := relatedPassedBuilds[passedJobID]
		if hasUsedJob {
			var paginatedBuilds db.PaginatedBuilds
			var err error

			if currentCandidate != nil {
				paginatedBuilds, err = r.vdb.UnusedBuildsVersionConstrained(ctx, passedJobID, lastUsedBuild, constraints)
			} else {
				paginatedBuilds, err = r.vdb.UnusedBuilds(ctx, passedJobID, lastUsedBuild)
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
			return db.PaginatedBuilds{}, true, nil
		}
	}

	var paginatedBuilds db.PaginatedBuilds
	var err error
	if currentCandidate != nil {
		paginatedBuilds, err = r.vdb.SuccessfulBuildsVersionConstrained(ctx, passedJobID, constraints)
	} else {
		paginatedBuilds = r.vdb.SuccessfulBuilds(ctx, passedJobID)
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

func (r *groupResolver) outputIsRelatedAndMatches(ctx context.Context, span trace.Span, output db.AlgorithmVersion, candidateIdx int, passedJobID int) (bool, bool, error) {
	inputConfig := r.inputConfigs[candidateIdx]
	candidate := r.candidates[candidateIdx]

	if inputConfig.ResourceID != output.ResourceID {
		// unrelated; different resource
		return false, false, nil
	}

	if !inputConfig.Passed[passedJobID] {
		// unrelated; this input is unaffected by the current job
		return false, false, nil
	}

	if candidate != nil && candidate.Version != output.Version {
		// we have already chosen a version for the candidate but it's different
		// from the version provided by this output
		return false, true, nil
	}

	disabled, err := r.vdb.VersionIsDisabled(ctx, output.ResourceID, output.Version)
	if err != nil {
		return false, false, err
	}

	if disabled {
		// this version is disabled so it cannot be used
		span.AddEvent(
			ctx,
			"version disabled",
			key.New("resourceID").Int(output.ResourceID),
			key.New("version").String(string(output.Version)),
		)
		return false, false, nil
	}

	if inputConfig.PinnedVersion != nil && r.pins[candidateIdx] != output.Version {
		// input is both pinned and assigned a 'passed' constraint, but the pinned
		// version doesn't match the job's output version

		span.AddEvent(
			ctx,
			"pin mismatch",
			key.New("resourceID").Int(output.ResourceID),
			key.New("outputHas").String(string(output.Version)),
			key.New("pinHas").String(string(r.pins[candidateIdx])),
		)

		return false, false, nil
	}

	return true, false, nil
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
