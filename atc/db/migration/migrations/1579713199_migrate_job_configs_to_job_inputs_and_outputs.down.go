package migrations

func (m *migrations) Down_1574452410() error {
	tx := m.Tx

	_, err := tx.Exec("TRUNCATE TABLE job_inputs")
	if err != nil {
		return err
	}

	_, err = tx.Exec("TRUNCATE TABLE job_outputs")
	if err != nil {
		return err
	}

	return nil
}
