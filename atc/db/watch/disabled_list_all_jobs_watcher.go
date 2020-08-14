package watch

import (
	"context"
)

type DisabledListAllJobsWatcher struct {
}

func (d DisabledListAllJobsWatcher) WatchListAllJobs(ctx context.Context) (<-chan []DashboardJobEvent, error) {
	return nil, ErrDisabled
}
