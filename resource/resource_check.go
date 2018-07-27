package resource

import (
	"context"

	"github.com/concourse/atc"
)

type checkRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

func (resource *resource) Check(source atc.Source, fromVersion atc.Version) ([]atc.Version, error) {
	var versions []atc.Version

	err := resource.runScript(
		context.TODO(),
		"/opt/resource/check",
		nil,
		checkRequest{source, fromVersion},
		&versions,
		nil,
		false,
	)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
