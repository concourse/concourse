package db

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
)

var ErrEndOfBuildEventStream = errors.New("end of build event stream")
var ErrBuildEventStreamClosed = errors.New("build event stream closed")

//counterfeiter:generate . EventSource
type EventSource interface {
	Next() (event.Envelope, error)
	Close() error
}

type buildCompleteWatcherFunc func(Tx, int) (bool, error)

func newBuildEventSource(
	buildID int,
	table string,
	conn DbConn,
	from uint,
	watcher buildCompleteWatcherFunc,
) (*buildEventSource, error) {
	wg := new(sync.WaitGroup)

	source := &buildEventSource{
		buildID: buildID,
		table:   table,

		conn: conn,

		events: make(chan event.Envelope, 2000),
		stop:   make(chan struct{}),
		wg:     wg,

		watcherFunc: watcher,
	}

	tx, err := conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	completed, err := watcher(tx, buildID)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	if !completed {
		notifier, err := newConditionNotifier(conn.Bus(), buildEventsChannel(buildID), func() (bool, error) {
			return true, nil
		})
		if err != nil {
			return nil, err
		}

		err = MarkBuildAsBeingWatched(conn, buildEventsChannel(buildID))
		if err != nil {
			notifier.Close()
			return nil, err
		}

		source.notifier = notifier
	}

	wg.Add(1)
	go source.collectEvents(from, completed)

	return source, nil
}

type buildEventSource struct {
	buildID int
	table   string

	conn     DbConn
	notifier Notifier

	events chan event.Envelope
	stop   chan struct{}
	err    error
	wg     *sync.WaitGroup

	watcherFunc buildCompleteWatcherFunc
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
		// If closed already, then do nothing.
		return nil
	default:
		close(source.stop)
	}

	source.wg.Wait()

	if source.notifier != nil {
		return source.notifier.Close()
	}
	return nil
}

func (source *buildEventSource) collectEvents(from uint, completed bool) {
	defer source.wg.Done()

	batchSize := cap(source.events)
	// cursor points to the last emitted event, so subtract 1
	// (the first event is fetched using cursor == -1)
	cursor := int(from) - 1

	for {
		select {
		case <-source.stop:
			source.err = ErrBuildEventStreamClosed
			close(source.events)
			return
		default:
		}

		tx, err := source.conn.Begin()
		if err != nil {
			return
		}

		defer Rollback(tx)

		if !completed {
			completed, err = source.watcherFunc(tx, source.buildID)
			if err != nil {
				source.err = err
				close(source.events)
				return
			}
		}

		eventsQuery := psql.Select("event_id", "type", "version", "payload").
			From(source.table)

		var query sq.SelectBuilder
		if source.buildID > math.MaxInt32 {
			query = eventsQuery.Where(sq.Eq{"build_id": source.buildID})
		} else {
			query = eventsQuery.Where(sq.Or{
				sq.Eq{"build_id": source.buildID},
				sq.Eq{"build_id_old": source.buildID},
			})
		}

		rows, err := query.
			Where(sq.Gt{"event_id": cursor}).
			OrderBy("event_id ASC").
			Limit(uint64(batchSize)).
			RunWith(tx).
			Query()
		if err != nil {
			source.err = err
			close(source.events)
			return
		}

		rowsReturned := 0

		for rows.Next() {
			rowsReturned++

			var t, v, p string
			err := rows.Scan(&cursor, &t, &v, &p)
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
				EventID: strconv.Itoa(cursor),
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

		err = tx.Commit()
		if err != nil {
			close(source.events)
			return
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
