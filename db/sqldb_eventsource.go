package db

import (
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
)

func newSQLDBBuildEventSource(
	buildID int,
	table string,
	conn Conn,
	notifier Notifier,
	from uint,
) *sqldbBuildEventSource {
	wg := new(sync.WaitGroup)

	source := &sqldbBuildEventSource{
		buildID: buildID,
		table:   table,

		conn: conn,

		notifier: notifier,

		events: make(chan atc.Event, 20),
		stop:   make(chan struct{}),
		wg:     wg,
	}

	wg.Add(1)
	go source.collectEvents(from)

	return source
}

type sqldbBuildEventSource struct {
	buildID int
	table   string

	conn     Conn
	notifier Notifier

	events chan atc.Event
	stop   chan struct{}
	err    error
	wg     *sync.WaitGroup
}

func (source *sqldbBuildEventSource) Next() (atc.Event, error) {
	select {
	case e, ok := <-source.events:
		if !ok {
			return nil, source.err
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

	return source.notifier.Close()
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
				rows.Close()

				source.err = err
				close(source.events)
				return
			}

			ev, err := event.ParseEvent(atc.EventVersion(v), atc.EventType(t), []byte(p))
			if err != nil {
				rows.Close()

				source.err = err
				close(source.events)
				return
			}

			select {
			case source.events <- ev:
			case <-source.stop:
				rows.Close()

				source.err = ErrBuildEventStreamClosed
				close(source.events)
				return
			}
		}

		if rowsReturned == batchSize {
			// still more events
			continue
		}

		var completed bool
		var lastEventID uint
		err = source.conn.QueryRow(`
			SELECT builds.completed, coalesce(max(`+source.table+`.event_id), 0)
			FROM builds
			LEFT JOIN `+source.table+`
			ON `+source.table+`.build_id = builds.id
			WHERE builds.id = $1
			GROUP BY builds.id
		`, source.buildID).Scan(&completed, &lastEventID)
		if err != nil {
			source.err = err
			close(source.events)
			return
		}

		if completed && cursor > lastEventID {
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
