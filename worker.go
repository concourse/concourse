package atc

type Worker struct {
	Addr string `json:"addr"`

	ActiveContainers int `json:"active_containers"`
}
