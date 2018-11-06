package radar

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func NewCheckEventHandler(logger lager.Logger, tx db.Tx, resourceConfig db.ResourceConfig, spaces map[atc.Space]atc.Version) *checkEventHandler {
	return &checkEventHandler{
		logger:         logger,
		tx:             tx,
		resourceConfig: resourceConfig,
		spaces:         spaces,
	}
}

type checkEventHandler struct {
	logger         lager.Logger
	tx             db.Tx
	resourceConfig db.ResourceConfig
	spaces         map[atc.Space]atc.Version
}

func (c *checkEventHandler) DefaultSpace(space atc.Space) error {
	if space != "" {
		err := c.resourceConfig.SaveDefaultSpace(c.tx, space)
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

func (c *checkEventHandler) Discovered(space atc.Space, version atc.Version, metadata atc.Metadata) error {
	if _, ok := c.spaces[space]; !ok {
		err := c.resourceConfig.SaveSpace(c.tx, space)
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

	err := c.resourceConfig.SaveVersion(c.tx, atc.SpaceVersion{space, version, metadata})
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

func (c *checkEventHandler) LatestVersions() error {
	if len(c.spaces) == 0 {
		c.logger.Debug("no-new-versions")
		return nil
	}

	for space, version := range c.spaces {
		err := c.resourceConfig.SaveSpaceLatestVersion(c.tx, space, version)
		if err != nil {
			return err
		}
	}

	return nil
}
