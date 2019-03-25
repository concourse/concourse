package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

type artifactCollector struct {
	artifactLifecycle db.WorkerArtifactLifecycle
}

func NewArtifactCollector(artifactLifecycle db.WorkerArtifactLifecycle) *artifactCollector {
	return &artifactCollector{
		artifactLifecycle: artifactLifecycle,
	}
}

func (a *artifactCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("artifact-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	return a.artifactLifecycle.RemoveExpiredArtifacts()
}
