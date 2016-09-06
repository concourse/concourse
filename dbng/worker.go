package dbng

type Worker struct {
	Name       string
	GardenAddr string
}

func (worker *Worker) Create(tx Tx) error {
	_, err := psql.Insert("workers").
		Columns("name", "addr").
		Values(worker.Name, worker.GardenAddr).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return nil
}
