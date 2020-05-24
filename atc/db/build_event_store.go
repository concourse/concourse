package db

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
)

//go:generate counterfeiter . EventStore

type EventStore interface {
	Initialize(ctx context.Context, build Build) error
	Finalize(ctx context.Context, build Build) error

	Put(ctx context.Context, build Build, event []atc.Event) (EventKey, error)
	Get(ctx context.Context, build Build, requested int, cursor *EventKey) ([]event.Envelope, error)

	Delete(ctx context.Context, build []Build) error
	DeletePipeline(ctx context.Context, pipeline Pipeline) error
	DeleteTeam(ctx context.Context, team Team) error

	UnmarshalKey(data []byte, key *EventKey) error
}

type EventKey interface {
	Marshal() ([]byte, error)
	GreaterThan(EventKey) bool
}
