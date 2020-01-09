package atc

// TODO-L Can this be consolidated with atc/runtime/types.go -> Artifact
type WorkerArtifact struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	BuildID   int    `json:"build_id"`
	CreatedAt int64  `json:"created_at"`
}
