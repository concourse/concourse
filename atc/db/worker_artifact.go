package db

import (
	"database/sql"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/lib/pq"
)

//go:generate counterfeiter . WorkerArtifact

type WorkerArtifact interface {
	ID() int
	Path() string
	Checksum() string
	CreatedAt() time.Time
	Volume(int) (CreatedVolume, bool, error)
}

type artifact struct {
	conn Conn

	id        int
	path      string
	checksum  string
	createdAt time.Time
}

func (a *artifact) ID() int              { return a.id }
func (a *artifact) Path() string         { return a.path }
func (a *artifact) Checksum() string     { return a.checksum }
func (a *artifact) CreatedAt() time.Time { return a.createdAt }

func (a *artifact) Volume(teamID int) (CreatedVolume, bool, error) {
	where := map[string]interface{}{
		"v.team_id":            teamID,
		"v.worker_artifact_id": a.id,
	}

	_, created, err := getVolume(a.conn, where)
	if err != nil {
		return nil, false, err
	}

	if created == nil {
		return nil, false, nil
	}

	return created, true, nil
}

func saveWorkerArtifact(tx Tx, conn Conn, atcArtifact atc.WorkerArtifact) (WorkerArtifact, error) {

	var artifactID int

	err := psql.Insert("worker_artifacts").
		SetMap(map[string]interface{}{
			"path":     atcArtifact.Path,
			"checksum": atcArtifact.Checksum,
		}).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&artifactID)

	if err != nil {
		return nil, err
	}

	artifact, found, err := getWorkerArtifact(tx, conn, artifactID)

	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("Not found")
	}

	return artifact, nil
}

func getWorkerArtifact(tx Tx, conn Conn, id int) (WorkerArtifact, bool, error) {
	var createdAtTime pq.NullTime

	artifact := &artifact{conn: conn}

	err := psql.Select("id", "created_at", "path", "checksum").
		From("worker_artifacts").
		Where(sq.Eq{
			"id": id,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&artifact.id, &createdAtTime, &artifact.path, &artifact.checksum)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	artifact.createdAt = createdAtTime.Time
	return artifact, true, nil
}
