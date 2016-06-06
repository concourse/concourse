package db

import "fmt"

//go:generate counterfeiter . BuildDBFactory

type BuildDBFactory interface {
	GetBuildDB(build Build) BuildDB
}

func NewBuildDBFactory(conn Conn, bus *notificationsBus) BuildDBFactory {
	return &buildDBFactory{
		conn: conn,
		bus:  bus,
	}
}

type buildDBFactory struct {
	conn Conn
	bus  *notificationsBus
}

func (f *buildDBFactory) GetBuildDB(build Build) BuildDB {
	return &buildDB{
		buildID:    build.ID,
		pipelineID: build.PipelineID,
		conn:       f.conn,
		bus:        f.bus,
	}
}

//go:generate counterfeiter . BuildDB

type BuildDB interface {
	Get() (Build, bool, error)
	GetID() int

	Events(from uint) (EventSource, error)

	AbortNotifier() (Notifier, error)
}

type buildDB struct {
	buildID    int
	pipelineID int
	conn       Conn
	bus        *notificationsBus
}

func (db *buildDB) Get() (Build, bool, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE b.id = $1
	`, db.buildID))
}

func (db *buildDB) GetID() int {
	return db.buildID
}

func (db *buildDB) Events(from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(db.bus, buildEventsChannel(db.buildID), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	table := "build_events"
	if db.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", db.pipelineID)
	}

	return newSQLDBBuildEventSource(
		db.buildID,
		table,
		db.conn,
		notifier,
		from,
	), nil
}

func (db *buildDB) AbortNotifier() (Notifier, error) {
	return newConditionNotifier(db.bus, buildAbortChannel(db.buildID), func() (bool, error) {
		var aborted bool
		err := db.conn.QueryRow(`
			SELECT status = 'aborted'
			FROM builds
			WHERE id = $1
		`, db.buildID).Scan(&aborted)

		return aborted, err
	})
}

func newConditionNotifier(bus *notificationsBus, channel string, cond func() (bool, error)) (Notifier, error) {
	notified, err := bus.Listen(channel)
	if err != nil {
		return nil, err
	}

	notifier := &conditionNotifier{
		cond:    cond,
		bus:     bus,
		channel: channel,

		notified: notified,
		notify:   make(chan struct{}, 1),

		stop: make(chan struct{}),
	}

	go notifier.watch()

	return notifier, nil
}
