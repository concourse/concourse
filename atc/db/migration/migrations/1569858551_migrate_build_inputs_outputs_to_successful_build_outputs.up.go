package migrations

import (
	"encoding/json"
	"strconv"
)

func (self *migrations) Up_1562949427() error {
	type output struct {
		name       string
		resourceID int
		version    string
		buildID    int
		jobID      int
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	rows, err := tx.Query("SELECT i.name, i.resource_id, i.version_md5, i.build_id, b.job_id FROM build_resource_config_version_inputs i JOIN builds b ON b.id = i.build_id WHERE b.status = 'succeeded'")
	if err != nil {
		return err
	}

	oldInputs := []output{}
	for rows.Next() {
		o := output{}

		if err = rows.Scan(&o.name, &o.resourceID, &o.version, &o.buildID, &o.jobID); err != nil {
			return err
		}

		oldInputs = append(oldInputs, o)
	}

	rows, err = tx.Query("SELECT o.name, o.resource_id, o.version_md5, o.build_id, b.job_id FROM build_resource_config_version_outputs o JOIN builds b ON b.id = o.build_id WHERE b.status = 'succeeded'")
	if err != nil {
		return err
	}

	oldOutputs := []output{}
	for rows.Next() {
		o := output{}

		if err = rows.Scan(&o.name, &o.resourceID, &o.version, &o.buildID, &o.jobID); err != nil {
			return err
		}

		oldOutputs = append(oldOutputs, o)
	}

	buildSucceededOutputs := map[int]map[string][]string{}
	buildToJobID := map[int]int{}
	for _, o := range oldOutputs {
		outputs, ok := buildSucceededOutputs[o.buildID]
		if !ok {
			outputs = map[string][]string{}
			buildSucceededOutputs[o.buildID] = outputs
		}

		key := strconv.Itoa(o.resourceID)

		exists := false
		for _, version := range outputs[key] {
			if version == o.version {
				exists = true
			}
		}

		if !exists {
			outputs[key] = append(outputs[key], o.version)
		}

		buildToJobID[o.buildID] = o.jobID
	}

	for _, i := range oldInputs {
		outputs, ok := buildSucceededOutputs[i.buildID]
		if !ok {
			outputs = map[string][]string{}
			buildSucceededOutputs[i.buildID] = outputs
		}

		key := strconv.Itoa(i.resourceID)

		exists := false
		for _, version := range outputs[key] {
			if version == i.version {
				exists = true
			}
		}

		if !exists {
			outputs[key] = append(outputs[key], i.version)
		}

		buildToJobID[i.buildID] = i.jobID
	}

	for buildID, thinger := range buildSucceededOutputs {
		jsonThing, err := json.Marshal(thinger)
		if err != nil {
			return err
		}

		_, err = tx.Exec("INSERT INTO successful_build_outputs (build_id, job_id, outputs) VALUES ($1, $2, $3::json)", buildID, buildToJobID[buildID], jsonThing)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
