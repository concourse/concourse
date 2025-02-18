package db

import (
	"database/sql"
	"runtime"
	"time"

	"code.cloudfoundry.org/clock"

	sq "github.com/Masterminds/squirrel"
)

var componentsQuery = psql.Select("c.id, c.name, c.interval, c.last_ran, c.paused").
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
	UpdateLastRan() error
}

//counterfeiter:generate . ComponentRand
type ComponentRand interface {
	Int() int
}

//counterfeiter:generate . GoroutineCounter
type GoroutineCounter interface {
	NumGoroutine() int
}

type RealGoroutineCounter struct{}

func (r RealGoroutineCounter) NumGoroutine() int {
	return runtime.NumGoroutine()
}

type component struct {
	id       int
	name     string
	interval time.Duration
	lastRan  time.Time
	paused   bool

	rander                ComponentRand
	clock                 clock.Clock
	numGoroutineThreshold int
	goRoutineCounter      GoroutineCounter

	conn DbConn
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

// IntervalElapsed adds a random tiny drift [-1, 1] seconds to interval, when
// there are multiple ATCs, each ATC uses a slightly different interval, so
// that shorter interval gets more chance to run. This mechanism helps distribute
// component to across ATCs more evenly.
func (c *component) IntervalElapsed() bool {
	interval := c.interval
	drift := c.computeDrift()
	if interval+drift > 0 {
		interval += drift
	}
	return c.clock.Now().After(c.lastRan.Add(interval))
}

func (c *component) UpdateLastRan() error {
	var lastRan time.Time
	err := psql.Update("components").
		Set("last_ran", sq.Expr("now()")).
		Where(sq.Eq{
			"id": c.id,
		}).
		Suffix("RETURNING last_ran").
		RunWith(c.conn).
		QueryRow().Scan(&lastRan)
	if err != nil {
		return err
	}
	c.lastRan = lastRan
	return nil
}

const maxDrift = 5 * time.Second

// ComputeDrift computes a drift for components scheduler, the drift should help
// distribute workloads more evenly across ATCs. When numGoroutineThreshold
// is not set, drift will be a random value in range of [-1, 1] second. When
// numGoroutineThreshold is set, then drift will be in range of [-1, 5] seconds.
// Say numGoroutineThreshold is 50000:
//   - if current numGoroutine is 0 (which is impossible), drift should be -1 second
//   - if current numGoroutine is 100, drift should be close to -1 second
//   - if current numGoroutine is 50000 (equals to threshold), drift should be 0
//   - if current numGoroutine is 100000 (double to threshold), drift should be 1 second
//   - if current numGoroutine is 150000 (triple to threshold), drift should be 2 seconds
//   - and so on, but drift will be no longer than 5 seconds
func (c *component) computeDrift() time.Duration {
	if c.numGoroutineThreshold == 0 {
		drift := time.Duration(c.rander.Int())%(2*time.Second) - time.Second
		return drift
	}

	d := 2 * float64(c.goRoutineCounter.NumGoroutine()-c.numGoroutineThreshold) / float64(c.numGoroutineThreshold*2)
	drift := time.Millisecond * time.Duration(d*1000)
	if drift > maxDrift {
		drift = maxDrift
	}
	return drift
}

func scanComponent(c *component, row scannable) error {
	var (
		lastRan  sql.NullTime
		interval string
	)

	err := row.Scan(
		&c.id,
		&c.name,
		&interval,
		&lastRan,
		&c.paused,
	)
	if err != nil {
		return err
	}

	c.lastRan = lastRan.Time

	c.interval, err = time.ParseDuration(interval)
	if err != nil {
		return err
	}

	return nil
}
