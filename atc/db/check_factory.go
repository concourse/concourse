package db

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . CheckFactory

type CheckFactory interface {
	Checks() ([]Check, error)
	CreateCheck(int, string) (Check, error)
	CreateCheckFromVersion(int, string, atc.Version) (Check, error)
}

type checkFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewCheckFactory(conn Conn, lockFactory lock.LockFactory) CheckFactory {
	return &checkFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (r *checkFactory) Checks() ([]Check, error) {
	rows, err := checksQuery.
		OrderBy("r.id ASC").
		RunWith(r.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var checks []Check

	for rows.Next() {
		check := &check{conn: r.conn, lockFactory: r.lockFactory}

		err := scanCheck(check, rows)
		if err != nil {
			return nil, err
		}

		checks = append(checks, check)
	}

	return checks, nil
}

func (r *checkFactory) CreateCheck(reasourceID int, checkType string) (Check, error) {
	return nil, errors.New("nope")
}

func (r *checkFactory) CreateCheckFromVersion(reasourceID int, checkType string, fromVersion atc.Version) (Check, error) {
	return nil, errors.New("nope")
}
