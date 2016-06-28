package atc

type Volume struct {
	ID                string `json:"id"`
	TTLInSeconds      int64  `json:"ttl_in_seconds"`
	ValidityInSeconds int64  `json:"validity_in_seconds"`
	WorkerName        string `json:"worker_name"`
	Type              string `json:"type"`
	Identifier        string `json:"identifier"`
	SizeInBytes       int64  `json:"size_in_bytes"`
}
