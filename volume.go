package atc

type Volume struct {
	ID              string `json:"id"`
	WorkerName      string `json:"worker_name"`
	Type            string `json:"type"`
	Identifier      string `json:"identifier"`
	SizeInBytes     int64  `json:"size_in_bytes"`
	ContainerHandle string `json:"container_handle"`
	ParentHandle    string `json:"parent_handle"`
}
