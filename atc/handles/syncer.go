package handles

// HandlesSyncer takes care of syncing the state between what the database sees
// as truth, and the worker.
//
type Syncer interface {
	Sync(handles []string, worker string) (err error)
}
