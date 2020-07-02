package watch

import (
	"context"

	"github.com/concourse/concourse/atc/api/accessor"
)

type DisabledListAllJobsWatcher struct {
}

func (d DisabledListAllJobsWatcher) WatchListAllJobs(ctx context.Context, access accessor.Access) (<-chan []DashboardJobEvent, error) {
	return nil, ErrDisabled
}