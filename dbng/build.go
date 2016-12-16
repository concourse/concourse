package dbng

import (
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

type Build struct {
	ID int
}

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusStarted   BuildStatus = "started"
	BuildStatusAborted   BuildStatus = "aborted"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusErrored   BuildStatus = "errored"
)

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

func (build *Build) Delete(tx Tx) (bool, error) {
	rows, err := psql.Delete("builds").
		Where(sq.Eq{
			"id": build.ID,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}

	if affected == 0 {
		panic("TESTME")
		return false, nil
	}

	return true, nil
}

func (b *Build) SaveImageResourceVersion(tx Tx, planID atc.PlanID, resourceVersion atc.Version, resourceHash string) error {
	version, err := json.Marshal(resourceVersion)
	if err != nil {
		return err
	}

	rows, err := psql.Update("image_resource_versions").
		Set("version", version).
		Set("resource_hash", resourceHash).
		Where(sq.Eq{
			"build_id": b.ID,
			"plan_id":  string(planID),
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
		_, err := psql.Insert("image_resource_versions").
			Columns("version", "build_id", "plan_id", "resource_hash").
			Values(version, b.ID, string(planID), resourceHash).
			RunWith(tx).
			Exec()
		if err != nil {
			// TODO: handle unique violation err
			return err
		}
	}

	return nil
}
