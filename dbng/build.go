package dbng

import (
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
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

	pipelineID int
	teamID     int
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

func (build *Build) Finish(tx Tx, status BuildStatus) error {
	var endTime time.Time

	err := tx.QueryRow(`
		UPDATE builds
		SET status = $2, end_time = now(), completed = true
		WHERE id = $1
		RETURNING end_time
	`, build.ID, string(status)).Scan(&endTime)
	if err != nil {
		return err
	}

	err = build.saveEvent(tx, event.Status{
		Status: atc.BuildStatus(status),
		Time:   endTime.Unix(),
	})
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		DROP SEQUENCE %s
	`, buildEventSeq(build.ID)))
	if err != nil {
		return err
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

func (build *Build) saveEvent(tx Tx, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	table := fmt.Sprintf("team_build_events_%d", build.teamID)
	if build.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", build.pipelineID)
	}

	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO %s (event_id, build_id, type, version, payload)
		VALUES (nextval('%s'), $1, $2, $3, $4)
	`, table, buildEventSeq(build.ID)), build.ID, string(event.EventType()), string(event.Version()), payload)
	if err != nil {
		return err
	}

	return nil
}

func createBuildEventSeq(tx Tx, buildID int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(buildID)))
	return err
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
