package db

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
)

var ErrEndOfBuildEventStream = errors.New("end of build event stream")
var ErrBuildEventStreamClosed = errors.New("build event stream closed")

//go:generate counterfeiter . EventSource

type EventSource interface {
	Next() (event.Envelope, error)
	Close() error
}

func newBuildEventSource(
	buildID int,
	table string,
	conn Conn,
	notifier Notifier,
	from uint,
) *buildEventSource {
	wg := new(sync.WaitGroup)

	source := &buildEventSource{
		buildID: buildID,
		table:   table,

		conn: conn,

		notifier: notifier,

		events: make(chan event.Envelope, 2000),
		stop:   make(chan struct{}),
		wg:     wg,
	}

	wg.Add(1)
	go source.collectEvents(from)

	return source
}

type buildEventSource struct {
	buildID int
	table   string

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

func (source *buildEventSource) collectEvents(cursor uint) {
	defer source.wg.Done()

	var batchSize = cap(source.events)

	for {
		select {
		case <-source.stop:
			source.err = ErrBuildEventStreamClosed
			close(source.events)
			return
		default:
		}

		completed := false

		err := source.conn.QueryRow(`
			SELECT builds.completed
			FROM builds
			WHERE builds.id = $1
		`, source.buildID).Scan(&completed)
		if err != nil {
			source.err = err
			close(source.events)
			return
		}

		rows, err := source.conn.Query(`
			SELECT type, version, payload
			FROM `+source.table+`
			WHERE build_id = $1
			ORDER BY event_id ASC
			OFFSET $2
			LIMIT $3
		`, source.buildID, cursor, batchSize)
		if err != nil {
			source.err = err
			close(source.events)
			return
		}

		rowsReturned := 0

		for rows.Next() {
			rowsReturned++

			cursor++

			var t, v, p string
			err := rows.Scan(&t, &v, &p)
			if err != nil {
				_ = rows.Close()

				source.err = err
				close(source.events)
				return
			}

			data := json.RawMessage(p)

			ev := event.Envelope{
				Data:    &data,
				Event:   atc.EventType(t),
				Version: atc.EventVersion(v),
			}

			select {
			case source.events <- ev:
			case <-source.stop:
				_ = rows.Close()

				source.err = ErrBuildEventStreamClosed
				close(source.events)
				return
			}
		}

		if rowsReturned == batchSize {
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
