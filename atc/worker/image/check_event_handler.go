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

// When versions are discovered, they are not saved into the database because
// image resources are checked everytime in order to get the latest version of
// the image everytime it is run
func (c *CheckEventHandler) Discovered(space atc.Space, version atc.Version, metadata atc.Metadata) error {
	c.SavedLatestVersions[space] = version
	return nil
}

func (c *CheckEventHandler) LatestVersions() error {
	return nil
}
