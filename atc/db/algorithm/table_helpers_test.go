package algorithm_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/lib/pq"
	. "github.com/onsi/gomega"
)

type DB struct {
	BuildInputs  []DBRow
	BuildOutputs []DBRow
	Resources    []DBRow
}

type DBRow struct {
	Job        string
	BuildID    int
	Resource   string
	Version    string
	CheckOrder int
	VersionID  int
	Disabled   bool
}

type Example struct {
	LoadDB string
	DB     DB
	Inputs Inputs
	Result Result
}

type Inputs []Input

type Input struct {
	Name     string
	Resource string
	Passed   []string
	Version  Version
}

type Version struct {
	Every  bool
	Latest bool
	Pinned string
}

type Result struct {
	OK     bool
	Values map[string]string
}

type StringMapping map[string]int

func (mapping StringMapping) ID(str string) int {
	id, found := mapping[str]
	if !found {
		id = len(mapping) + 1
		mapping[str] = id
	}

	return id
}

func (mapping StringMapping) Name(id int) string {
	for mappingName, mappingID := range mapping {
		if id == mappingID {
			return mappingName
		}
	}

	panic(fmt.Sprintf("no name found for %d", id))
}

const CurrentJobName = "current"

func (example Example) Run() {
	db := &algorithm.VersionsDB{
		Runner: dbConn,
	}

	jobIDs := StringMapping{}
	resourceIDs := StringMapping{}
	versionIDs := StringMapping{}

	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(dbConn)

	teamID := 1
	_, err := psql.Insert("teams").
		Columns("id", "name").
		Values(teamID, "algorithm").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	pipelineID := 1
	_, err = psql.Insert("pipelines").
		Columns("id", "team_id", "name").
		Values(pipelineID, teamID, "algorithm").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	insertJob := func(name string) int {
		jobID := jobIDs.ID(name)

		_, err := psql.Insert("jobs").
			Columns("id", "pipeline_id", "name", "config").
			Values(jobID, pipelineID, name, "{}").
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		return jobID
	}

	insertResource := func(name string) int {
		resourceID := resourceIDs.ID(name)

		_, err := psql.Insert("resource_configs").
			Columns("id", "source_hash").
			Values(resourceID, "bogus-hash").
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		_, err = psql.Insert("resource_config_scopes").
			Columns("id", "resource_config_id").
			Values(resourceID, resourceID).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		_, err = psql.Insert("resources").
			Columns("id", "name", "config", "pipeline_id", "resource_config_id", "resource_config_scope_id").
			Values(resourceID, name, "{}", pipelineID, resourceID, resourceID).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		return resourceID
	}

	insertRowVersion := func(row DBRow) {
		versionID := versionIDs.ID(row.Version)

		resourceID := insertResource(row.Resource)

		_, err = psql.Insert("resource_config_versions").
			Columns("id", "resource_config_scope_id", "version", "version_md5", "check_order").
			Values(versionID, resourceID, "{}", sq.Expr("md5(?)", row.Version), row.CheckOrder).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		if row.Disabled {
			_, err = psql.Insert("resource_disabled_versions").
				Columns("resource_id", "version_md5").
				Values(resourceID, sq.Expr("md5(?)", row.Version)).
				Suffix("ON CONFLICT DO NOTHING").
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}
	}

	insertRowBuild := func(row DBRow) {
		jobID := insertJob(row.Job)

		var existingJobID int
		err := psql.Insert("builds").
			Columns("team_id", "id", "job_id", "name", "status").
			Values(teamID, row.BuildID, jobID, "some-name", "succeeded").
			Suffix("ON CONFLICT (id) DO UPDATE SET name = excluded.name").
			Suffix("RETURNING job_id").
			QueryRow().
			Scan(&existingJobID)
		Expect(err).ToNot(HaveOccurred())

		Expect(existingJobID).To(Equal(jobID), fmt.Sprintf("build ID %d already used by job other than %s", row.BuildID, row.Job))
	}

	if example.LoadDB != "" {
		dbFile, err := os.Open(example.LoadDB)
		Expect(err).ToNot(HaveOccurred())

		gr, err := gzip.NewReader(dbFile)
		Expect(err).ToNot(HaveOccurred())

		log.Println("LOADING DB", example.LoadDB)
		var legacyDB algorithm.LegacyVersionsDB
		err = json.NewDecoder(gr).Decode(&legacyDB)
		Expect(err).ToNot(HaveOccurred())
		log.Println("LOADED")

		log.Println("IMPORTING", len(legacyDB.JobIDs), len(legacyDB.ResourceIDs), len(legacyDB.ResourceVersions), len(legacyDB.BuildInputs), len(legacyDB.BuildOutputs))

		for name, id := range legacyDB.JobIDs {
			jobIDs[name] = id

			insertJob(name)
		}

		for name, id := range legacyDB.ResourceIDs {
			resourceIDs[name] = id

			insertResource(name)
		}

		log.Println("IMPORTING VERSIONS")

		tx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err := tx.Prepare(pq.CopyIn("resource_config_versions", "id", "resource_config_scope_id", "version", "version_md5", "check_order"))
		Expect(err).ToNot(HaveOccurred())

		for _, row := range legacyDB.ResourceVersions {
			name := fmt.Sprintf("imported-r%dv%d", row.ResourceID, row.VersionID)
			versionIDs[name] = row.VersionID

			_, err := stmt.Exec(row.VersionID, row.ResourceID, "{}", strconv.Itoa(row.VersionID), row.CheckOrder)
			Expect(err).ToNot(HaveOccurred())
		}

		_, err = stmt.Exec()
		Expect(err).ToNot(HaveOccurred())

		err = stmt.Close()
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		log.Println("IMPORTING BUILDS")

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err = tx.Prepare(pq.CopyIn("builds", "team_id", "id", "job_id", "name", "status"))
		Expect(err).ToNot(HaveOccurred())

		imported := map[int]bool{}

		for _, row := range legacyDB.BuildInputs {
			if imported[row.BuildID] {
				continue
			}

			_, err := stmt.Exec(teamID, row.BuildID, row.JobID, "some-name", "succeeded")
			Expect(err).ToNot(HaveOccurred())

			imported[row.BuildID] = true
		}

		for _, row := range legacyDB.BuildOutputs {
			if imported[row.BuildID] {
				continue
			}

			_, err := stmt.Exec(teamID, row.BuildID, row.JobID, "some-name", "succeeded")
			Expect(err).ToNot(HaveOccurred())

			imported[row.BuildID] = true
		}

		_, err = stmt.Exec()
		Expect(err).ToNot(HaveOccurred())

		err = stmt.Close()
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		log.Println("IMPORTING INPUTS")

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err = tx.Prepare(pq.CopyIn("build_resource_config_version_inputs", "build_id", "resource_id", "version_md5", "name"))
		Expect(err).ToNot(HaveOccurred())

		for i, row := range legacyDB.BuildInputs {
			_, err := stmt.Exec(row.BuildID, row.ResourceID, strconv.Itoa(row.VersionID), strconv.Itoa(i))
			Expect(err).ToNot(HaveOccurred())
		}

		_, err = stmt.Exec()
		Expect(err).ToNot(HaveOccurred())

		err = stmt.Close()
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		log.Println("IMPORTING OUTPUTS")

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err = tx.Prepare(pq.CopyIn("build_resource_config_version_outputs", "build_id", "resource_id", "version_md5", "name"))
		Expect(err).ToNot(HaveOccurred())

		for i, row := range legacyDB.BuildOutputs {
			_, err := stmt.Exec(row.BuildID, row.ResourceID, strconv.Itoa(row.VersionID), strconv.Itoa(i))
			Expect(err).ToNot(HaveOccurred())
		}

		_, err = stmt.Exec()
		Expect(err).ToNot(HaveOccurred())

		err = stmt.Close()
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		log.Println("DONE IMPORTING")
	} else {
		for _, row := range example.DB.Resources {
			insertRowVersion(row)
		}

		for _, row := range example.DB.BuildInputs {
			insertRowVersion(row)
			insertRowBuild(row)

			resourceID := resourceIDs.ID(row.Resource)

			_, err := psql.Insert("build_resource_config_version_inputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(row.BuildID, resourceID, sq.Expr("md5(?)", row.Version), row.Resource).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		for _, row := range example.DB.BuildOutputs {
			insertRowVersion(row)
			insertRowBuild(row)

			resourceID := resourceIDs.ID(row.Resource)

			_, err := psql.Insert("build_resource_config_version_outputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(row.BuildID, resourceID, sq.Expr("md5(?)", row.Version), row.Resource).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}
	}

	for _, input := range example.Inputs {
		insertResource(input.Resource)
	}

	inputConfigs := make(algorithm.InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := algorithm.JobSet{}
		for _, jobName := range input.Passed {
			passed[jobIDs.ID(jobName)] = struct{}{}
		}

		var versionID int
		if input.Version.Pinned != "" {
			versionID = versionIDs.ID(input.Version.Pinned)
		}

		inputConfigs[i] = algorithm.InputConfig{
			Name:            input.Name,
			Passed:          passed,
			ResourceID:      resourceIDs.ID(input.Resource),
			UseEveryVersion: input.Version.Every,
			PinnedVersionID: versionID,
			JobID:           jobIDs.ID(CurrentJobName),
		}
	}

	resolved, ok, err := inputConfigs.Resolve(db)
	Expect(err).ToNot(HaveOccurred())

	prettyValues := map[string]string{}
	for name, inputSource := range resolved {
		prettyValues[name] = versionIDs.Name(inputSource.InputVersion.VersionID)
	}

	actualResult := Result{OK: ok, Values: prettyValues}

	Expect(actualResult).To(Equal(example.Result))
}
