package dbng

type ContainerState string

const (
	ContainerStateCreating   = "creating"
	ContainerStateCreated    = "created"
	ContainerStateDestroying = "destroying"
)

type CreatingContainer struct {
	ID         int
	Handle     string
	WorkerName string
	conn       Conn
}

type CreatedContainer struct {
	ID         int
	Handle     string
	WorkerName string
	conn       Conn
}

type DestroyingContainer struct {
	ID         int
	Handle     string
	WorkerName string
	conn       Conn
}
