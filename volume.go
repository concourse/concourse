package atc

type Volume struct {
	ID              string  `json:"id"`
	TTLInSeconds    int64   `json:"ttl_in_seconds"`
	ResourceVersion Version `json:"resource_version"`
	WorkerName      string  `json:"worker_name"`
}
