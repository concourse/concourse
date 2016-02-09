package db

import (
	"database/sql"
	"encoding/json"
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
	Inputs           map[string]string
}

type buildPreparationHelper struct{}

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
	SET paused_pipeline = $2, paused_job = $3, max_running_builds = $4, inputs = $5
	WHERE build_id = $1
	`,
		buildPrep.BuildID,
		string(buildPrep.PausedPipeline),
		string(buildPrep.PausedJob),
		string(buildPrep.MaxRunningBuilds),
		string(inputsJSON),
	)
	return err
}

func (b buildPreparationHelper) GetBuildPreparation(conn Conn, passedBuildID int) (BuildPreparation, bool, error) {
	row := conn.QueryRow(`
			SELECT build_id, paused_pipeline, paused_job, max_running_builds, inputs
			FROM build_preparation
			WHERE build_id = $1
		`, passedBuildID)

	var buildID int
	var pausedPipeline, pausedJob, maxRunningBuilds string
	var inputsBlob []byte

	err := row.Scan(&buildID, &pausedPipeline, &pausedJob, &maxRunningBuilds, &inputsBlob)
	if err != nil {
		if err == sql.ErrNoRows {
			return BuildPreparation{}, false, nil
		}
		return BuildPreparation{}, false, err
	}

	var inputs map[string]string
	err = json.Unmarshal(inputsBlob, &inputs)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	return BuildPreparation{
		BuildID:          buildID,
		PausedPipeline:   BuildPreparationStatus(pausedPipeline),
		PausedJob:        BuildPreparationStatus(pausedJob),
		MaxRunningBuilds: BuildPreparationStatus(maxRunningBuilds),
		Inputs:           inputs,
	}, true, nil
}
