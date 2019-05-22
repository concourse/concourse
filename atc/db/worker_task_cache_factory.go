package db

//go:generate counterfeiter . WorkerTaskCacheFactory

type WorkerTaskCacheFactory interface {
	FindOrCreate(WorkerTaskCache) (*UsedWorkerTaskCache, error)
	Find(WorkerTaskCache) (*UsedWorkerTaskCache, bool, error)
}

type workerTaskCacheFactory struct {
	conn Conn
}

func NewWorkerTaskCacheFactory(conn Conn) WorkerTaskCacheFactory {
	return &workerTaskCacheFactory{
		conn: conn,
	}
}

func (f *workerTaskCacheFactory) Find(workerTaskCache WorkerTaskCache) (*UsedWorkerTaskCache, bool, error) {
	return workerTaskCache.find(f.conn)
}

func (f *workerTaskCacheFactory) FindOrCreate(workerTaskCache WorkerTaskCache) (*UsedWorkerTaskCache, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	usedWorkerTaskCache, err := workerTaskCache.findOrCreate(tx)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedWorkerTaskCache, nil
}
