package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
)

type SQLDB struct {
	logger lager.Logger

	conn *sql.DB
	bus  *notificationsBus
}

const buildColumns = "id, name, job_name, status, engine, engine_metadata, start_time, end_time"
const qualifiedBuildColumns = "b.id, b.name, b.job_name, b.status, b.engine, b.engine_metadata, b.start_time, b.end_time"

func NewSQL(
	logger lager.Logger,
	sqldbConnection *sql.DB,
	listener *pq.Listener,
) *SQLDB {
	return &SQLDB{
		logger: logger,

		conn: sqldbConnection,
		bus:  newNotificationsBus(listener),
	}
}

func (db *SQLDB) GetConfig() (atc.Config, error) {
	var configBlob []byte
	err := db.conn.QueryRow(`
		SELECT config
		FROM config
	`).Scan(&configBlob)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.Config{}, nil
		} else {
			return atc.Config{}, err
		}
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, err
	}

	return config, nil
}

func (db *SQLDB) SaveConfig(config atc.Config) error {
	payload, err := json.Marshal(config)
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE config
		SET config = $1
	`, payload)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		_, err := tx.Exec(`
			INSERT INTO config (config)
			VALUES ($1)
		`, payload)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *SQLDB) GetAllJobBuilds(job string) ([]Build, error) {
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

	bs := []Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) GetAllBuilds() ([]Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + buildColumns + `
		FROM builds
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) GetAllStartedBuilds() ([]Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + buildColumns + `
		FROM builds
		WHERE status = 'started'
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) GetBuild(buildID int) (Build, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE id = $1
	`, buildID))
}

