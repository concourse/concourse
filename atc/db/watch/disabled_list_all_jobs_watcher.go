package watch

import (
	"context"
)

type DisabledListAllJobsWatcher struct {
}

func (d DisabledListAllJobsWatcher) WatchListAllJobs(ctx context.Context) (<-chan []JobSummaryEvent, error) {
	return nil, ErrDisabled
}
