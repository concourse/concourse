package elasticsearch

import (
	"github.com/concourse/concourse/atc/events"
	"github.com/jessevdk/go-flags"
)

func init() {
	events.Register("elasticsearch", StoreFactory{})
}

type StoreFactory struct {
}

func (s StoreFactory) AddConfig(group *flags.Group) events.Store {
	store := &Store{}

	subGroup, err := group.AddGroup("Elasticsearch Event Store", "", store)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "elasticsearch"

	return store
}
