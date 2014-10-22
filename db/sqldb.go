package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type sqldb struct {
	logger lager.Logger
	conn   *sql.DB
}

const buildColumns = "id, name, job_name, status, guid, endpoint, start_time, end_time"

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
		SELECT `+buildColumns+`
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
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *sqldb) GetAllBuilds() ([]builds.Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + buildColumns + `
		FROM builds
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []builds.Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *sqldb) GetAllStartedBuilds() ([]builds.Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + buildColumns + `
		FROM builds
		WHERE status = 'started'
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []builds.Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *sqldb) GetBuild(buildID int) (builds.Build, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE id = $1
	`, buildID))
}

func (db *sqldb) GetJobBuild(job string, name string) (builds.Build, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND name = $2
	`, job, name))
}

func (db *sqldb) GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	rows, err := db.conn.Query(`
		SELECT v.resource_name, v.type, v.source, v.version, v.metadata,
		NOT EXISTS (
			SELECT 1
			FROM build_inputs, builds
			WHERE versioned_resource_id = v.id
			AND job_name = b.job_name
			AND build_id = id
			AND build_id < b.id
		)
		FROM versioned_resources v, build_inputs i, builds b
		WHERE b.id = $1
		AND i.build_id = b.id
		AND i.versioned_resource_id = v.id
	`, buildID)
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
		WHERE b.id = $1
		AND o.build_id = b.id
		AND o.versioned_resource_id = v.id
		AND NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE versioned_resource_id = v.id
			AND build_id = b.id
		)
	`, buildID)
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
	rows, err := db.conn.Query(`
		SELECT `+buildColumns+`
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
			SELECT `+buildColumns+`
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

	return scanBuild(rows)
}

func (db *sqldb) GetJobFinishedAndNextBuild(job string) (*builds.Build, *builds.Build, error) {
	var finished *builds.Build
	var next *builds.Build

	finishedBuild, err := scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND status NOT IN ('pending', 'started')
		ORDER BY id DESC
		LIMIT 1
	`, job))
	if err == nil {
		finished = &finishedBuild
	} else if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}

	nextBuild, err := scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND status IN ('pending', 'started')
		ORDER BY id ASC
		LIMIT 1
	`, job))
	if err == nil {
		next = &nextBuild
	} else if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}

	return finished, next, nil
}

func (db *sqldb) CreateJobBuild(job string) (builds.Build, error) {
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

	build, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds(name, job_name, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+buildColumns+`
	`, name, job))
	if err != nil {
		return builds.Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return builds.Build{}, err
	}

	return build, nil
}

func (db *sqldb) CreateOneOffBuild() (builds.Build, error) {
	return scanBuild(db.conn.QueryRow(`
		INSERT INTO builds(name, status)
		VALUES (nextval('one_off_name'), 'pending')
		RETURNING ` + buildColumns + `
	`))
}

func (db *sqldb) ScheduleBuild(buildID int, serial bool) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE builds AS b
		SET scheduled = true

		-- only the given build
		WHERE b.id = $1
		AND b.status = 'pending'

		-- if serial, only if it's the nextmost pending
		AND (
			NOT $2 OR id IN (
				SELECT p.id
				FROM builds p
				WHERE p.job_name = b.job_name
				AND p.status = 'pending'
				ORDER BY p.id ASC
				LIMIT 1
			)
		)

		-- if serial, not if another build is started or scheduled
		AND NOT (
			$2 AND EXISTS (
				SELECT 1
				FROM builds s
				WHERE s.job_name = b.job_name
				AND (s.status = 'started' OR (s.status = 'pending' AND s.scheduled = true))
			)
		)
	`, buildID, serial)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *sqldb) StartBuild(buildID int, guid, endpoint string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET status = 'started', guid = $2, endpoint = $3
		WHERE id = $1
		AND status = 'pending'
	`, buildID, guid, endpoint)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *sqldb) SaveBuildStartTime(buildID int, startTime time.Time) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET start_time = $2
		WHERE id = $1
	`, buildID, startTime)
	if err != nil {
		return err
	}

	return nil
}

func (db *sqldb) SaveBuildEndTime(buildID int, endTime time.Time) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET end_time = $2
		WHERE id = $1
	`, buildID, endTime)
	if err != nil {
		return err
	}

	return nil
}

func (db *sqldb) AbortBuild(buildID int) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET status = $2
		WHERE id = $1
	`, buildID, string(builds.StatusAborted))
	return err
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

func (db *sqldb) GetBuildEvents(buildID int) ([]BuildEvent, error) {
	var events []BuildEvent

	rows, err := db.conn.Query(`
		SELECT event_id, type, payload
		FROM build_events
		WHERE build_id = $1
		ORDER BY event_id ASC
	`, buildID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var event BuildEvent
		err := rows.Scan(&event.ID, &event.Type, &event.Payload)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

func (db *sqldb) SaveBuildEvent(buildID int, event BuildEvent) error {
	result, err := db.conn.Exec(`
		INSERT INTO build_events (build_id, event_id, type, payload)
		VALUES ($1, $2, $3, $4)
	`, buildID, event.ID, event.Type, event.Payload)
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
	from := []string{"builds"}
	conditions := []string{"job_name = $1"}
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
			fmt.Sprintf("i%d.build_id = id", i+1),
			fmt.Sprintf("i%d.versioned_resource_id = $%d", i+1, len(params)),
		)
	}

	return scanBuild(db.conn.QueryRow(fmt.Sprintf(`
		SELECT `+buildColumns+`
		FROM %s
		WHERE %s
		`,
		strings.Join(from, ", "),
		strings.Join(conditions, "\nAND ")),
		params...,
	))
}

func (db *sqldb) CreateJobBuildWithInputs(job string, inputs builds.VersionedResources) (builds.Build, error) {
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

	build, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds(name, job_name, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+buildColumns+`
	`, name, job))
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
		`, build.ID, vrID)
		if err != nil {
			return builds.Build{}, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return builds.Build{}, err
	}

	return build, nil
}

