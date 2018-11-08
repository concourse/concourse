package image

import (
	"github.com/concourse/concourse/atc"
)

type CheckEventHandler struct {
	SavedDefaultSpace   atc.Space
	SavedLatestVersions map[atc.Space]atc.Version
}

func (c *CheckEventHandler) DefaultSpace(space atc.Space) error {
	c.SavedDefaultSpace = space
	return nil
}

func (c *CheckEventHandler) Discovered(space atc.Space, version atc.Version, metadata atc.Metadata) error {
	c.SavedLatestVersions[space] = version
	return nil
}

func (c *CheckEventHandler) LatestVersions() error {
	return nil
}
