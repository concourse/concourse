package db

import (
	"context"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
)

//go:generate counterfeiter . EventStore

type EventStore interface {
	Setup(ctx context.Context) error
	// TODO: add Close(ctx context.Context) error

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

type EventID uint

func (id EventID) Marshal() ([]byte, error) {
	return json.Marshal(id)
}

func (id EventID) GreaterThan(o EventKey) bool {
	if o == nil {
		return true
	}
	other, ok := o.(EventID)
	if !ok {
		return false
	}
	return id > other
}

type buildEventStore struct {
	conn Conn
}

func NewBuildEventStore(conn Conn) EventStore {
	return &buildEventStore{
		conn: conn,
	}
}

func (s *buildEventStore) Setup(ctx context.Context) error {
	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS build_events (
			  build_id integer,
			  type character varying(32) NOT NULL,
			  payload text NOT NULL,
			  event_id integer NOT NULL,
			  version text NOT NULL
		);

		CREATE UNIQUE INDEX IF NOT EXISTS build_events_build_id_event_id ON build_events USING btree (build_id, event_id);

		CREATE INDEX IF NOT EXISTS build_events_build_id_idx ON build_events USING btree (build_id);
	`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *buildEventStore) Initialize(ctx context.Context, build Build) error {
	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	tableName := buildEventsTable(build)

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s () INHERITS (build_events)
	`, tableName))
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_build_id ON %s (build_id);
	`, tableName, tableName))
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE UNIQUE INDEX IF NOT EXISTS %s_build_id_event_id ON %s (build_id, event_id)
	`, tableName, tableName))
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventsSeq(build.ID())))

	return tx.Commit()
}

func (s *buildEventStore) Finalize(ctx context.Context, build Build) error {
	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DROP SEQUENCE %s
	`, buildEventsSeq(build.ID())))
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *buildEventStore) Put(ctx context.Context, build Build, events []atc.Event) (EventKey, error) {
	if len(events) == 0 {
		return nil, nil
	}
	tx, err := s.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	lastKey, err := s.saveEvents(ctx, tx, build, events)
	if err != nil {
		return nil, err
	}

	return lastKey, tx.Commit()
}

func (s *buildEventStore) saveEvents(ctx context.Context, tx Tx, build Build, events []atc.Event) (EventKey, error) {
	query := psql.Insert(buildEventsTable(build)).
		Columns("event_id", "build_id", "type", "version", "payload")
	for _, evt := range events {
		payload, err := json.Marshal(evt)
		if err != nil {
			return nil, err
		}
		query = query.Values(
			sq.Expr("nextval('" + buildEventsSeq(build.ID()) + "')"),
			build.ID(),
			string(evt.EventType()),
			string(evt.Version()),
			payload,
		)
	}
	rows, err := query.Suffix("RETURNING event_id").RunWith(tx).QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer Close(rows)
	var k EventID
	for rows.Next() {
		err = rows.Scan(&k)
		if err != nil {
			return nil, err
		}
	}
	return k + 1, err
}

func (s *buildEventStore) Get(ctx context.Context, build Build, requested int, cursor *EventKey) ([]event.Envelope, error) {
	tx, err := s.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	offset, err := s.offset(cursor)
	if err != nil {
		return nil, err
	}

	rows, err := psql.Select("type", "version", "payload").
		From(buildEventsTable(build)).
		Where(sq.Eq{"build_id": build.ID()}).
		OrderBy("event_id ASC").
		Offset(uint64(offset)).
		Limit(uint64(requested)).
		RunWith(tx).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	var events []event.Envelope
	for rows.Next() {
		var t, v, p string
		err := rows.Scan(&t, &v, &p)
		if err != nil {
			return nil, err
		}

		data := json.RawMessage(p)

		events = append(events, event.Envelope{
			Data:    &data,
			Event:   atc.EventType(t),
			Version: atc.EventVersion(v),
		})

		*cursor = offset
		offset++
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *buildEventStore) offset(cursor *EventKey) (EventID, error) {
	if cursor == nil || *cursor == nil {
		return 0, nil
	}
	offset, ok := (*cursor).(EventID)
	if !ok {
		return 0, fmt.Errorf("invalid Key type (expected uint, got %T)", *cursor)
	}
	return offset + 1, nil
}

func (s *buildEventStore) Delete(ctx context.Context, builds []Build) error {
	if len(builds) == 0 {
		return nil
	}

	buildIDs := make([]int, len(builds))
	for i, build := range builds {
		buildIDs[i] = build.ID()
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	_, err = psql.Delete("build_events").
		Where(sq.Eq{"build_id": buildIDs}).
		RunWith(tx).
		ExecContext(ctx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *buildEventStore) DeletePipeline(ctx context.Context, pipeline Pipeline) error {
	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	err = dropTableIfExists(ctx, tx, pipelineBuildEventsTable(pipeline.ID()))
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *buildEventStore) DeleteTeam(ctx context.Context, team Team) error {
	pipelines, err := team.Pipelines()
	if err != nil {
		return err
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	err = dropTableIfExists(ctx, tx, teamBuildEventsTable(team.ID()))
	if err != nil {
		return err
	}

	for _, pipeline := range pipelines {
		err = dropTableIfExists(ctx, tx, pipelineBuildEventsTable(pipeline.ID()))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *buildEventStore) UnmarshalKey(data []byte, key *EventKey) error {
	var i uint
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	*key = EventID(i)
	return nil
}

func dropTableIfExists(ctx context.Context, tx Tx, tableName string) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		DROP TABLE IF EXISTS %s
	`, tableName))
	return err
}

func buildEventsTable(build Build) string {
	if build.PipelineID() != 0 {
		return pipelineBuildEventsTable(build.PipelineID())
	}
	return teamBuildEventsTable(build.TeamID())
}

func pipelineBuildEventsTable(pipelineID int) string {
	return fmt.Sprintf("pipeline_build_events_%d", pipelineID)
}

func teamBuildEventsTable(teamID int) string {
	return fmt.Sprintf("team_build_events_%d", teamID)
}

func buildEventsSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
