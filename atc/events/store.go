package events

import (
	"context"

	"github.com/concourse/concourse/atc/db"
	"github.com/jessevdk/go-flags"
)

type Store interface {
	IsConfigured() bool

	Setup(ctx context.Context, conn db.Conn) error
	Close(ctx context.Context) error

	db.EventStore
}

type StoreFactory interface {
	AddConfig(*flags.Group) Store
}

type Stores map[string]Store

var StoreFactories = make(map[string]StoreFactory)

func Register(name string, factory StoreFactory) {
	StoreFactories[name] = factory
}
