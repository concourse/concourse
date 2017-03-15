package resource

import (
	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type checkRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

func (resource *resource) Check(source atc.Source, fromVersion atc.Version) ([]atc.Version, error) {
	var versions []atc.Version

	checking := ifrit.Invoke(resource.runScript(
		"/opt/resource/check",
		nil,
		checkRequest{source, fromVersion},
		&versions,
		nil,
		false,
	))

	err := <-checking.Wait()
	if err != nil {
		return nil, err
	}

	return versions, nil
}