func (db *sqldb) GetNextPendingBuild(job string) (builds.Build, builds.VersionedResources, error) {
	build, err := scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND status = 'pending'
		AND scheduled = false
		ORDER BY id ASC
		LIMIT 1
	`, job))
	if err != nil {
		return builds.Build{}, builds.VersionedResources{}, err
	}

	inputs, _, err := db.GetBuildResources(build.ID)
	if err != nil {
		return builds.Build{}, builds.VersionedResources{}, err
	}

	vrs := make([]builds.VersionedResource, len(inputs))
	for i, input := range inputs {
		vrs[i] = input.VersionedResource
	}

	return build, vrs, nil
}

func (db *sqldb) GetResourceHistory(resource string) ([]*VersionHistory, error) {
	hs := []*VersionHistory{}
	vhs := map[int]*VersionHistory{}

	inputHs := map[int]map[string]*JobHistory{}
	outputHs := map[int]map[string]*JobHistory{}
	seenInputs := map[int]map[int]bool{}

	vrRows, err := db.conn.Query(`
		SELECT v.id, v.resource_name, v.type, v.version, v.source, v.metadata
		FROM versioned_resources v
		WHERE v.resource_name = $1
		ORDER BY v.id DESC
	`, resource)
	if err != nil {
		return nil, err
	}

	defer vrRows.Close()

	for vrRows.Next() {
		var vrID int
		var vr builds.VersionedResource

		var versionString, sourceString, metadataString string

		err := vrRows.Scan(&vrID, &vr.Name, &vr.Type, &versionString, &sourceString, &metadataString)
		if err != nil {
			return nil, err
		}

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

		vhs[vrID] = &VersionHistory{
			VersionedResource: vr,
		}

		hs = append(hs, vhs[vrID])

		inputHs[vrID] = map[string]*JobHistory{}
		outputHs[vrID] = map[string]*JobHistory{}
		seenInputs[vrID] = map[int]bool{}
	}

	for id, vh := range vhs {
		inRows, err := db.conn.Query(`
			SELECT `+buildColumns+`
			FROM builds, build_inputs i
			WHERE i.versioned_resource_id = $1
			AND i.build_id = id
			ORDER BY id ASC
		`, id)
		if err != nil {
			return nil, err
		}

		defer inRows.Close()

		outRows, err := db.conn.Query(`
			SELECT `+buildColumns+`
			FROM builds, build_outputs o
			WHERE o.versioned_resource_id = $1
			AND o.build_id = id
			ORDER BY id ASC
		`, id)
		if err != nil {
			return nil, err
		}

		defer outRows.Close()

		for inRows.Next() {
			inBuild, err := scanBuild(inRows)
			if err != nil {
				return nil, err
			}

			seenInputs[id][inBuild.ID] = true

			inputH, found := inputHs[id][inBuild.JobName]
			if !found {
				inputH = &JobHistory{
					JobName: inBuild.JobName,
				}

				vh.InputsTo = append(vh.InputsTo, inputH)

				inputHs[id][inBuild.JobName] = inputH
			}

			inputH.Builds = append(inputH.Builds, inBuild)
		}

		for outRows.Next() {
			outBuild, err := scanBuild(outRows)
			if err != nil {
				return nil, err
			}

			if seenInputs[id][outBuild.ID] {
				// don't show implicit outputs
				continue
			}

			outputH, found := outputHs[id][outBuild.JobName]
			if !found {
				outputH = &JobHistory{
					JobName: outBuild.JobName,
				}

				vh.OutputsOf = append(vh.OutputsOf, outputH)

				outputHs[id][outBuild.JobName] = outputH
			}

			outputH.Builds = append(outputH.Builds, outBuild)
		}
	}

	return hs, nil
}

func (db *sqldb) AcquireResourceCheckingLock() (Lock, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("LOCK TABLE resource_checking_lock")
	if err != nil {
		return nil, err
	}

	return &txLock{tx}, nil
}

func (db *sqldb) AcquireBuildSchedulingLock() (Lock, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("LOCK TABLE build_scheduling_lock")
	if err != nil {
		return nil, err
	}

	return &txLock{tx}, nil
}

type txLock struct {
	tx *sql.Tx
}

func (lock *txLock) Release() error {
	return lock.tx.Commit()
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

type scannable interface {
	Scan(destinations ...interface{}) error
}

func scanBuild(row scannable) (builds.Build, error) {
	var id int
	var name string
	var jobName sql.NullString
	var status string
	var guid sql.NullString
	var endpoint sql.NullString
	var startTime pq.NullTime
	var endTime pq.NullTime

	err := row.Scan(&id, &name, &jobName, &status, &guid, &endpoint, &startTime, &endTime)
	if err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:      id,
		Name:    name,
		JobName: jobName.String,
		Status:  builds.Status(status),

		Guid:     guid.String,
		Endpoint: endpoint.String,

		StartTime: startTime.Time,
		EndTime:   endTime.Time,
	}, nil
}
