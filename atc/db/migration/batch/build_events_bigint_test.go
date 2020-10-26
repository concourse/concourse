package batch_test

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/db/migration/batch"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BuildEventsBigintSuite struct {
	suite.Suite
	*require.Assertions

	PostgresHost string

	dbName string
	conn   db.Conn

	ctx context.Context
}

func TestBuildEventsBigint(t *testing.T) {
	pgHost := os.Getenv("BATCH_POSTGRES_HOST")
	if pgHost == "" {
		t.Skip("$BATCH_POSTGRES_HOST not configured")
		return
	}

	suite.Run(t, &BuildEventsBigintSuite{
		Assertions: require.New(t),

		PostgresHost: pgHost,
	})
}

func (s *BuildEventsBigintSuite) SetupTest() {
	logger := lager.NewLogger("test")

	if testing.Verbose() {
		logger.RegisterSink(lager.NewPrettySink(os.Stderr, lager.ERROR))
	}

	s.ctx = lagerctx.NewContext(context.Background(), logger)
	s.dbName = s.uniqueName()

	setupConn := s.open(fmt.Sprintf("host=%s dbname=postgres", s.PostgresHost))

	s.dropDb(setupConn)

	_, err := setupConn.ExecContext(s.ctx, `CREATE DATABASE `+s.dbName)
	s.NoError(err)

	s.NoError(setupConn.Close())

	dsn := fmt.Sprintf("host=%s dbname=%s", s.PostgresHost, s.dbName)
	testConn := s.open(dsn)

	err = migration.NewMigrator(testConn, nil).Migrate(nil, nil, 1603405319)
	s.NoError(err)

	s.conn = db.NewConn("test", testConn, dsn, nil, nil)
}

func (s *BuildEventsBigintSuite) TearDownTest() {
	if s.conn != nil {
		s.NoError(s.conn.Close())
	}

	teardownConn := s.open(fmt.Sprintf("host=%s dbname=postgres", s.PostgresHost))

	s.dropDb(teardownConn)

	s.NoError(teardownConn.Close())
}

func (s *BuildEventsBigintSuite) TestBasic() {
	migrator := batch.BuildEventsBigintMigrator{
		DB:        s.conn,
		BatchSize: 100,
	}

	s.seedData(1, 100)

	cleanup, err := migrator.Migrate(s.ctx)
	s.NoError(err)
	s.False(cleanup)

	cleanup, err = migrator.Migrate(s.ctx)
	s.NoError(err)
	s.True(cleanup)

	cleanup, err = migrator.Migrate(s.ctx)
	s.NoError(err)
	s.True(cleanup)

	err = migrator.Cleanup(s.ctx)
	s.NoError(err)

	cleanup, err = migrator.Migrate(s.ctx)
	s.NoError(err)
	s.False(cleanup)

	s.assertFinished()
}

func (s *BuildEventsBigintSuite) TestMigratesInBatches() {
	migrator := batch.BuildEventsBigintMigrator{
		DB:        s.conn,
		BatchSize: 5000,
	}

	totalEvents := s.seedData(100, 500)

	batchesDone := 0

	for {
		done, err := migrator.Migrate(s.ctx)
		s.NoError(err)

		if done {
			break
		}

		batchesDone++
		s.assertRemaining(totalEvents - (migrator.BatchSize * batchesDone))
	}

	err := migrator.Cleanup(s.ctx)
	s.NoError(err)

	s.assertFinished()
}

func (s *BuildEventsBigintSuite) TestNoBuildsSucceeds() {
	migrator := batch.BuildEventsBigintMigrator{
		DB:        s.conn,
		BatchSize: 1,
	}

	migrated, err := migrator.Migrate(s.ctx)
	s.NoError(err)
	s.True(migrated)

	err = migrator.Cleanup(s.ctx)
	s.NoError(err)

	s.assertFinished()
}

