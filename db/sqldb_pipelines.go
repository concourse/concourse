package db

const pipelineColumns = "id, name, config, version, paused, team_id, public"

func (db *SQLDB) GetAllPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT ` + pipelineColumns + `
		FROM pipelines
		ORDER BY ordering
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	pipelines := []SavedPipeline{}

	for rows.Next() {
		pipeline, err := scanPipeline(rows)

		if err != nil {
			return nil, err
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}
