package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	_ "github.com/lib/pq"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type sqldb struct {
	conn *sql.DB
}

func NewSQL(sqldbConnection *sql.DB) DB {
	return &sqldb{sqldbConnection}
}

func (db *sqldb) RegisterJob(name string) error {
	_, err := db.conn.Exec(`
		INSERT INTO jobs (name)
		SELECT $1
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs WHERE name = $1
		)
	`, name)
	return err
}

func (db *sqldb) RegisterResource(name string) error {
	_, err := db.conn.Exec(`
		INSERT INTO resources (name)
		SELECT $1
		WHERE NOT EXISTS (
			SELECT 1 FROM resources WHERE name = $1
		)
	`, name)
	return err
}

func (db *sqldb) Builds(job string) ([]builds.Build, error) {
	rows, err := db.conn.Query(`
		SELECT name, status, abort_url
		FROM builds
		WHERE job_name = $1
		ORDER BY id ASC
	`, job)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []builds.Build{}

	for rows.Next() {
		var name int
		var status string
		var abortURL sql.NullString
		err := rows.Scan(&name, &status, &abortURL)
		if err != nil {
			return nil, err
		}

		bs = append(bs, builds.Build{
			ID:       name,
			Status:   builds.Status(status),
			AbortURL: abortURL.String,
		})
	}

	return bs, nil
}

