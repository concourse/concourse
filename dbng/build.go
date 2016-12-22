package dbng

import (
	sq "github.com/Masterminds/squirrel"
)

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusStarted   BuildStatus = "started"
	BuildStatusAborted   BuildStatus = "aborted"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusErrored   BuildStatus = "errored"
)

type Build struct {
	ID int
}

func (build *Build) SaveStatus(tx Tx, s BuildStatus) error {
	rows, err := psql.Update("builds").
		Set("status", string(s)).
		Where(sq.Eq{
			"id": build.ID,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		panic("TESTME")
		return nil
	}

	return nil
}
