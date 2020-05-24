package elasticsearch

import (
	"math/rand"
	"time"

	"github.com/concourse/concourse/atc/events"
	"github.com/jessevdk/go-flags"
)

func init() {
	events.Register("elasticsearch", StoreFactory{})
}

type StoreFactory struct {
}

func (s StoreFactory) AddConfig(group *flags.Group) events.Store {
	rand.Seed(time.Now().UnixNano())
	store := &Store{counter: rand.Int63()}

	subGroup, err := group.AddGroup("Elasticsearch Event Store", "", store)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "elasticsearch"

	return store
}
