package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func WorkerArtifacts(artifacts []db.WorkerArtifact) []atc.WorkerArtifact {
	wa := []atc.WorkerArtifact{}
	for _, a := range artifacts {
		wa = append(wa, WorkerArtifact(a))
	}
	return wa
}

func WorkerArtifact(artifact db.WorkerArtifact) atc.WorkerArtifact {
	return atc.WorkerArtifact{
		ID:        artifact.ID(),
		Name:      artifact.Name(),
		BuildID:   artifact.BuildID(),
		CreatedAt: artifact.CreatedAt().Unix(),
	}
}
