package migrations

import (
	"errors"
)

func (m *migrations) Down_1528470872() error {
	tx := m.Tx
	var count int
	err := tx.QueryRow("SELECT count(*) FROM teams WHERE legacy_auth IS NULL AND name != 'main'").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("changes have been made to the teams table since the 'global users' upgrade. There is no turning back")
	}

	_, err = tx.Exec("ALTER TABLE teams DROP COLUMN auth")
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE teams RENAME COLUMN legacy_auth TO auth")
	if err != nil {
		return err
	}

	return nil
}