func (s *BuildEventsBigintSuite) TestPerformance() {
	if os.Getenv("SLOW") == "" {
		s.T().Skip("$SLOW not set; skipping slow test")
	}

	immediate := make(chan time.Time)
	close(immediate)

	for _, sample := range []struct {
		BatchSize      int
		Builds         int
		EventsPerBuild int
	}{
		{
			BatchSize:      10000,
			Builds:         1000,
			EventsPerBuild: 1000,
		},
		{
			BatchSize:      100000,
			Builds:         1000,
			EventsPerBuild: 1000,
		},
		{
			BatchSize:      500000,
			Builds:         1000,
			EventsPerBuild: 1000,
		},
		{
			BatchSize:      1000000,
			Builds:         1000,
			EventsPerBuild: 1000,
		},
		{
			BatchSize:      100000,
			Builds:         10000,
			EventsPerBuild: 1000,
		},
	} {
		sample := sample

		s.Run(fmt.Sprintf("%d events @ %d batch", sample.Builds*sample.EventsPerBuild, sample.BatchSize), func() {
			migrator := batch.BuildEventsBigintMigrator{
				DB:        s.conn,
				BatchSize: sample.BatchSize,
			}

			s.seedData(sample.Builds, sample.EventsPerBuild)

			start := time.Now()

			last := start
			for {
				done, err := migrator.Migrate(s.ctx)
				s.NoError(err)

				if done {
					break
				}

				debug("batch took", time.Since(last).String())

				last = time.Now()
			}

			debug("total took", time.Since(start).String())

			s.assertRemaining(0)
		})
	}
}

func (s *BuildEventsBigintSuite) seedData(builds, events int) int {
	_, err := s.conn.ExecContext(s.ctx, `TRUNCATE TABLE build_events`)
	s.NoError(err)

	start := time.Now()
	res, err := s.conn.ExecContext(s.ctx, `
		INSERT INTO build_events (build_id_old, type, payload, event_id, version)
		SELECT build.id, 'log', '{"origin":{"id":"some-plan-id","source":"stderr"},"payload":"hello","time":123}', event.id, '1.0'
		FROM generate_series(1, $1) AS build (id), generate_series(1, $2) AS event (id)
	`, builds, events)
	s.NoError(err)

	rows, err := res.RowsAffected()
	s.NoError(err)

	debug("seeding", int(rows), "rows took", time.Since(start).String())

	return int(rows)
}

func (s *BuildEventsBigintSuite) assertRemaining(n int) {
	var remaining int
	s.NoError(s.conn.QueryRowContext(s.ctx, `SELECT COUNT(1) FROM build_events WHERE build_id IS NULL`).Scan(&remaining))
	s.Equal(n, remaining)
}

func (s *BuildEventsBigintSuite) assertFinished() {
	s.assertRemaining(0)

	_, err := s.conn.ExecContext(s.ctx, `SELECT build_id_old FROM build_events LIMIT 1`)
	s.Error(err)

	var pqErr *pq.Error
	s.True(errors.As(err, &pqErr))
	s.Equal("undefined_column", pqErr.Code.Name())
}

func (s *BuildEventsBigintSuite) open(dsn string) *sql.DB {
	conn, err := sql.Open("postgres", dsn)
	s.NoError(err)

	return conn
}

func (s *BuildEventsBigintSuite) uniqueName() string {
	hash := sha1.New()

	_, err := fmt.Fprint(hash, s.T().Name())
	s.NoError(err)

	return fmt.Sprintf("testdb_%x", hash.Sum(nil))
}

func (s *BuildEventsBigintSuite) dropDb(conn *sql.DB) {
	_, err := conn.ExecContext(s.ctx, `
		SELECT pid, pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()
	`, s.dbName)
	s.NoError(err)

	_, err = conn.ExecContext(s.ctx, `DROP DATABASE IF EXISTS `+s.dbName)
	s.NoError(err)
}

var logger = log.New(os.Stderr, "", log.Ltime|log.Lmicroseconds)

func debug(args ...interface{}) {
	if testing.Verbose() {
		logger.Println(args...)
	}
}
