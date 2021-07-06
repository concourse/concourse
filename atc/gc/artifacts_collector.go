package gc

import (
	"context"
	"github.com/concourse/concourse/atc/component"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

type artifactCollector struct {
	artifactLifecycle db.WorkerArtifactLifecycle
}

func NewArtifactCollector(artifactLifecycle db.WorkerArtifactLifecycle) *artifactCollector {
	return &artifactCollector{
		artifactLifecycle: artifactLifecycle,
	}
}

func (a *artifactCollector) Run(ctx context.Context, _ string) (component.RunResult, error) {
	logger := lagerctx.FromContext(ctx).Session("artifact-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.ArtifactCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	return nil, a.artifactLifecycle.RemoveExpiredArtifacts()
}
