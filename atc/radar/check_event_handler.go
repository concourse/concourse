package radar

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func NewCheckEventHandler(logger lager.Logger, tx db.Tx, resourceConfigScope db.ResourceConfigScope, spaces map[atc.Space]atc.Version) *CheckEventHandler {
	return &CheckEventHandler{
		logger:              logger,
		tx:                  tx,
		resourceConfigScope: resourceConfigScope,
		spaces:              spaces,
	}
}

type CheckEventHandler struct {
	logger              lager.Logger
	tx                  db.Tx
	resourceConfigScope db.ResourceConfigScope
	spaces              map[atc.Space]atc.Version
}

func (c *CheckEventHandler) DefaultSpace(space atc.Space) error {
	if space != "" {
		err := c.resourceConfigScope.SaveDefaultSpace(space)
		if err != nil {
			c.logger.Error("failed-to-save-default-space", err, lager.Data{
				"space": space,
			})
			return err
		}

		c.logger.Debug("default-space-saved", lager.Data{
			"space": space,
		})
	}

	return nil
}

func (c *CheckEventHandler) Discovered(space atc.Space, version atc.Version, metadata atc.Metadata) error {
	if _, ok := c.spaces[space]; !ok {
		err := c.resourceConfigScope.SaveSpace(space)
		if err != nil {
			c.logger.Error("failed-to-save-space", err, lager.Data{
				"space": space,
			})
			return err
		}

		c.logger.Debug("space-saved", lager.Data{
			"space": space,
		})
	}

	err := c.resourceConfigScope.SavePartialVersion(space, version, metadata)
	if err != nil {
		c.logger.Error("failed-to-save-resource-config-version", err, lager.Data{
			"version": fmt.Sprintf("%v", version),
		})
		return err
	}

	c.logger.Debug("version-saved", lager.Data{
		"space":   space,
		"version": fmt.Sprintf("%v", version),
	})

	c.spaces[space] = version
	return nil
}

func (c *CheckEventHandler) LatestVersions() error {
	if len(c.spaces) == 0 {
		c.logger.Debug("no-new-versions")
		return nil
	}

	err := c.resourceConfigScope.FinishSavingVersions()
	if err != nil {
		return err
	}

	for space, version := range c.spaces {
		err := c.resourceConfigScope.SaveSpaceLatestVersion(space, version)
		if err != nil {
			return err
		}
	}

	return nil
}