func (db *sqldb) GetBuild(job string, name int) (builds.Build, error) {
	var id int
	var status string
	var abortURL sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, status, abort_url
		FROM builds
		WHERE job_name = $1
		AND name = $2
	`, job, name).Scan(&id, &status, &abortURL)
	if err != nil {
		return builds.Build{}, err
	}

	inputs := []builds.VersionedResource{}

	rows, err := db.conn.Query(`
		SELECT v.resource_name, v.source, v.version, v.metadata
		FROM versioned_resources v, build_inputs i
		WHERE i.build_id = $1
		AND i.versioned_resource_id = v.id
	`, id)
	if err != nil {
		return builds.Build{}, err
	}

	defer rows.Close()

	for rows.Next() {
		var input builds.VersionedResource

		var source, version, metadata string
		err := rows.Scan(&input.Name, &source, &version, &metadata)
		if err != nil {
			return builds.Build{}, err
		}

		err = json.Unmarshal([]byte(source), &input.Source)
		if err != nil {
			return builds.Build{}, err
		}

		err = json.Unmarshal([]byte(version), &input.Version)
		if err != nil {
			return builds.Build{}, err
		}

		err = json.Unmarshal([]byte(metadata), &input.Metadata)
		if err != nil {
			return builds.Build{}, err
		}

		inputs = append(inputs, input)
	}

	return builds.Build{
		ID:       name,
		Status:   builds.Status(status),
		AbortURL: abortURL.String,
		Inputs:   inputs,
	}, nil
}

func (db *sqldb) GetCurrentBuild(job string) (builds.Build, error) {
	var name int
	var status string
	var abortURL sql.NullString

	rows, err := db.conn.Query(`
		SELECT name, status, abort_url
		FROM builds
		WHERE job_name = $1
		AND status != 'pending'
		ORDER BY id DESC
		LIMIT 1
	`, job)
	if err != nil {
		return builds.Build{}, err
	}

	defer rows.Close()

	if !rows.Next() {
		rows, err = db.conn.Query(`
			SELECT name, status, abort_url
			FROM builds
			WHERE job_name = $1
			AND status = 'pending'
			ORDER BY id ASC
			LIMIT 1
		`, job)
		if err != nil {
			return builds.Build{}, err
		}

		defer rows.Close()

		rows.Next()
	}

	err = rows.Scan(&name, &status, &abortURL)
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:       name,
		Status:   builds.Status(status),
		AbortURL: abortURL.String,
	}, nil
}

func (db *sqldb) AttemptBuild(job string, resourceName string, version builds.Version, serial bool) (builds.Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return builds.Build{}, err
	}

	defer tx.Rollback()

	var totalStarted int
	err = tx.QueryRow(`
		SELECT COUNT(id)
		FROM builds b
		WHERE status IN ('started', 'pending')
		AND job_name = $1
	`, job).Scan(&totalStarted)
	if err != nil {
		return builds.Build{}, err
	}

	if totalStarted > 0 {
		rows, err := tx.Query(`
			SELECT v.version
			FROM versioned_resources v, builds b, build_inputs i
			WHERE b.status IN ('started', 'pending')
			AND b.job_name = $1
			AND i.build_id = b.id
			AND i.versioned_resource_id = v.id
			AND v.resource_name = $2
		`, job, resourceName)
		if err != nil {
			return builds.Build{}, err
		}

		defer rows.Close()

		versionsChecked := 0
		for rows.Next() {
			var versionJSON string
			err := rows.Scan(&versionJSON)
			if err != nil {
				return builds.Build{}, err
			}

			var inputVersion builds.Version
			err = json.Unmarshal([]byte(versionJSON), &inputVersion)
			if err != nil {
				return builds.Build{}, err
			}

			if reflect.DeepEqual(inputVersion, version) {
				return builds.Build{}, ErrInputRedundant
			}

			versionsChecked++
		}

		if versionsChecked < totalStarted {
			return builds.Build{}, ErrInputNotDetermined
		}

		rows, err = tx.Query(`
			SELECT version
			FROM versioned_resources v, builds b, build_outputs o
			WHERE b.status IN ('started', 'pending')
			AND b.job_name = $1
			AND o.build_id = b.id
			AND o.versioned_resource_id = v.id
			AND v.resource_name = $2
		`, job, resourceName)
		if err != nil {
			return builds.Build{}, err
		}

		defer rows.Close()

		versionsChecked = 0
		for rows.Next() {
			var versionJSON string
			err := rows.Scan(&versionJSON)
			if err != nil {
				return builds.Build{}, err
			}

			var outputVersion builds.Version
			err = json.Unmarshal([]byte(versionJSON), &outputVersion)
			if err != nil {
				return builds.Build{}, err
			}

			if reflect.DeepEqual(outputVersion, version) {
				return builds.Build{}, ErrOutputRedundant
			}

			versionsChecked++
		}

		if serial && versionsChecked < totalStarted {
			return builds.Build{}, ErrOutputNotDetermined
		}
	}

	var name int
	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1
		RETURNING build_number_seq
	`, job).Scan(&name)
	if err != nil {
		return builds.Build{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO builds(name, job_name, status)
		VALUES ($1, $2, 'pending')
	`, name, job)
	if err != nil {
		return builds.Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:     name,
		Status: builds.StatusPending,
	}, nil
}

func (db *sqldb) CreateBuild(job string) (builds.Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return builds.Build{}, err
	}

	defer tx.Rollback()

	var name int
	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1
		RETURNING build_number_seq
	`, job).Scan(&name)
	if err != nil {
		return builds.Build{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO builds(name, job_name, status)
		VALUES ($1, $2, 'pending')
	`, name, job)
	if err != nil {
		return builds.Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:     name,
		Status: builds.StatusPending,
	}, nil
}

func (db *sqldb) ScheduleBuild(job string, id int, serial bool) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET scheduled = true

		-- only the given build
		WHERE job_name = $1
		AND name = $2
		AND status = 'pending'

		-- if serial, only if it's the nextmost pending
		AND (
			NOT $3 OR id IN (
				SELECT id
				FROM builds
				WHERE job_name = $1
				AND status = 'pending'
				ORDER BY id ASC
				LIMIT 1
			)
		)

		-- if serial, not if another build is started or scheduled
		AND NOT (
			$3 AND EXISTS (
				SELECT 1
				FROM builds
				WHERE job_name = $1
				AND (status = 'started' OR (status = 'pending' AND scheduled = true))
			)
		)
	`, job, id, serial)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *sqldb) StartBuild(job string, id int, abortURL string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET status = 'started', abort_url = $3
		WHERE job_name = $1
		AND name = $2
		AND status = 'pending'
	`, job, id, abortURL)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *sqldb) AbortBuild(job string, id int) error {
	return db.SaveBuildStatus(job, id, builds.StatusAborted)
}

func (db *sqldb) SaveBuildInput(job string, build int, vr builds.VersionedResource) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	vrID, err := db.saveVersionedResource(tx, vr)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO build_inputs (build_id, versioned_resource_id)
		SELECT id, $3
		FROM builds
		WHERE job_name = $1
		AND name = $2
	`, job, build, vrID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *sqldb) SaveBuildOutput(job string, build int, vr builds.VersionedResource) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	vrID, err := db.saveVersionedResource(tx, vr)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO build_outputs (build_id, versioned_resource_id)
		SELECT id, $3
		FROM builds
		WHERE job_name = $1
		AND name = $2
	`, job, build, vrID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *sqldb) SaveBuildStatus(job string, build int, status builds.Status) error {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET status = $3
		WHERE job_name = $1
		AND name = $2
	`, job, build, string(status))
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows != 1 {
		return fmt.Errorf("more than one row affected: %d", rows)
	}

	return nil
}