func (db *SQLDB) GetJobBuild(job string, name string) (Build, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND name = $2
	`, job, name))
}

func (db *SQLDB) GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	rows, err := db.conn.Query(`
		SELECT i.name, v.resource_name, v.type, v.source, v.version, v.metadata,
		NOT EXISTS (
			SELECT 1
			FROM build_inputs ci, builds cb
			WHERE versioned_resource_id = v.id
			AND cb.job_name = b.job_name
			AND ci.build_id = cb.id
			AND ci.build_id < b.id
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
		var inputName string
		var vr VersionedResource
		var firstOccurrence bool

		var source, version, metadata string
		err := rows.Scan(&inputName, &vr.Resource, &vr.Type, &source, &version, &metadata, &firstOccurrence)
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
			Name:              inputName,
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
		var vr VersionedResource

		var source, version, metadata string
		err := rows.Scan(&vr.Resource, &vr.Type, &source, &version, &metadata)
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

func (db *SQLDB) GetCurrentBuild(job string) (Build, error) {
	rows, err := db.conn.Query(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND status != 'pending'
		ORDER BY id DESC
		LIMIT 1
	`, job)
	if err != nil {
		return Build{}, err
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
			return Build{}, err
		}

		defer rows.Close()

		rows.Next()
	}

	return scanBuild(rows)
}

func (db *SQLDB) GetJobFinishedAndNextBuild(job string) (*Build, *Build, error) {
	var finished *Build
	var next *Build

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

func (db *SQLDB) CreateJobBuild(job string) (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	err = registerJob(tx, job)
	if err != nil {
		return Build{}, err
	}

	var name string
	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1
		RETURNING build_number_seq
	`, job).Scan(&name)
	if err != nil {
		return Build{}, err
	}

	build, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds (name, job_name, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+buildColumns+`
	`, name, job))
	if err != nil {
		return Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (db *SQLDB) CreateOneOffBuild() (Build, error) {
	return scanBuild(db.conn.QueryRow(`
		INSERT INTO builds(name, status)
		VALUES (nextval('one_off_name'), 'pending')
		RETURNING ` + buildColumns + `
	`))
}

func (db *SQLDB) ScheduleBuild(buildID int, serial bool) (bool, error) {
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

func (db *SQLDB) StartBuild(buildID int, engine, metadata string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE builds
		SET status = 'started', engine = $2, engine_metadata = $3
		WHERE id = $1
		AND status = 'pending'
	`, buildID, engine, metadata)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (db *SQLDB) SaveBuildStartTime(buildID int, startTime time.Time) error {
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

func (db *SQLDB) SaveBuildEndTime(buildID int, endTime time.Time) error {
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

func (db *SQLDB) SaveBuildInput(buildID int, input BuildInput) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	vrID, err := db.saveVersionedResource(tx, input.VersionedResource)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO build_inputs (build_id, versioned_resource_id, name)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE build_id = $1
			AND versioned_resource_id = $2
			AND name = $3
		)
	`, buildID, vrID, input.Name)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *SQLDB) SaveBuildOutput(buildID int, vr VersionedResource) error {
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

func (db *SQLDB) SaveBuildStatus(buildID int, status Status) error {
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

func (db *SQLDB) GetBuildEvents(buildID int, from uint) (BuildEventSource, error) {
	channel := buildEventsChannel(buildID)

	notify, err := db.bus.Listen(channel)
	if err != nil {
		return nil, err
	}

	return newSQLDBBuildEventSource(
		buildID,
		db.conn,
		db.bus,
		notify,
		from,
	), nil
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func (db *SQLDB) GetLastBuildEventID(buildID int) (int, error) {
	var id int
	err := db.conn.QueryRow(`
		SELECT event_id
		FROM build_events
		WHERE build_id = $1
		ORDER BY event_id DESC
		LIMIT 1
	`, buildID).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (db *SQLDB) SaveBuildEvent(buildID int, event BuildEvent) error {
	_, err := db.conn.Exec(`
		INSERT INTO build_events (build_id, event_id, type, payload, version)
		SELECT $1, $2, $3, $4, $5
		WHERE NOT EXISTS (
			SELECT 1
			FROM build_events
			WHERE build_id = $1
			AND event_id = $2
		)
	`, buildID, event.ID, event.Type, event.Payload, event.Version)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec("NOTIFY " + buildEventsChannel(buildID))
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) CompleteBuild(buildID int) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET completed = true
		WHERE id = $1
	`, buildID)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec("NOTIFY " + buildEventsChannel(buildID))
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) SaveVersionedResource(vr VersionedResource) error {
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

func (db *SQLDB) GetLatestVersionedResource(name string) (VersionedResource, error) {
	var sourceBytes, versionBytes, metadataBytes string

	vr := VersionedResource{
		Resource: name,
	}

	err := db.conn.QueryRow(`
		SELECT type, source, version, metadata
		FROM versioned_resources
		WHERE resource_name = $1
		ORDER BY id DESC
		LIMIT 1
	`, name).Scan(&vr.Type, &sourceBytes, &versionBytes, &metadataBytes)
	if err != nil {
		return VersionedResource{}, err
	}

	err = json.Unmarshal([]byte(sourceBytes), &vr.Source)
	if err != nil {
		return VersionedResource{}, err
	}

	err = json.Unmarshal([]byte(versionBytes), &vr.Version)
	if err != nil {
		return VersionedResource{}, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &vr.Metadata)
	if err != nil {
		return VersionedResource{}, err
	}

	return vr, nil
}

func (db *SQLDB) GetLatestInputVersions(inputs []atc.JobInputConfig) (VersionedResources, error) {
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

	vrs := []VersionedResource{}

	for i, _ := range inputs {
		var vr VersionedResource

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
		), params...).Scan(&id, &vr.Resource, &vr.Type, &source, &version, &metadata)

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

func (db *SQLDB) GetJobBuildForInputs(job string, inputs []BuildInput) (Build, error) {
	from := []string{"builds b"}
	conditions := []string{"job_name = $1"}
	params := []interface{}{job}

	for i, input := range inputs {
		vr := input.VersionedResource

		versionBytes, err := json.Marshal(vr.Version)
		if err != nil {
			return Build{}, err
		}

		var id int

		err = db.conn.QueryRow(`
			SELECT id
			FROM versioned_resources
			WHERE resource_name = $1
			AND type = $2
			AND version = $3
		`, vr.Resource, vr.Type, string(versionBytes)).Scan(&id)
		if err != nil {
			return Build{}, err
		}

		from = append(from, fmt.Sprintf("build_inputs i%d", i+1))
		params = append(params, id, input.Name)

		conditions = append(conditions,
			fmt.Sprintf("i%d.build_id = id", i+1),
			fmt.Sprintf("i%d.versioned_resource_id = $%d", i+1, len(params)-1),
			fmt.Sprintf("i%d.name = $%d", i+1, len(params)),
		)
	}

	return scanBuild(db.conn.QueryRow(fmt.Sprintf(`
		SELECT `+qualifiedBuildColumns+`
		FROM %s
		WHERE %s
		`,
		strings.Join(from, ", "),
		strings.Join(conditions, "\nAND ")),
		params...,
	))
}

func (db *SQLDB) CreateJobBuildWithInputs(job string, inputs []BuildInput) (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	err = registerJob(tx, job)
	if err != nil {
		return Build{}, err
	}

	var name string
	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1
		RETURNING build_number_seq
	`, job).Scan(&name)
	if err != nil {
		return Build{}, err
	}

	build, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds (name, job_name, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+buildColumns+`
	`, name, job))
	if err != nil {
		return Build{}, err
	}

	for _, input := range inputs {
		vrID, err := db.saveVersionedResource(tx, input.VersionedResource)
		if err != nil {
			return Build{}, err
		}

		_, err = tx.Exec(`
			INSERT INTO build_inputs (build_id, versioned_resource_id, name)
			VALUES ($1, $2, $3)
		`, build.ID, vrID, input.Name)
		if err != nil {
			return Build{}, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (db *SQLDB) GetNextPendingBuild(job string) (Build, []BuildInput, error) {
	build, err := scanBuild(db.conn.QueryRow(`
		SELECT `+buildColumns+`
		FROM builds
		WHERE job_name = $1
		AND status = 'pending'
		ORDER BY id ASC
		LIMIT 1
	`, job))
	if err != nil {
		return Build{}, nil, err
	}

	inputs, _, err := db.GetBuildResources(build.ID)
	if err != nil {
		return Build{}, nil, err
	}

	return build, inputs, nil
}

func (db *SQLDB) GetResourceHistory(resource string) ([]*VersionHistory, error) {
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
		var vr VersionedResource

		var versionString, sourceString, metadataString string

		err := vrRows.Scan(&vrID, &vr.Resource, &vr.Type, &versionString, &sourceString, &metadataString)
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
			SELECT `+qualifiedBuildColumns+`
			FROM builds b, build_inputs i
			WHERE i.versioned_resource_id = $1
			AND i.build_id = b.id
			ORDER BY b.id ASC
		`, id)
		if err != nil {
			return nil, err
		}

		defer inRows.Close()

		outRows, err := db.conn.Query(`
			SELECT `+qualifiedBuildColumns+`
			FROM builds b, build_outputs o
			WHERE o.versioned_resource_id = $1
			AND o.build_id = b.id
			ORDER BY b.id ASC
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

func (db *SQLDB) acquireLock(lockType string, locks []NamedLock) (*txLock, error) {
	params := []interface{}{}
	refs := []string{}
	for i, lock := range locks {
		params = append(params, lock.Name())
		refs = append(refs, fmt.Sprintf("$%d", i+1))

		_, err := db.conn.Exec(`
			INSERT INTO locks (name)
			VALUES ($1)
		`, lock.Name())
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok {
				if pqErr.Code.Class().Name() == "integrity_constraint_violation" {
					// unique violation is ok; no way to atomically upsert
					continue
				}
			}

			return nil, err
		}
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := tx.Exec(`
	SELECT 1 FROM locks
	WHERE name IN (`+strings.Join(refs, ",")+`)
	FOR `+lockType+`
	`, params...)
	if err != nil {
		tx.Commit()
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Commit()
		return nil, err
	}
	if rowsAffected == 0 {
		tx.Commit()
		return nil, ErrLockRowNotPresentOrAlreadyDeleted
	}

	return &txLock{tx, db, locks}, nil
}

func (db *SQLDB) acquireLockLoop(lockType string, lock []NamedLock) (Lock, error) {
	for {
		lock, err := db.acquireLock(lockType, lock)
		if err != ErrLockRowNotPresentOrAlreadyDeleted {
			return lock, err
		}
	}
}

func (db *SQLDB) AcquireWriteLockImmediately(lock []NamedLock) (Lock, error) {
	return db.acquireLockLoop("UPDATE NOWAIT", lock)
}

func (db *SQLDB) AcquireWriteLock(lock []NamedLock) (Lock, error) {
	return db.acquireLockLoop("UPDATE", lock)
}

func (db *SQLDB) AcquireReadLock(lock []NamedLock) (Lock, error) {
	return db.acquireLockLoop("SHARE", lock)
}

func (db *SQLDB) ListLocks() ([]string, error) {
	rows, err := db.conn.Query("SELECT name FROM locks")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	locks := []string{}

	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			return nil, err
		}

		locks = append(locks, name)
	}

	return locks, nil
}

type txLock struct {
	tx         *sql.Tx
	db         *SQLDB
	namedLocks []NamedLock
}

func (lock *txLock) release() error {
	return lock.tx.Commit()
}

func (lock *txLock) cleanup() error {
	lockNames := []interface{}{}
	refs := []string{}
	for i, l := range lock.namedLocks {
		lockNames = append(lockNames, l.Name())
		refs = append(refs, fmt.Sprintf("$%d", i+1))
	}

	cleanupLock, err := lock.db.acquireLock("UPDATE NOWAIT", lock.namedLocks)
	if err != nil {
		return nil
	}

	_, err = cleanupLock.tx.Exec(`
		DELETE FROM locks
		WHERE name IN (`+strings.Join(refs, ",")+`)
	`, lockNames...)
	return cleanupLock.release()
}

func (lock *txLock) Release() error {
	err := lock.release()
	if err != nil {
		return err
	}

	return lock.cleanup()
}

func (db *SQLDB) saveVersionedResource(tx *sql.Tx, vr VersionedResource) (int, error) {
	_, err := tx.Exec(`
			INSERT INTO resources (name)
			SELECT $1
			WHERE NOT EXISTS (
				SELECT 1 FROM resources WHERE name = $1
			)
		`, vr.Resource)
	if err != nil {
		return 0, err
	}

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
	`, vr.Resource, vr.Type, string(versionJSON), string(sourceJSON), string(metadataJSON))
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
		`, vr.Resource, vr.Type, string(versionJSON), string(sourceJSON), string(metadataJSON)).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}

type scannable interface {
	Scan(destinations ...interface{}) error
}

func scanBuild(row scannable) (Build, error) {
	var id int
	var name string
	var jobName sql.NullString
	var status string
	var engine, engineMetadata sql.NullString
	var startTime pq.NullTime
	var endTime pq.NullTime

	err := row.Scan(&id, &name, &jobName, &status, &engine, &engineMetadata, &startTime, &endTime)
	if err != nil {
		return Build{}, err
	}

	return Build{
		ID:      id,
		Name:    name,
		JobName: jobName.String,
		Status:  Status(status),

		Engine:         engine.String,
		EngineMetadata: engineMetadata.String,

		StartTime: startTime.Time,
		EndTime:   endTime.Time,
	}, nil
}

func registerJob(tx *sql.Tx, name string) error {
	_, err := tx.Exec(`
		INSERT INTO jobs (name)
		SELECT $1
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs WHERE name = $1
		)
	`, name)
	return err
}
