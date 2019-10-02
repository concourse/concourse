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
	Interval() string
	LastRan() time.Time
	Paused() bool

	Reload() (bool, error)
	IntervalElapsed() bool
	UpdateLastRan() error
}

type component struct {
	id       int
	name     string
	interval string
	lastRan  time.Time
	paused   bool

	conn Conn
}

func (c *component) ID() int            { return c.id }
func (c *component) Name() string       { return c.name }
func (c *component) Interval() string   { return c.interval }
func (c *component) LastRan() time.Time { return c.lastRan }
func (c *component) Paused() bool       { return c.paused }

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
	inveral, _ := time.ParseDuration(c.interval)

	return time.Now().After(c.lastRan.Add(inveral))
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
		lastRan pq.NullTime
	)

	err := row.Scan(
		&c.id,
		&c.name,
		&c.interval,
		&lastRan,
		&c.paused,
	)
	if err != nil {
		return err
	}

	c.lastRan = lastRan.Time

	return nil
}
