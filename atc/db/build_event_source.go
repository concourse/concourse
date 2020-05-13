package db

import (
	"errors"
	"sync"

	"github.com/concourse/concourse/atc/event"
)

var ErrEndOfBuildEventStream = errors.New("end of build event stream")
var ErrBuildEventStreamClosed = errors.New("build event stream closed")

//go:generate counterfeiter . EventSource

type EventSource interface {
	Next() (event.Envelope, error)
	Close() error
}

func newBuildEventSource(
	build Build,
	conn Conn,
	eventStore EventStore,
	notifier Notifier,
) *buildEventSource {
	wg := new(sync.WaitGroup)

	source := &buildEventSource{
		build:      build,
		eventStore: eventStore,

		conn: conn,

		notifier: notifier,

		events: make(chan event.Envelope, 2000),
		stop:   make(chan struct{}),
		wg:     wg,
	}

	wg.Add(1)
	go source.collectEvents()

	return source
}

type buildEventSource struct {
	build      Build
	eventStore EventStore

	conn     Conn
	notifier Notifier

	events chan event.Envelope
	stop   chan struct{}
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

func (source *buildEventSource) Close() error {
	select {
	case <-source.stop:
		return nil
	default:
		close(source.stop)
	}

	source.wg.Wait()

	return source.notifier.Close()
}

func (source *buildEventSource) collectEvents() {
	defer source.wg.Done()

	var batchSize = cap(source.events)

	var cursor Key
	for {
		select {
		case <-source.stop:
			source.err = ErrBuildEventStreamClosed
			close(source.events)
			return
		default:
		}

		var completed bool
		err := source.conn.QueryRow(`
			SELECT builds.completed
			FROM builds
			WHERE builds.id = $1
		`, source.build.ID()).Scan(&completed)
		if err != nil {
			source.err = err
			close(source.events)
			return
		}

		events, err := source.eventStore.Get(source.build, batchSize, &cursor)
		if err != nil {
			source.err = err
			close(source.events)
			return
		}

		for _, evt := range events {
			select {
			case source.events <- evt:
			case <-source.stop:
				source.err = ErrBuildEventStreamClosed
				close(source.events)
				return
			}
		}

		if len(events) >= batchSize {
			// still more events
			continue
		}

		if completed {
			source.err = ErrEndOfBuildEventStream
			close(source.events)
			return
		}

		select {
		case <-source.notifier.Notify():
		case <-source.stop:
			source.err = ErrBuildEventStreamClosed
			close(source.events)
			return
		}
	}
}
