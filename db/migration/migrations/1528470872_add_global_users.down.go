package migrations

import (
	"errors"
)

func (self *migrations) Down_1528470872() error {
	var count int
	err := self.DB.QueryRow("SELECT count(*) FROM teams WHERE legacy_auth IS NULL AND name != 'main'").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("Changes have been made to the teams table since the 'global users' upgrade. There is no turning back.")
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE teams DROP COLUMN auth")
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("ALTER TABLE teams RENAME COLUMN legacy_auth TO auth")
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}
