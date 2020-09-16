package accessor

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/patrickmn/go-cache"
)

//go:generate counterfeiter . Notifications

type Notifications interface {
	Listen(string) (chan bool, error)
	Unlisten(string, chan bool) error
}

type teamsCacher struct {
	logger        lager.Logger
	cache         *cache.Cache
	notifications Notifications
	teamFactory   db.TeamFactory
}

func NewTeamsCacher(
	logger lager.Logger,
	notifications Notifications,
	teamFactory db.TeamFactory,
	expiration time.Duration,
	cleanupInterval time.Duration,
) *teamsCacher {
	c := &teamsCacher{
		logger:        logger,
		cache:         cache.New(expiration, cleanupInterval),
		notifications: notifications,
		teamFactory:   teamFactory,
	}

	go c.waitForNotifications()

	return c
}

func (c *teamsCacher) GetTeams() ([]db.Team, error) {
	if teams, found := c.cache.Get(atc.TeamCacheName); found {
		return teams.([]db.Team), nil
	}

	teams, err := c.teamFactory.GetTeams()
	if err != nil {
		return nil, err
	}

	c.cache.Set(atc.TeamCacheName, teams, cache.DefaultExpiration)

	return teams, nil
}

func (c *teamsCacher) waitForNotifications() {
	notifier, err := c.notifications.Listen(atc.TeamCacheChannel)
	if err != nil {
		c.logger.Error("failed-to-listen-for-team-cache", err)
	}

	defer c.notifications.Unlisten(atc.TeamCacheChannel, notifier)

	for {
		select {
		case <-notifier:
			c.cache.Delete(atc.TeamCacheName)
		}
	}
}
