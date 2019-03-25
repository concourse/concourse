package algorithm_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/algorithm"
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

	var teamID int
	err := psql.Insert("teams").
		Columns("name").
		Values("algorithm").
		Suffix("RETURNING id").
		QueryRow().
		Scan(&teamID)
	Expect(err).ToNot(HaveOccurred())

	var pipelineID int
	err = psql.Insert("pipelines").
		Columns("team_id", "name").
		Values(teamID, "algorithm").
		Suffix("RETURNING id").
		QueryRow().
		Scan(&pipelineID)
	Expect(err).ToNot(HaveOccurred())

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
	}

	insertRowBuild := func(row DBRow) {
		jobID := jobIDs.ID(row.Job)

		_, err := psql.Insert("jobs").
			Columns("id", "pipeline_id", "name", "config").
			Values(jobID, pipelineID, row.Job, "{}").
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		_, err = psql.Insert("builds").
			Columns("team_id", "id", "job_id", "name", "status").
			Values(teamID, row.BuildID, jobID, "some-name", "succeeded").
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())
	}

	if example.LoadDB != "" {
		dbFile, err := os.Open(example.LoadDB)
		Expect(err).ToNot(HaveOccurred())

		gr, err := gzip.NewReader(dbFile)
		Expect(err).ToNot(HaveOccurred())

		log.Println("IMPORTING")
		var legacyDB algorithm.LegacyVersionsDB
		err = json.NewDecoder(gr).Decode(&legacyDB)
		Expect(err).ToNot(HaveOccurred())

		jobNames := map[int]string{}
		for name, id := range legacyDB.JobIDs {
			jobIDs[name] = id
			jobNames[id] = name
		}

		resourceNames := map[int]string{}
		for name, id := range legacyDB.ResourceIDs {
			resourceIDs[name] = id
			resourceNames[id] = name
		}

		versionNames := map[int]string{}
		for _, v := range legacyDB.ResourceVersions {
			versionNames[v.VersionID] = fmt.Sprintf("imported-r%dv%d", v.ResourceID, v.VersionID)
		}

		for _, row := range legacyDB.ResourceVersions {
			resource := resourceNames[row.ResourceID]
			version := versionNames[row.VersionID]

			insertRowVersion(DBRow{
				Version:    version,
				Resource:   resource,
				CheckOrder: row.CheckOrder,
			})
		}

		for _, row := range legacyDB.BuildInputs {
			resource := resourceNames[row.ResourceID]
			version := versionNames[row.VersionID]
			job := jobNames[row.JobID]

			insertRowVersion(DBRow{
				Version:    version,
				Resource:   resource,
				CheckOrder: row.CheckOrder,
			})

			insertRowBuild(DBRow{
				Job:     job,
				BuildID: row.BuildID,
			})

			_, err := psql.Insert("build_resource_config_version_inputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(row.BuildID, row.ResourceID, sq.Expr("md5(?)", version), row.InputName).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		for i, row := range legacyDB.BuildOutputs {
			resource := resourceNames[row.ResourceID]
			version := versionNames[row.VersionID]
			job := jobNames[row.JobID]

			insertRowVersion(DBRow{
				Version:    version,
				Resource:   resource,
				CheckOrder: row.CheckOrder,
			})

			insertRowBuild(DBRow{
				Job:     job,
				BuildID: row.BuildID,
			})

			_, err := psql.Insert("build_resource_config_version_outputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(row.BuildID, row.ResourceID, sq.Expr("md5(?)", version), fmt.Sprintf("%s-output-%d", resource, i)).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}
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
	for name, inputVersion := range resolved {
		prettyValues[name] = versionIDs.Name(inputVersion.VersionID)
	}

	actualResult := Result{OK: ok, Values: prettyValues}

	// time.Sleep(time.Hour)
	Expect(actualResult).To(Equal(example.Result))
}
