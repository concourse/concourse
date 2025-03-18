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

// ComputeDrift calculates a time offset (drift) for component schedulers to help
// distribute workloads more evenly across multiple ATCs (Air Traffic Controllers).
//
// Purpose:
// - Prevents thundering herd problems when multiple instances run on the same schedule
// - Allows busier instances to process jobs later, giving preference to less loaded instances
// - Creates a natural load-balancing effect across the system
//
// Calculation modes:
//
// 1. Random drift mode (numGoroutineThreshold == 0):
//   - Returns a random value in range [-1, 1) second
//   - Uses the rander to get a pseudo-random int value
//   - Converts to Duration and applies modulo to constrain range
//   - Subtracts 1 second to center the range around zero
//   - This mode ensures even statistical distribution of component execution
//
// 2. Load-based drift mode (numGoroutineThreshold > 0):
//   - Uses goroutine count as a proxy for system load
//   - Calculates drift proportional to relative load compared to threshold
//   - Formula: drift = (goroutineCount/threshold - 1) * second
//   - Range is capped at maxDrift (5 seconds) for very high loads
//
// Examples with numGoroutineThreshold = 50000:
//   - 0 goroutines:     drift = -1 second (runs earlier than scheduled)
//   - 25000 goroutines: drift = -0.5 seconds
//   - 50000 goroutines: drift = 0 seconds (runs exactly on schedule)
//   - 100000 goroutines: drift = +1 second (runs later than scheduled)
//   - 300000 goroutines: drift = +5 seconds (capped at maxDrift)
//
// Note: Negative drift means the component runs earlier than scheduled,
// while positive drift means it runs later, deferring to less loaded instances.
func (c *component) computeDrift() time.Duration {
	if c.numGoroutineThreshold == 0 {
		return time.Duration(c.rander.Int())%(2*time.Second) - time.Second
	}

	d := 2 * float64(c.goRoutineCounter.NumGoroutine()-c.numGoroutineThreshold) / float64(c.numGoroutineThreshold*2)
	drift := time.Millisecond * time.Duration(d*1000)

	// Cap the drift
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
