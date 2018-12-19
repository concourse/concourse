package atc

type WorkerArtifact struct {
	ID        int    `json:"id"`
	Path      string `json:"path"`
	Checksum  string `json:"checksum"`
	CreatedAt int64  `json:"created_at"`
}
