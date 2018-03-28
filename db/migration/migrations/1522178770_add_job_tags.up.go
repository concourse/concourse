package migrations

import (
	"database/sql"
	"encoding/json"
)

type GroupConfig struct {
	Name string   `json:"name"`
	Jobs []string `json:"jobs,omitempty"`
}

type GroupConfigs []GroupConfig

type Pipeline struct {
	ID     int          `json:"id"`
	Groups GroupConfigs `json:"groups,omitempty"`
}

func (self *migrations) Up_1522178770() error {
	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.Query("SELECT id, name FROM jobs")
	if err != nil {
		return err
	}

	jobs := make(map[string]int)
	for rows.Next() {
		var id int
		var name string

		err = rows.Scan(&id, &name)
		if err != nil {
			return err
		}

		jobs[name] = id
	}

	rows, err = tx.Query("SELECT id, groups FROM pipelines")
	if err != nil {
		return err
	}

	pipelines := []Pipeline{}
	for rows.Next() {
		pipeline := Pipeline{}

		var groups sql.NullString
		err = rows.Scan(&pipeline.ID, &groups)
		if err != nil {
			return err
		}

		if groups.Valid {
			var pipelineGroups GroupConfigs

			err = json.Unmarshal([]byte(groups.String), &pipelineGroups)
			if err != nil {
				return err
			}

			pipeline.Groups = pipelineGroups
		}

		pipelines = append(pipelines, pipeline)
	}

	for _, pipeline := range pipelines {
		for _, group := range pipeline.Groups {
			for _, job := range group.Jobs {
				_, err = tx.Exec(`
					INSERT INTO job_tags(job_id, tag)
					VALUES ($1, $2)
				`, jobs[job], group.Name)
				if err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}
