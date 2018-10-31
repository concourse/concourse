package v1

import (
	"context"

	"github.com/concourse/concourse/atc"
)

type checkRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

func (r *Resource) Check(ctx context.Context, src atc.Source, fromVersion atc.Version) ([]atc.Version, error) {
	var versions []atc.Version

	err := RunScript(
		ctx,
		"/opt/resource/check",
		nil,
		checkRequest{src, fromVersion},
		&versions,
		nil,
		false,
		r.Container,
	)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
