package gc

import (
	"context"
	"github.com/concourse/concourse/atc/db"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/metric"
)

type versionReaper struct {
	conn                   db.Conn
	maxVersionsPerResource int
}

func NewVersionReaper(conn db.Conn, maxVersionsPerResource int) *versionReaper {
	if maxVersionsPerResource < 100 {
		maxVersionsPerResource = 100
	}

	return &versionReaper{
		conn:                   conn,
		maxVersionsPerResource: maxVersionsPerResource,
	}
}

func (r *versionReaper) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("version-reaper")

	start := time.Now()
	defer func() {
		metric.VersionReaperDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	logger.Debug("start")
	defer logger.Debug("done")

	return db.NewResourceVersionReaper(logger, r.conn, r.maxVersionsPerResource).ReapVersions(ctx)
}
