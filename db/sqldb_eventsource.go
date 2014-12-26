package db

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

func newSQLDBBuildEventSource(
	buildID int,
	conn *sql.DB,
	notificationsBus *notificationsBus,
	notify chan struct{},
	from uint,
) *sqldbBuildEventSource {
	wg := new(sync.WaitGroup)

	source := &sqldbBuildEventSource{
		buildID: buildID,

		conn: conn,

		notify: notify,
		bus:    notificationsBus,

		events: make(chan BuildEvent, 20),
		stop:   make(chan struct{}),
		wg:     wg,
	}

	wg.Add(1)
	go source.collectEvents(from)

	return source
}

type sqldbBuildEventSource struct {
	buildID int

	conn     *sql.DB
	listener *pq.Listener

	notify chan struct{}
	bus    *notificationsBus

	events chan BuildEvent
	stop   chan struct{}
	err    error
	wg     *sync.WaitGroup
}

func (source *sqldbBuildEventSource) Next() (BuildEvent, error) {
	select {
	case e, ok := <-source.events:
		if !ok {
			return BuildEvent{}, source.err
		}

		return e, nil
	}
}

func (source *sqldbBuildEventSource) Close() error {
	select {
	case <-source.stop:
		return nil
	default:
		close(source.stop)
	}

	source.wg.Wait()

	channel := buildEventsChannel(source.buildID)
	return source.bus.Unlisten(channel, source.notify)
}

func (source *sqldbBuildEventSource) collectEvents(cursor uint) {
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

		rows, err := source.conn.Query(`
			SELECT event_id, type, payload, version
			FROM build_events
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

			var event BuildEvent
			err := rows.Scan(&event.ID, &event.Type, &event.Payload, &event.Version)
			if err != nil {
				rows.Close()

				source.err = err
				close(source.events)
				return
			}

			select {
			case source.events <- event:
			case <-source.stop:
				rows.Close()

				source.err = ErrBuildEventStreamClosed
				close(source.events)
				return
			}
		}

		if rowsReturned == batchSize {
			// still potentially more events; keep going
			continue
		}

		var completed bool
		var lastEventID uint
		err = source.conn.QueryRow(`
			SELECT builds.completed, coalesce(max(build_events.event_id), 0)
			FROM builds
			LEFT JOIN build_events
			ON build_events.build_id = builds.id
			WHERE builds.id = $1
			GROUP BY builds.id
		`, source.buildID).Scan(&completed, &lastEventID)
		if err != nil {
			source.err = err
			close(source.events)
			return
		} else if completed {
			if cursor > lastEventID {
				source.err = ErrEndOfBuildEventStream
				close(source.events)
				return
			}
		}

		select {
		case <-source.notify:
		case <-source.stop:
			source.err = ErrBuildEventStreamClosed
			close(source.events)
			return
		}
	}
}
