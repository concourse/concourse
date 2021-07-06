package db

import (
	"database/sql"
	atc_component "github.com/concourse/concourse/atc/component"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

var componentsQuery = psql.Select("c.id, c.name, c.interval, c.last_ran, c.paused, c.last_run_result").
	From("components c")

//counterfeiter:generate . Component
type Component interface {
	ID() int
	Name() string
	Interval() time.Duration
	LastRan() time.Time
	Paused() bool

	Reload() (bool, error)
	IntervalElapsed() bool
	UpdateLastRan(when time.Time, runResult atc_component.RunResult) error

	LastRunResult() string
}

type component struct {
	id       int
	name     string
	interval time.Duration
	lastRan  time.Time
	paused   bool
	lastRunResult string

	conn Conn
}

func (c *component) ID() int                 { return c.id }
func (c *component) Name() string            { return c.name }
func (c *component) Interval() time.Duration { return c.interval }
func (c *component) LastRan() time.Time      { return c.lastRan }
func (c *component) Paused() bool            { return c.paused }

func (c *component) Reload() (bool, error) {
	row := componentsQuery.Where(sq.Eq{"c.id": c.id}).
		RunWith(c.conn).
		QueryRow()

	err := scanComponent(c, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *component) IntervalElapsed() bool {
	return !time.Now().Before(c.lastRan.Add(c.interval))
}

func (c *component) UpdateLastRan(when time.Time, lastRunResult atc_component.RunResult) error {
	builder := psql.Update("components")
	if when.IsZero() {
		builder = builder.Set("last_ran", sq.Expr("now()"))
	} else {
		builder = builder.Set("last_ran", when)
	}
	if lastRunResult != nil {
		builder = builder.Set("last_run_result", lastRunResult.String())
	}
	_, err := builder.
		Where(sq.Eq{
			"id": c.id,
		}).
		RunWith(c.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (c *component) LastRunResult() string {
	return c.lastRunResult
}

func scanComponent(c *component, row scannable) error {
	var (
		lastRan  pq.NullTime
		interval string
		lastRunResult sql.NullString
	)

	err := row.Scan(
		&c.id,
		&c.name,
		&interval,
		&lastRan,
		&c.paused,
		&lastRunResult,
	)
	if err != nil {
		return err
	}

	c.lastRan = lastRan.Time

	c.interval, err = time.ParseDuration(interval)
	if err != nil {
		return err
	}

	if lastRunResult.Valid {
		c.lastRunResult = lastRunResult.String
	}

	return nil
}
