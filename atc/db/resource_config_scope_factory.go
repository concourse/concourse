package db

import "github.com/concourse/concourse/atc/db/lock"

type ResourceConfigScopeFactory interface {
	FindResourceConfigScopeByID(int) (ResourceConfigScope, bool, error)
}

type resourceConfigScopeFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewResourceConfigScope(conn Conn, lockFactory lock.LockFactory) ResourceConfigScopeFactory {
	return &resourceConfigScopeFactory{
		conn:	conn,
		lockFactory: lockFactory,
	}
}
