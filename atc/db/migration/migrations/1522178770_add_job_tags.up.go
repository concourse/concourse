package migrations

import (
	"database/sql"
	"encoding/json"
	"strings"
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

	rows, err := tx.Query("SELECT id, groups FROM pipelines")
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
		jobGroups := make(map[string][]string)

		for _, group := range pipeline.Groups {
			for _, job := range group.Jobs {
				jobGroups[job] = append(jobGroups[job], group.Name)
			}
		}

		for job, groups := range jobGroups {
			_, err = tx.Exec(`
					UPDATE jobs
					SET tags = '{`+strings.Join(groups, ",")+`}'
					WHERE pipeline_id = $1
					AND name = $2
				`, pipeline.ID, job)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
