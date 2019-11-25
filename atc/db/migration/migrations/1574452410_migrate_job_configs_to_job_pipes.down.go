package migrations

func (self *migrations) Down_1574452410() error {
	_, err := self.DB.Exec("TRUNCATE TABLE job_pipes")
	if err != nil {
		return err
	}

	return nil
}

