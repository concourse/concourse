package resources

type Pipe struct {
	ID string `json:"id"`

	PeerAddr string `json:"peer_addr"`
}

type Build struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`

	JobName string `json:"job_name"`
}
