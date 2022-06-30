package db

import (
	"database/sql"
	"math/rand"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
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

type component struct {
	id       int
	name     string
	interval time.Duration
	lastRan  time.Time
	paused   bool
	rander   *rand.Rand

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

	if c.rander == nil {
		c.rander = rand.New(rand.NewSource(time.Now().Unix()))
	}

	return true, nil
}

// IntervalElapsed adds a random tiny drift [-1, 1] seconds to interval, when
// there are multiple ATCs, each ATC uses a slightly different interval, so
// that shorter interval gets more chance to run. This mechanism helps distribute
// component to across ATCs more evenly.
func (c *component) IntervalElapsed() bool {
	interval := c.interval
	drift := time.Duration(c.rander.Int())%(2*time.Second) - time.Second
	if interval+drift > 0 {
		interval += drift
	}
	return time.Now().After(c.lastRan.Add(interval))
}

func (c *component) UpdateLastRan() error {
	_, err := psql.Update("components").
		Set("last_ran", sq.Expr("now()")).
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

func scanComponent(c *component, row scannable) error {
	var (
		lastRan  pq.NullTime
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
