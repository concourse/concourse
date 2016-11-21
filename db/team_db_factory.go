package db

import (
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . TeamDBFactory

type TeamDBFactory interface {
	GetTeamDB(string) TeamDB
}

type teamDBFactory struct {
	conn        Conn
	bus         *notificationsBus
	lockFactory lock.LockFactory
}

func NewTeamDBFactory(conn Conn, bus *notificationsBus, lockFactory lock.LockFactory) TeamDBFactory {
	return &teamDBFactory{
		conn:        conn,
		bus:         bus,
		lockFactory: lockFactory,
	}
}

func (f *teamDBFactory) GetTeamDB(teamName string) TeamDB {
	return &teamDB{
		teamName:     teamName,
		conn:         f.conn,
		buildFactory: newBuildFactory(f.conn, f.bus, f.lockFactory),
	}
}
