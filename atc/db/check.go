package db

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

type CheckStatus string

const (
	CheckStatusStarted   CheckStatus = "started"
	CheckStatusSucceeded CheckStatus = "succeeded"
	CheckStatusErrored   CheckStatus = "errored"
)

//go:generate counterfeiter . Check

type Check interface {
	ID() int
	ResourceConfigScope() (ResourceConfigScope, error)
	Start() error
	Timeout() time.Duration
	FromVersion() atc.Version
	Finish() error
	FinishWithError(err error) error

	Status() CheckStatus
	IsRunning() bool
	AcquireTrackingLock() (lock.Lock, bool, error)
}

const (
	CheckTypeResource     = "resource"
	CheckTypeResourceType = "resource_type"
)

var checksQuery = psql.Select("r.id, r.resource_config_scope_id, r.start_time, r.end_time, r.timeout, r.from_version, r.check_error, r.create_time").
	From("checks r")

type check struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func (r *check) ID() int                    { return 0 }
func (r *check) ResourceConfigScopeID() int { return 0 }
func (r *check) StartTime() time.Time       { return time.Now() }
func (r *check) EndTime() time.Time         { return time.Now() }
func (r *check) CreateTime() time.Time      { return time.Now() }
func (r *check) Timeout() time.Duration     { return time.Second }
func (r *check) FromVersion() atc.Version   { return nil }

func (r *check) Start() error {
	return nil
}

func (r *check) Finish() error {
	return nil
}

func (r *check) FinishWithError(err error) error {
	return nil
}

func (r *check) ResourceConfigScope() (ResourceConfigScope, error) {
	return nil, nil
}

func (r *check) IsRunning() bool {
	return false
}

func (r *check) Status() CheckStatus {
	return CheckStatusErrored
}

func (r *check) AcquireTrackingLock() (lock.Lock, bool, error) {
	return nil, false, nil
}

func scanCheck(r *check, row scannable) error {
	return nil
}