func (db *sqldb) BuildLog(job string, build int) ([]byte, error) {
	var log string

	err := db.conn.QueryRow(`
		SELECT log
		FROM builds
		WHERE job_name = $1
		AND name = $2
	`, job, build).Scan(&log)
	if err != nil {
		return nil, err
	}

	return []byte(log), nil
}

func (db *sqldb) AppendBuildLog(job string, build int, log []byte) error {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET log = log || $3
		WHERE job_name = $1
		AND name = $2
	`, job, build, string(log))
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows != 1 {
		return fmt.Errorf("more than one row affected: %d", rows)
	}

	return nil
}

func (db *sqldb) GetCurrentVersion(job, resource string) (builds.Version, error) {
	var versionString string

	err := db.conn.QueryRow(`
		SELECT version
		FROM transitional_current_versions
		WHERE job_name = $1
		AND resource_name = $2
		ORDER BY id DESC
		LIMIT 1
	`, job, resource).Scan(&versionString)
	if err != nil {
		return nil, err
	}

	var version builds.Version

	err = json.Unmarshal([]byte(versionString), &version)
	return version, err
}

func (db *sqldb) SaveCurrentVersion(job, resource string, version builds.Version) error {
	versionBytes, err := json.Marshal(version)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO transitional_current_versions(job_name, resource_name, version)
		VALUES ($1, $2, $3)
	`, job, resource, string(versionBytes))
	if err != nil {
		return err
	}

	return err
}

func (db *sqldb) SaveVersionedResource(vr builds.VersionedResource) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = db.saveVersionedResource(tx, vr)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *sqldb) GetLatestVersionedResource(name string) (builds.VersionedResource, error) {
	var sourceBytes, versionBytes, metadataBytes string

	err := db.conn.QueryRow(`
		SELECT source, version, metadata
		FROM versioned_resources
		WHERE resource_name = $1
		ORDER BY id DESC
		LIMIT 1
	`, name).Scan(&sourceBytes, &versionBytes, &metadataBytes)
	if err != nil {
		return builds.VersionedResource{}, err
	}

	vr := builds.VersionedResource{
		Name: name,
	}

	err = json.Unmarshal([]byte(sourceBytes), &vr.Source)
	if err != nil {
		return builds.VersionedResource{}, err
	}

	err = json.Unmarshal([]byte(versionBytes), &vr.Version)
	if err != nil {
		return builds.VersionedResource{}, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &vr.Metadata)
	if err != nil {
		return builds.VersionedResource{}, err
	}

	return vr, nil
}

