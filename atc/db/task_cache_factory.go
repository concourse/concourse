package db

//go:generate counterfeiter . TaskCacheFactory

type TaskCacheFactory interface {
	Find(jobID int, stepName string, path string) (UsedTaskCache, bool, error)
	FindOrCreate(jobID int, stepName string, path string) (UsedTaskCache, error)
}

type taskCacheFactory struct {
	conn Conn
}

func NewTaskCacheFactory(conn Conn) TaskCacheFactory {
	return &taskCacheFactory{
		conn: conn,
	}
}

func (f *taskCacheFactory) Find(jobID int, stepName string, path string) (UsedTaskCache, bool, error) {
	return usedTaskCache{
		jobID:    jobID,
		stepName: stepName,
		path:     path,
	}.find(f.conn)
}

func (f *taskCacheFactory) FindOrCreate(jobID int, stepName string, path string) (UsedTaskCache, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	utc, err := usedTaskCache{
		jobID:    jobID,
		stepName: stepName,
		path:     path,
	}.findOrCreate(tx)

	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return utc, nil
}
