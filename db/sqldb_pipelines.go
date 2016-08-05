package db

//go:generate counterfeiter . PipelinesDB

type PipelinesDB interface {
	GetAllPublicPipelines() ([]SavedPipeline, error)
}

const pipelineColumns = "p.id, p.name, p.config, p.version, p.paused, p.team_id, p.public, t.name as team_name"
const unqualifiedPipelineColumns = "id, name, config, version, paused, team_id, public"

func (db *SQLDB) GetAllPublicPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT ` + pipelineColumns + `
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.public = true
		ORDER BY team_name, ordering
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanPipelines(rows)
}

func (db *SQLDB) GetAllPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT ` + pipelineColumns + `
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		ORDER BY ordering
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanPipelines(rows)
}

func (db *SQLDB) GetPipelineByID(pipelineID int) (SavedPipeline, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
	`, pipelineID)

	return scanPipeline(row)
}
