package db

//go:generate counterfeiter . WorkerBaseResourceTypeFactory

type WorkerBaseResourceTypeFactory interface {
	Find(name string, worker Worker) (*UsedWorkerBaseResourceType, bool, error)
}

type workerBaseResourceTypeFactory struct {
	conn Conn
}

func NewWorkerBaseResourceTypeFactory(conn Conn) WorkerBaseResourceTypeFactory {
	return &workerBaseResourceTypeFactory{
		conn: conn,
	}
}

func (f *workerBaseResourceTypeFactory) Find(name string, worker Worker) (*UsedWorkerBaseResourceType, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	return WorkerBaseResourceType{
		Name:       name,
		WorkerName: worker.Name(),
	}.Find(tx)
}
