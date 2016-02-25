package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type BuildPreparationStatus string

const (
	BuildPreparationStatusUnknown     BuildPreparationStatus = "unknown"
	BuildPreparationStatusBlocking    BuildPreparationStatus = "blocking"
	BuildPreparationStatusNotBlocking BuildPreparationStatus = "not_blocking"
)

type BuildPreparation struct {
	BuildID          int
	PausedPipeline   BuildPreparationStatus
	PausedJob        BuildPreparationStatus
	MaxRunningBuilds BuildPreparationStatus
	Inputs           map[string]BuildPreparationStatus
	InputsSatisfied  BuildPreparationStatus
}

func NewBuildPreparation(buildID int) BuildPreparation {
	return BuildPreparation{
		BuildID:          buildID,
		PausedPipeline:   BuildPreparationStatusUnknown,
		PausedJob:        BuildPreparationStatusUnknown,
		MaxRunningBuilds: BuildPreparationStatusUnknown,
		Inputs:           map[string]BuildPreparationStatus{},
		InputsSatisfied:  BuildPreparationStatusUnknown,
	}
}

type buildPreparationHelper struct{}

const BuildPreparationColumns string = "build_id, paused_pipeline, paused_job, max_running_builds, inputs, inputs_satisfied"

func (b buildPreparationHelper) CreateBuildPreparation(tx Tx, buildID int) error {
	_, err := tx.Exec(`
		INSERT INTO build_preparation (build_id)
		VALUES ($1)
	`, buildID)
	return err
}

func (b buildPreparationHelper) UpdateBuildPreparation(tx Tx, buildPrep BuildPreparation) error {
	inputsJSON, err := json.Marshal(buildPrep.Inputs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
	UPDATE build_preparation
	SET paused_pipeline = $2, paused_job = $3, max_running_builds = $4, inputs = $5, inputs_satisfied = $6
	WHERE build_id = $1
	`,
		buildPrep.BuildID,
		string(buildPrep.PausedPipeline),
		string(buildPrep.PausedJob),
		string(buildPrep.MaxRunningBuilds),
		string(inputsJSON),
		string(buildPrep.InputsSatisfied),
	)
	return err
}

func (b buildPreparationHelper) GetBuildPreparation(conn Conn, buildID int) (BuildPreparation, bool, error) {
	rows, err := conn.Query(fmt.Sprintf(`
			SELECT %s
			FROM build_preparation
			WHERE build_id = %d
		`, BuildPreparationColumns, buildID))
	if err != nil {
		return BuildPreparation{}, false, err
	}

	buildPreps, err := b.constructBuildPreparations(rows)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	switch len(buildPreps) {
	case 0:
		return BuildPreparation{}, false, nil
	case 1:
		return buildPreps[0], true, nil
	default:
		return BuildPreparation{}, false, errors.New(fmt.Sprintf("Found too many build preparations for build %d", buildID))
	}
}

func (b buildPreparationHelper) constructBuildPreparations(rows *sql.Rows) ([]BuildPreparation, error) {
	defer rows.Close()

	buildPreps := []BuildPreparation{}
	for rows.Next() {
		var buildID int
		var pausedPipeline, pausedJob, maxRunningBuilds, inputsSatisfied string
		var inputsBlob []byte

		err := rows.Scan(&buildID, &pausedPipeline, &pausedJob, &maxRunningBuilds, &inputsBlob, &inputsSatisfied)
		if err != nil {
			if err == sql.ErrNoRows {
				return []BuildPreparation{}, nil
			}
			return []BuildPreparation{}, err
		}

		var inputs map[string]BuildPreparationStatus
		err = json.Unmarshal(inputsBlob, &inputs)
		if err != nil {
			return []BuildPreparation{}, err
		}

		buildPreps = append(buildPreps, BuildPreparation{
			BuildID:          buildID,
			PausedPipeline:   BuildPreparationStatus(pausedPipeline),
			PausedJob:        BuildPreparationStatus(pausedJob),
			MaxRunningBuilds: BuildPreparationStatus(maxRunningBuilds),
			Inputs:           inputs,
			InputsSatisfied:  BuildPreparationStatus(inputsSatisfied),
		})
	}

	return buildPreps, nil
}
