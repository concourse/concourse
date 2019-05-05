package resource

import (
	"context"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/metrics"
)

type checkRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

func (resource *resource) Check(ctx context.Context, source atc.Source, fromVersion atc.Version) ([]atc.Version, error) {
	var versions []atc.Version

	checkStartTime := time.Now()
	err := resource.runScript(
		ctx,
		"/opt/resource/check",
		nil,
		checkRequest{source, fromVersion},
		&versions,
		nil,
		false,
	)
	metrics.
		ResourceChecksDuration.
		WithLabelValues(metrics.StatusFromError(err)).
		Observe(time.Now().Sub(checkStartTime).Seconds())
	if err != nil {
		return nil, err
	}

	return versions, nil
}
