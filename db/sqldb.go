package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type sqldb struct {
	logger lager.Logger
	conn   *sql.DB
}

func NewSQL(logger lager.Logger, sqldbConnection *sql.DB) DB {
	return &sqldb{
		logger: logger,
		conn:   sqldbConnection,
	}
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

func (db *sqldb) GetAllJobBuilds(job string) ([]builds.Build, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, status, abort_url
		FROM builds
		WHERE job_name = $1
		ORDER BY id DESC
	`, job)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []builds.Build{}

	for rows.Next() {
		var id int
		var name string
		var status string
		var abortURL sql.NullString
		err := rows.Scan(&id, &name, &status, &abortURL)
		if err != nil {
			return nil, err
		}

		bs = append(bs, builds.Build{
			ID:       id,
			Name:     name,
			Status:   builds.Status(status),
			JobName:  job,
			AbortURL: abortURL.String,
		})
	}

	return bs, nil
}

func (db *sqldb) GetBuild(buildID int) (builds.Build, error) {
	var name string
	var jobName string
	var status string
	var abortURL sql.NullString

	err := db.conn.QueryRow(`
		SELECT name, job_name, status, abort_url
		FROM builds
		WHERE id = $1
	`, buildID).Scan(&name, &jobName, &status, &abortURL)
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:       buildID,
		Name:     name,
		JobName:  jobName,
		Status:   builds.Status(status),
		AbortURL: abortURL.String,
	}, nil
}

func (db *sqldb) GetJobBuild(job string, name string) (builds.Build, error) {
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

	return builds.Build{
		ID:       id,
		Name:     name,
		JobName:  job,
		Status:   builds.Status(status),
		AbortURL: abortURL.String,
	}, nil
}

func (db *sqldb) GetBuildResources(job string, name string) ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	rows, err := db.conn.Query(`
		SELECT v.resource_name, v.type, v.source, v.version, v.metadata,
		NOT EXISTS (
			SELECT 1
			FROM build_inputs, builds
			WHERE versioned_resource_id = v.id
			AND job_name = $1
			AND build_id = id
			AND build_id < b.id
		)
		FROM versioned_resources v, build_inputs i, builds b
		WHERE b.job_name = $1
		AND b.name = $2
		AND i.build_id = b.id
		AND i.versioned_resource_id = v.id
	`, job, name)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var vr builds.VersionedResource
		var firstOccurrence bool

		var source, version, metadata string
		err := rows.Scan(&vr.Name, &vr.Type, &source, &version, &metadata, &firstOccurrence)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(source), &vr.Source)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, nil, err
		}

		inputs = append(inputs, BuildInput{
			VersionedResource: vr,
			FirstOccurrence:   firstOccurrence,
		})
	}

	rows, err = db.conn.Query(`
		SELECT v.resource_name, v.type, v.source, v.version, v.metadata
		FROM versioned_resources v, build_outputs o, builds b
		WHERE b.job_name = $1
		AND b.name = $2
		AND o.build_id = b.id
		AND o.versioned_resource_id = v.id
		AND NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE versioned_resource_id = v.id
			AND build_id = b.id
		)
	`, job, name)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var vr builds.VersionedResource

		var source, version, metadata string
		err := rows.Scan(&vr.Name, &vr.Type, &source, &version, &metadata)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(source), &vr.Source)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, nil, err
		}

		outputs = append(outputs, BuildOutput{
			VersionedResource: vr,
		})
	}

	return inputs, outputs, nil
}

func (db *sqldb) GetCurrentBuild(job string) (builds.Build, error) {
	var id int
	var name string
	var status string
	var abortURL sql.NullString

	rows, err := db.conn.Query(`
		SELECT id, name, status, abort_url
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
			SELECT id, name, status, abort_url
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

	err = rows.Scan(&id, &name, &status, &abortURL)
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:       id,
		Name:     name,
		JobName:  job,
		Status:   builds.Status(status),
		AbortURL: abortURL.String,
	}, nil
}

func (db *sqldb) CreateBuild(job string) (builds.Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return builds.Build{}, err
	}

	defer tx.Rollback()

	var name string
	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1
		RETURNING build_number_seq
	`, job).Scan(&name)
	if err != nil {
		return builds.Build{}, err
	}

	var id int
	err = tx.QueryRow(`
		INSERT INTO builds(name, job_name, status)
		VALUES ($1, $2, 'pending')
		RETURNING id
	`, name, job).Scan(&id)
	if err != nil {
		return builds.Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:      id,
		Name:    name,
		JobName: job,
		Status:  builds.StatusPending,
	}, nil
}

func (db *sqldb) ScheduleBuild(job string, name string, serial bool) (bool, error) {
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
	`, job, name, serial)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *sqldb) StartBuild(job string, name string, abortURL string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET status = 'started', abort_url = $3
		WHERE job_name = $1
		AND name = $2
		AND status = 'pending'
	`, job, name, abortURL)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *sqldb) AbortBuild(buildID int) (string, error) {
	var abortURL sql.NullString
	err := db.conn.QueryRow(`
		UPDATE builds
		SET status = $2
		WHERE id = $1
		RETURNING abort_url
	`, buildID, string(builds.StatusAborted)).Scan(&abortURL)
	if err != nil {
		return "", err
	}

	return abortURL.String, nil
}

func (db *sqldb) SaveBuildInput(buildID int, vr builds.VersionedResource) error {
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
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE build_id = $1
			AND versioned_resource_id = $2
		)
	`, buildID, vrID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *sqldb) SaveBuildOutput(buildID int, vr builds.VersionedResource) error {
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
		VALUES ($1, $2)
	`, buildID, vrID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *sqldb) SaveBuildStatus(buildID int, status builds.Status) error {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET status = $2
		WHERE id = $1
	`, buildID, string(status))
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

