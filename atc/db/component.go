package db

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

var componentsQuery = psql.Select("c.id, c.name, c.interval, c.last_ran, c.paused").
	From("components c")

//go:generate counterfeiter . Component

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
	return time.Now().After(c.lastRan.Add(c.interval))
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
