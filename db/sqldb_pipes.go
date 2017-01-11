package db

func (db *SQLDB) CreatePipe(pipeGUID string, url string, teamName string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO pipes(id, url, team_id)
		VALUES (
			$1,
			$2,
			( SELECT id
				FROM teams
				WHERE name = $3
			)
		)
	`, pipeGUID, url, teamName)

	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) GetPipe(pipeGUID string) (Pipe, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Pipe{}, err
	}

	defer tx.Rollback()

	var pipe Pipe

	err = tx.QueryRow(`
		SELECT p.id AS pipe_id, coalesce(url, '') AS url, t.name AS team_name
		FROM pipes p
			JOIN teams t
			ON t.id = p.team_id
		WHERE p.id = $1
	`, pipeGUID).Scan(&pipe.ID, &pipe.URL, &pipe.TeamName)

	if err != nil {
		return Pipe{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Pipe{}, err
	}

	return pipe, nil
}
