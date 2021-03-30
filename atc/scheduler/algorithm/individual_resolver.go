package algorithm

import (
	"context"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type individualResolver struct {
	vdb         db.VersionsDB
	inputConfig db.InputConfig
}

func NewIndividualResolver(vdb db.VersionsDB, inputConfig db.InputConfig) Resolver {
	return &individualResolver{
		vdb:         vdb,
		inputConfig: inputConfig,
	}
}

func (r *individualResolver) InputConfigs() db.InputConfigs {
	return db.InputConfigs{r.inputConfig}
}

// Handles two different configurations of a resource without passed
// constraints: every and latest
func (r *individualResolver) Resolve(ctx context.Context) (map[string]*versionCandidate, db.ResolutionFailure, error) {
	ctx, span := tracing.StartSpan(ctx, "individualResolver.Resolve", tracing.Attrs{
		"input": r.inputConfig.Name,
	})
	defer span.End()

	var version db.ResourceVersion
	var hasNext bool
	if r.inputConfig.UseEveryVersion {
		var found bool
		var err error
		version, hasNext, found, err = r.vdb.NextEveryVersion(ctx, r.inputConfig.JobID, r.inputConfig.ResourceID)
		if err != nil {
			tracing.End(span, err)
			return nil, "", err
		}

		if !found {
			span.AddEvent("next every version not found")
			span.SetStatus(codes.Error, "next every version not found")
			return nil, db.VersionNotFound, nil
		}

		span.AddEvent("found via every", trace.WithAttributes(
			attribute.String("version", string(version)),
		))
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
			span.AddEvent("latest version not found")
			span.SetStatus(codes.Error, "latest version not found")
			return nil, db.LatestVersionNotFound, nil
		}

		span.AddEvent("found via latest", trace.WithAttributes(
			attribute.String("version", string(version)),
		))
	}

	candidate := newCandidateVersion(version)
	candidate.HasNextEveryVersion = hasNext

	versionCandidates := map[string]*versionCandidate{
		r.inputConfig.Name: candidate,
	}

	span.SetStatus(codes.Ok, "")
	return versionCandidates, "", nil
}
