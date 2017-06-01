package db

import "fmt"

func (db *SQLDB) CreateTeam(team Team) (SavedTeam, error) {
	savedTeam, err := scanTeam(db.conn.QueryRow(`
	INSERT INTO teams (
    name
	) VALUES (
		$1
	)
	RETURNING id, name, admin
	`, team.Name))
	if err != nil {
		return SavedTeam{}, err
	}

	createTableString := fmt.Sprintf(`
		CREATE TABLE team_build_events_%d ()
		INHERITS (build_events);`, savedTeam.ID)
	_, err = db.conn.Exec(createTableString)
	if err != nil {
		return SavedTeam{}, err
	}

	return savedTeam, nil
}

func scanTeam(rows scannable) (SavedTeam, error) {
	var savedTeam SavedTeam

	err := rows.Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
	)
	if err != nil {
		return savedTeam, err
	}

	return savedTeam, nil
}
