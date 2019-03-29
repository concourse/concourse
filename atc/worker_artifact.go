package atc

type WorkerArtifact struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	BuildID   int    `json:"build_id"`
	CreatedAt int64  `json:"created_at"`
}
