package db

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/concourse/concourse/atc/event"
)

var ErrEndOfBuildEventStream = errors.New("end of build event stream")
var ErrBuildEventStreamClosed = errors.New("build event stream closed")

// Used for control flow
var errReconnectedToNotificationBus = errors.New("reconnected to notification bus")

type EventNotification struct {
	Event event.Envelope `json:"event"`
	Key   []byte         `json:"key"`
}

//go:generate counterfeiter . EventSource

type EventSource interface {
	Next() (event.Envelope, error)
}

func newBuildEventSource(
	ctx context.Context,
	build Build,
	conn Conn,
	eventStore EventStore,
	notifications chan Notification,
) *buildEventSource {
	wg := new(sync.WaitGroup)

	source := &buildEventSource{
		build:      build,
		eventStore: eventStore,

		conn:          conn,
		notifications: notifications,

		events: make(chan event.Envelope, 2000),
		wg:     wg,
	}

	wg.Add(1)
	go source.collectEvents(ctx)

	return source
}

type buildEventSource struct {
	build      Build
	eventStore EventStore

	conn          Conn
	notifications chan Notification

	events chan event.Envelope
	err    error
	wg     *sync.WaitGroup
}

func (source *buildEventSource) Next() (event.Envelope, error) {
	e, ok := <-source.events
	if !ok {
		return event.Envelope{}, source.err
	}

	return e, nil
}

func (source *buildEventSource) collectEvents(ctx context.Context) {
	defer source.wg.Done()

	var cursor Key

start:
	if err := source.collectExistingEvents(ctx, &cursor); err != nil {
		source.err = err
		close(source.events)
		return
	}
	if err := source.watchNotificationBus(ctx, &cursor); err != nil {
		if errors.Is(err, errReconnectedToNotificationBus) {
			// we may have missed a notification while reconnecting, so
			// collect existing events from where we left off
			goto start
		}
		source.err = err
		close(source.events)
	}
}

func (source *buildEventSource) collectExistingEvents(ctx context.Context, cursor *Key) error {
	batchSize := cap(source.events)

	for {
		select {
		case <-ctx.Done():
			return ErrBuildEventStreamClosed
		default:
		}

		events, err := source.eventStore.Get(ctx, source.build, batchSize, cursor)
		if err != nil {
			return err
		}

		for _, evt := range events {
			select {
			case source.events <- evt:
			case <-ctx.Done():
				return ErrBuildEventStreamClosed
			}
		}

		if len(events) < batchSize {
			// no more events stored (the remainder will come from the notification bus)
			return nil
		}
	}
}

func (source *buildEventSource) watchNotificationBus(ctx context.Context, cursor *Key) error {
	completedChan := make(chan struct{}, 1)
	var completed bool
	for {
		select {
		case <-ctx.Done():
			return ErrBuildEventStreamClosed
		default:
		}

		if !completed {
			err := source.conn.QueryRowContext(ctx, `
				SELECT builds.completed
				FROM builds
				WHERE builds.id = $1
			`, source.build.ID()).Scan(&completed)
			if err != nil {
				return err
			}

			if completed {
				completedChan <- struct{}{}
			}
		}

		select {
		case notification := <-source.notifications:
			if !notification.Healthy {
				return errReconnectedToNotificationBus
			}
			var eventNotif EventNotification
			if err := json.Unmarshal([]byte(notification.Payload), &eventNotif); err != nil {
				// TODO: what to do in this case? should at least log it
				continue
			}
			var incomingKey Key
			if err := source.eventStore.UnmarshalKey(eventNotif.Key, &incomingKey); err != nil {
				// TODO: what to do in this case? should at least log it
				continue
			}
			if incomingKey != nil && !incomingKey.GreaterThan(*cursor) {
				// This can happen if we reconnected to the notification bus, so we called "Get" on the
				// EventStore in case we missed anything - and we did miss events, but they were already
				// queued on the notification bus. Don't want to send them twice!
				continue
			}
			*cursor = incomingKey
			source.events <- eventNotif.Event
		case <-ctx.Done():
			return ErrBuildEventStreamClosed
		case <-completedChan:
			return ErrEndOfBuildEventStream
		}
	}
}
