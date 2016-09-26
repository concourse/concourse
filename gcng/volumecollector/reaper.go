package volumecollector

import "code.cloudfoundry.org/lager"

type Reaper interface {
	Run() error
}

type reaper struct {
	logger lager.Logger
	db     ReaperDB
}

func NewReaper(
	logger lager.Logger,
	db ReaperDB,
) BuildReaper {
	return &reaper{
		logger: logger,
		db:     db,
	}
}

func (r *Reaper) Run() error {

}
