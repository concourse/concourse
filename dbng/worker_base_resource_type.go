package dbng

type WorkerBaseResourceType struct {
	Name    string
	Version string
}

type UsedWorkerBaseResourceType struct {
	ID      int
	Name    string
	Version string

	Worker Worker
}
