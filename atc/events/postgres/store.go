package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type EventID uint

func (id EventID) Marshal() ([]byte, error) {
	return json.Marshal(id)
}

func (id EventID) GreaterThan(o db.EventKey) bool {
	if o == nil {
		return true
	}
	other, ok := o.(EventID)
	if !ok {
		return false
	}
	return id > other
}

type Store struct {
	Conn db.Conn
	LockFactory lock.LockFactory

	logger lager.Logger
}

func (s *Store) IsConfigured() bool {
	// eventually, it should be possible to configure an external Postgres using flags.
	// for now, the postgres event store will just be a special case - if no other external
	// event store is configured (currently, only Elasticsearch), then default to the Postgres
	// event store using the main postgres instance
	return false
}

func (s *Store) Setup(ctx context.Context) error {
	s.logger = lagerctx.FromContext(ctx)
	if s.logger == nil {
		return fmt.Errorf("missing logger in context")
	}

	l, acquired, err := s.LockFactory.Acquire(
		s.logger.Session("acquire-setup-lock"),
		lock.NewSetupEventStoreLockID(),
	)
	if err != nil {
		return err
	}
	if !acquired {
		// it's already being setup by another ATC
		return nil
	}
	defer l.Release()

	tx, err := s.Conn.Begin()
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

func (s *Store) Close(ctx context.Context) error {
	// for the time being, Store piggybacks off of a connection that's provided to it,
	// so it won't be responsible for its lifecycle
	return nil
}

func (s *Store) Initialize(ctx context.Context, build db.Build) error {
	tx, err := s.Conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	err = s.createBuildEventTable(ctx, tx, build)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventsSeq(build.ID())))

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) createBuildEventTable(ctx context.Context, tx db.Tx, build db.Build) error {
	l, acquired, err := s.LockFactory.Acquire(
		s.logger.Session("acquire-initialize-lock"),
		initializeBuildLockID(build),
	)
	if err != nil {
		return err
	}
	if !acquired {
		// it's already being initialized for another build
		return nil
	}
	defer l.Release()

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

	return nil
}

func (s *Store) Finalize(ctx context.Context, build db.Build) error {
	tx, err := s.Conn.Begin()
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

func (s *Store) Put(ctx context.Context, build db.Build, events []atc.Event) (db.EventKey, error) {
	if len(events) == 0 {
		return nil, nil
	}
	tx, err := s.Conn.Begin()
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

func (s *Store) saveEvents(ctx context.Context, tx db.Tx, build db.Build, events []atc.Event) (db.EventKey, error) {
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

func (s *Store) Get(ctx context.Context, build db.Build, requested int, cursor *db.EventKey) ([]event.Envelope, error) {
	tx, err := s.Conn.Begin()
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

func (s *Store) offset(cursor *db.EventKey) (EventID, error) {
	if cursor == nil || *cursor == nil {
		return 0, nil
	}
	offset, ok := (*cursor).(EventID)
	if !ok {
		return 0, fmt.Errorf("invalid Key type (expected uint, got %T)", *cursor)
	}
	return offset + 1, nil
}

func (s *Store) Delete(ctx context.Context, builds []db.Build) error {
	if len(builds) == 0 {
		return nil
	}

	buildIDs := make([]int, len(builds))
	for i, build := range builds {
		buildIDs[i] = build.ID()
	}

	tx, err := s.Conn.Begin()
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

func (s *Store) DeletePipeline(ctx context.Context, pipeline db.Pipeline) error {
	tx, err := s.Conn.Begin()
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

func (s *Store) DeleteTeam(ctx context.Context, team db.Team) error {
	pipelines, err := team.Pipelines()
	if err != nil {
		return err
	}

	tx, err := s.Conn.Begin()
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

func (s *Store) UnmarshalKey(data []byte, key *db.EventKey) error {
	var i uint
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	*key = EventID(i)
	return nil
}

func dropTableIfExists(ctx context.Context, tx db.Tx, tableName string) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		DROP TABLE IF EXISTS %s
	`, tableName))
	return err
}

func buildEventsTable(build db.Build) string {
	if build.PipelineID() != 0 {
		return pipelineBuildEventsTable(build.PipelineID())
	}
	return teamBuildEventsTable(build.TeamID())
}

func initializeBuildLockID(build db.Build) lock.LockID {
	if build.PipelineID() != 0 {
		return lock.NewInitializePipelineBuildEventsLockID(build.PipelineID())
	}
	return lock.NewInitializeTeamBuildEventsLockID(build.TeamID())
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

func Rollback(tx db.Tx) {
	_ = tx.Rollback()
}

func Close(closer io.Closer) {
	_ = closer.Close()
}