func (db *sqldb) GetCommonOutputs(jobs []string, resourceName string) ([]builds.Version, error) {
	fromAliases := make([]string, len(jobs))
	conditions := []string{}

	params := []interface{}{resourceName}
	for i, j := range jobs {
		params = append(params, j)

		fromAliases[i] = fmt.Sprintf("builds b%d, build_outputs o%d, versioned_resources v%d", i+1, i+1, i+1)
		conditions = append(conditions, fmt.Sprintf("o%d.build_id = b%d.id", i+1, i+1))
		conditions = append(conditions, fmt.Sprintf("o%d.versioned_resource_id = v%d.id", i+1, i+1))
		conditions = append(conditions, fmt.Sprintf("v%d.resource_name = $1", i+1))
		conditions = append(conditions, fmt.Sprintf("b%d.job_name = $%d", i+1, i+2))
		conditions = append(conditions, fmt.Sprintf("v1.version = v%d.version", i+1))
	}

	rows, err := db.conn.Query(fmt.Sprintf(
		`
			SELECT v1.version
			FROM %s
			WHERE %s
		`,
		strings.Join(fromAliases, ", "),
		strings.Join(conditions, " AND "),
	), params...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	vs := []builds.Version{}

	for rows.Next() {
		var versionString string
		err := rows.Scan(&versionString)
		if err != nil {
			return nil, err
		}

		var version builds.Version

		err = json.Unmarshal([]byte(versionString), &version)
		if err != nil {
			return nil, err
		}

		vs = append(vs, version)
	}

	return vs, nil
}

func (db *sqldb) GetLatestInputVersions(inputs []config.Input) ([]builds.VersionedResource, error) {
	idColumns := make([]string, len(inputs))
	orderBy := make([]string, len(inputs))
	fromAliases := []string{}
	conditions := []string{}
	params := []interface{}{}

	passedJobs := map[string]int{}

	for _, j := range inputs {
		params = append(params, j.Resource)
	}

	for i, j := range inputs {
		idColumns[i] = fmt.Sprintf("v%d.id", i+1)
		orderBy[i] = fmt.Sprintf("v%d.id DESC", i+1)

		fromAliases = append(fromAliases, fmt.Sprintf("versioned_resources v%d", i+1))

		conditions = append(conditions, fmt.Sprintf("v%d.resource_name = $%d", i+1, i+1))

		for _, name := range j.Passed {
			idx, found := passedJobs[name]
			if !found {
				idx = len(passedJobs)
				passedJobs[name] = idx

				fromAliases = append(fromAliases, fmt.Sprintf("builds b%d", idx+1))

				conditions = append(conditions, fmt.Sprintf("b%d.job_name = $%d", idx+1, idx+len(inputs)+1))

				// add job name to params
				params = append(params, name)
			}

			fromAliases = append(fromAliases, fmt.Sprintf("build_outputs v%db%d", i+1, idx+1))

			conditions = append(conditions, fmt.Sprintf("v%db%d.versioned_resource_id = v%d.id", i+1, idx+1, i+1))

			conditions = append(conditions, fmt.Sprintf("v%db%d.build_id = b%d.id", i+1, idx+1, idx+1))
		}
	}

	ids := []interface{}{}
	for _ = range inputs {
		var id int
		ids = append(ids, &id)
	}

	query := fmt.Sprintf(
		`
			SELECT DISTINCT %s
			FROM %s
			WHERE %s
			ORDER BY %s
			LIMIT 1
		`,
		strings.Join(idColumns, ", "),
		strings.Join(fromAliases, ", "),
		strings.Join(conditions, "\nAND "),
		strings.Join(orderBy, ", "),
	)

	err := db.conn.QueryRow(query, params...).Scan(ids...)
	if err != nil {
		return nil, err
	}

	vrs := []builds.VersionedResource{}

	for _, idPtr := range ids {
		id := *(idPtr.(*int))

		var vr builds.VersionedResource

		var source, version, metadata string

		err := db.conn.QueryRow(`
			SELECT resource_name, source, version, metadata
			FROM versioned_resources
			WHERE id = $1
		`, id).Scan(&vr.Name, &source, &version, &metadata)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(source), &vr.Source)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, err
		}

		vrs = append(vrs, vr)
	}

	return vrs, nil
}

func (db *sqldb) saveVersionedResource(tx *sql.Tx, vr builds.VersionedResource) (int, error) {
	versionJSON, err := json.Marshal(vr.Version)
	if err != nil {
		return 0, err
	}

	sourceJSON, err := json.Marshal(vr.Source)
	if err != nil {
		return 0, err
	}

	metadataJSON, err := json.Marshal(vr.Metadata)
	if err != nil {
		return 0, err
	}

	var id int

	_, err = tx.Exec(`
		INSERT INTO versioned_resources (resource_name, version, source, metadata)
		SELECT $1, $2, $3, $4
		WHERE NOT EXISTS (
			SELECT 1
			FROM versioned_resources
			WHERE resource_name = $1
			AND version = $2
		)
	`, vr.Name, string(versionJSON), string(sourceJSON), string(metadataJSON))
	if err != nil {
		return 0, err
	}

	err = tx.QueryRow(`
		SELECT id FROM versioned_resources
		WHERE resource_name = $1
		AND version = $2
	`, vr.Name, string(versionJSON)).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}
