package db

func (db *SQLDB) CreatePipe(pipeGUID string, url string, teamID int) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO pipes(id, url, team_id)
		VALUES ($1, $2, $3)
	`, pipeGUID, url, teamID)

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
		SELECT id, coalesce(url, '') AS url, team_id
		FROM pipes
		WHERE id = $1
	`, pipeGUID).Scan(&pipe.ID, &pipe.URL, &pipe.TeamID)

	if err != nil {
		return Pipe{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Pipe{}, err
	}

	return pipe, nil
}