func (db *sqldb) BuildLog(buildID int) ([]byte, error) {
	var log string

	err := db.conn.QueryRow(`
		SELECT log
		FROM builds
		WHERE id = $1
	`, buildID).Scan(&log)
	if err != nil {
		return nil, err
	}

	return []byte(log), nil
}

func (db *sqldb) AppendBuildLog(buildID int, log []byte) error {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET log = log || $2
		WHERE id = $1
	`, buildID, string(log))
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

	vr := builds.VersionedResource{
		Name: name,
	}

	err := db.conn.QueryRow(`
		SELECT type, source, version, metadata
		FROM versioned_resources
		WHERE resource_name = $1
		ORDER BY id DESC
		LIMIT 1
	`, name).Scan(&vr.Type, &sourceBytes, &versionBytes, &metadataBytes)
	if err != nil {
		return builds.VersionedResource{}, err
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

func (db *sqldb) GetLatestInputVersions(inputs []config.Input) (builds.VersionedResources, error) {
	startedAt := time.Now()

	getLog := db.logger.Session("get-latest-input-versions", lager.Data{
		"inputs": inputs,
	})

	getLog.Debug("start")

	defer func() {
		getLog.Info("finished", lager.Data{
			"took": time.Now().Sub(startedAt).String(),
		})
	}()

	fromAliases := []string{}
	conditions := []string{}
	params := []interface{}{}

	passedJobs := map[string]int{}

	for _, j := range inputs {
		params = append(params, j.Resource)
	}

	for i, j := range inputs {
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

	vrs := []builds.VersionedResource{}

	for i, _ := range inputs {
		var vr builds.VersionedResource

		var id int
		var source, version, metadata string

		err := db.conn.QueryRow(fmt.Sprintf(
			`
				SELECT v%[1]d.id, v%[1]d.resource_name, v%[1]d.type, v%[1]d.source, v%[1]d.version, v%[1]d.metadata
				FROM %s
				WHERE %s
				ORDER BY v%[1]d.id DESC
				LIMIT 1
			`,
			i+1,
			strings.Join(fromAliases, ", "),
			strings.Join(conditions, "\nAND "),
		), params...).Scan(&id, &vr.Name, &vr.Type, &source, &version, &metadata)

		params = append(params, id)
		conditions = append(conditions, fmt.Sprintf("v%d.id = $%d", i+1, len(params)))

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

func (db *sqldb) GetJobBuildForInputs(job string, inputs builds.VersionedResources) (builds.Build, error) {
	from := []string{"builds b"}
	conditions := []string{"b.job_name = $1"}
	params := []interface{}{job}

	for i, vr := range inputs {
		versionBytes, err := json.Marshal(vr.Version)
		if err != nil {
			return builds.Build{}, err
		}

		var id int

		err = db.conn.QueryRow(`
			SELECT id
			FROM versioned_resources
			WHERE resource_name = $1
			AND type = $2
			AND version = $3
		`, vr.Name, vr.Type, string(versionBytes)).Scan(&id)
		if err != nil {
			return builds.Build{}, err
		}

		from = append(from, fmt.Sprintf("build_inputs i%d", i+1))
		params = append(params, id)

		conditions = append(conditions,
			fmt.Sprintf("i%d.build_id = b.id", i+1),
			fmt.Sprintf("i%d.versioned_resource_id = $%d", i+1, len(params)),
		)
	}

	var buildID int
	var buildName string
	err := db.conn.QueryRow(fmt.Sprintf(`
		SELECT b.id, b.name
		FROM %s
		WHERE %s
		`,
		strings.Join(from, ", "),
		strings.Join(conditions, "\nAND ")), params...).Scan(&buildID, &buildName)
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:      buildID,
		Name:    buildName,
		JobName: job,
		Status:  builds.StatusPending,
	}, nil
}

func (db *sqldb) CreateBuildWithInputs(job string, inputs builds.VersionedResources) (builds.Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return builds.Build{}, err
	}

	defer tx.Rollback()

	var name string
	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1
		RETURNING build_number_seq
	`, job).Scan(&name)
	if err != nil {
		return builds.Build{}, err
	}

	var buildID int
	err = tx.QueryRow(`
		INSERT INTO builds(name, job_name, status)
		VALUES ($1, $2, 'pending')
		RETURNING id
	`, name, job).Scan(&buildID)
	if err != nil {
		return builds.Build{}, err
	}

	for _, vr := range inputs {
		vrID, err := db.saveVersionedResource(tx, vr)
		if err != nil {
			return builds.Build{}, err
		}

		_, err = tx.Exec(`
			INSERT INTO build_inputs (build_id, versioned_resource_id)
			VALUES ($1, $2)
		`, buildID, vrID)
		if err != nil {
			return builds.Build{}, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:      buildID,
		Name:    name,
		JobName: job,
		Status:  builds.StatusPending,
	}, nil
}

