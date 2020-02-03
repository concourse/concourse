package algorithm

import (
	"context"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/api/key"
	"google.golang.org/grpc/codes"
)

type individualResolver struct {
	vdb         db.VersionsDB
	inputConfig InputConfig
}

func NewIndividualResolver(vdb db.VersionsDB, inputConfig InputConfig) Resolver {
	return &individualResolver{
		vdb:         vdb,
		inputConfig: inputConfig,
	}
}

func (r *individualResolver) InputConfigs() InputConfigs {
	return InputConfigs{r.inputConfig}
}

// Handles the three different configurations of a resource without passed
// constraints: pinned, every and latest
func (r *individualResolver) Resolve(ctx context.Context) (map[string]*versionCandidate, db.ResolutionFailure, error) {
	ctx, span := tracing.StartSpan(ctx, "individualResolver.Resolve", tracing.Attrs{
		"input": r.inputConfig.Name,
	})
	defer span.End()

	var version db.ResourceVersion
	var hasNext bool
	if r.inputConfig.UseEveryVersion {
		buildID, found, err := r.vdb.LatestBuildID(ctx, r.inputConfig.JobID)
		if err != nil {
			tracing.End(span, err)
			return nil, "", err
		}

		if found {
			version, hasNext, found, err = r.vdb.NextEveryVersion(ctx, buildID, r.inputConfig.ResourceID)
			if err != nil {
				tracing.End(span, err)
				return nil, "", err
			}

			if !found {
				span.AddEvent(ctx, "next every version not found")
				span.SetStatus(codes.NotFound)
				return nil, db.VersionNotFound, nil
			}
		} else {
			version, found, err = r.vdb.LatestVersionOfResource(ctx, r.inputConfig.ResourceID)
			if err != nil {
				tracing.End(span, err)
				return nil, "", err
			}

			if !found {
				span.AddEvent(ctx, "latest version not found")
				span.SetStatus(codes.NotFound)
				return nil, db.LatestVersionNotFound, nil
			}
		}

		span.AddEvent(ctx, "found via every", key.New("version").String(string(version)))
	} else {
		// there are no passed constraints, so just take the latest version
		var err error
		var found bool
		version, found, err = r.vdb.LatestVersionOfResource(ctx, r.inputConfig.ResourceID)
		if err != nil {
			tracing.End(span, err)
			return nil, "", err
		}

		if !found {
			span.AddEvent(ctx, "latest version not found")
			span.SetStatus(codes.NotFound)
			return nil, db.LatestVersionNotFound, nil
		}

		span.AddEvent(ctx, "found via latest", key.New("version").String(string(version)))
	}

	candidate := newCandidateVersion(version)
	candidate.HasNextEveryVersion = hasNext

	versionCandidates := map[string]*versionCandidate{
		r.inputConfig.Name: candidate,
	}

	span.SetStatus(codes.OK)
	return versionCandidates, "", nil
}
