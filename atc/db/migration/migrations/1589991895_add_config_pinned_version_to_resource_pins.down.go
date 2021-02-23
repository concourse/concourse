package migrations

func (m *migrations) Down_1585079293() error {
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM resource_pins WHERE config = true")
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE resource_pins DROP COLUMN config")
	if err != nil {
		return err
	}

	return tx.Commit()
}
