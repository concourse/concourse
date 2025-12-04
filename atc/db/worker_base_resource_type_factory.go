package db

//counterfeiter:generate . WorkerBaseResourceTypeFactory
type WorkerBaseResourceTypeFactory interface {
	Find(name string, worker Worker) (*UsedWorkerBaseResourceType, bool, error)
}

type workerBaseResourceTypeFactory struct {
	conn DbConn
}

func NewWorkerBaseResourceTypeFactory(conn DbConn) WorkerBaseResourceTypeFactory {
	return &workerBaseResourceTypeFactory{
		conn: conn,
	}
}

func (f *workerBaseResourceTypeFactory) Find(name string, worker Worker) (*UsedWorkerBaseResourceType, bool, error) {
	return WorkerBaseResourceType{
		Name:       name,
		WorkerName: worker.Name(),
	}.Find(f.conn)
}