func (db *sqldb) GetNextPendingBuild(job string) (builds.Build, builds.VersionedResources, error) {
	var id int
	var name string

	err := db.conn.QueryRow(`
		SELECT id, name
		FROM builds
		WHERE job_name = $1
		AND status = 'pending'
		AND scheduled = false
		ORDER BY id ASC
		LIMIT 1
	`, job).Scan(&id, &name)
	if err != nil {
		return builds.Build{}, builds.VersionedResources{}, err
	}

	inputs, _, err := db.GetBuildResources(job, name)
	if err != nil {
		return builds.Build{}, builds.VersionedResources{}, err
	}

	vrs := make([]builds.VersionedResource, len(inputs))
	for i, input := range inputs {
		vrs[i] = input.VersionedResource
	}

	return builds.Build{
		ID:      id,
		Name:    name,
		JobName: job,
		Status:  builds.StatusPending,
	}, vrs, nil
}

func (db *sqldb) GetResourceHistory(resource string) ([]*VersionHistory, error) {
	rows, err := db.conn.Query(`
		SELECT v.id, v.resource_name, v.type, v.version, v.source, v.metadata, b.id, b.job_name, b.name, b.status, b.abort_url
		FROM versioned_resources v, builds b
		WHERE v.resource_name = $1
		AND (
			EXISTS (SELECT 1 FROM build_inputs WHERE build_id = b.id AND versioned_resource_id = v.id)
			OR EXISTS (SELECT 1 FROM build_outputs WHERE build_id = b.id AND versioned_resource_id = v.id)
		)
		ORDER BY v.id DESC, b.id ASC
	`, resource)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	hs := []*VersionHistory{}
	vhs := map[int]*VersionHistory{}
	jhs := map[int]map[string]*JobHistory{}

	for rows.Next() {
		var vrID int
		var vr builds.VersionedResource

		var jobName string

		var buildID int
		var buildName string
		var buildStatus string
		var buildAbortURL sql.NullString

		var versionString, sourceString, metadataString string

		err := rows.Scan(
			&vrID, &vr.Name, &vr.Type, &versionString, &sourceString, &metadataString,
			&buildID, &jobName, &buildName, &buildStatus, &buildAbortURL,
		)
		if err != nil {
			return nil, err
		}

		vh, found := vhs[vrID]
		if !found {
			err = json.Unmarshal([]byte(sourceString), &vr.Source)
			if err != nil {
				return nil, err
			}

			err = json.Unmarshal([]byte(versionString), &vr.Version)
			if err != nil {
				return nil, err
			}

			err = json.Unmarshal([]byte(metadataString), &vr.Metadata)
			if err != nil {
				return nil, err
			}

			vh = &VersionHistory{
				VersionedResource: vr,
			}

			hs = append(hs, vh)

			vhs[vrID] = vh
			jhs[vrID] = map[string]*JobHistory{}
		}

		jh, found := jhs[vrID][jobName]
		if !found {
			jh = &JobHistory{
				JobName: jobName,
			}

			vh.Jobs = append(vh.Jobs, jh)

			jhs[vrID][jobName] = jh
		}

		jh.Builds = append(jh.Builds, builds.Build{
			ID:       buildID,
			Name:     buildName,
			JobName:  jobName,
			Status:   builds.Status(buildStatus),
			AbortURL: buildAbortURL.String,
		})
	}

	return hs, nil
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
		INSERT INTO versioned_resources (resource_name, type, version, source, metadata)
		SELECT $1, $2, $3, $4, $5
		WHERE NOT EXISTS (
			SELECT 1
			FROM versioned_resources
			WHERE resource_name = $1
			AND type = $2
			AND version = $3
		)
	`, vr.Name, vr.Type, string(versionJSON), string(sourceJSON), string(metadataJSON))
	if err != nil {
		return 0, err
	}

	// separate from above, as it conditionally inserts (can't use RETURNING)
	err = tx.QueryRow(`
			UPDATE versioned_resources
			SET source = $4, metadata = $5
			WHERE resource_name = $1
			AND type = $2
			AND version = $3
			RETURNING id
		`, vr.Name, vr.Type, string(versionJSON), string(sourceJSON), string(metadataJSON)).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}
