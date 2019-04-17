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
	BuildPipes   []DBRow
	Resources    []DBRow
}

type DBRow struct {
	Job         string
	BuildID     int
	Resource    string
	Version     string
	CheckOrder  int
	VersionID   int
	Disabled    bool
	FromBuildID int
	ToBuildID   int
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
		Runner:             dbConn,
		DisabledVersionIDs: map[int]bool{},
	}

	setup := setupDB{
		teamID:      1,
		pipelineID:  1,
		psql:        sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(dbConn),
		jobIDs:      StringMapping{},
		resourceIDs: StringMapping{},
		versionIDs:  StringMapping{},
	}

	setup.insertTeamsPipelines()

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
			setup.jobIDs[name] = id

			setup.insertJob(name)
		}

		for name, id := range legacyDB.ResourceIDs {
			setup.resourceIDs[name] = id

			setup.insertResource(name)
		}

		log.Println("IMPORTING VERSIONS")

		tx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err := tx.Prepare(pq.CopyIn("resource_config_versions", "id", "resource_config_scope_id", "version", "version_md5", "check_order"))
		Expect(err).ToNot(HaveOccurred())

		for _, row := range legacyDB.ResourceVersions {
			name := fmt.Sprintf("imported-r%dv%d", row.ResourceID, row.VersionID)
			setup.versionIDs[name] = row.VersionID

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

			_, err := stmt.Exec(setup.teamID, row.BuildID, row.JobID, "some-name", "succeeded")
			Expect(err).ToNot(HaveOccurred())

			imported[row.BuildID] = true
		}

		for _, row := range legacyDB.BuildOutputs {
			if imported[row.BuildID] {
				continue
			}

			_, err := stmt.Exec(setup.teamID, row.BuildID, row.JobID, "some-name", "succeeded")
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

		stmt, err = tx.Prepare(pq.CopyIn("build_resource_config_version_inputs", "build_id", "resource_id", "version_md5", "name", "first_occurrence"))
		Expect(err).ToNot(HaveOccurred())

		for i, row := range legacyDB.BuildInputs {
			_, err := stmt.Exec(row.BuildID, row.ResourceID, strconv.Itoa(row.VersionID), strconv.Itoa(i), row.FirstOccurrence)
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
			setup.insertRowVersion(row)
		}

		for _, row := range example.DB.BuildInputs {
			setup.insertRowVersion(row)
			setup.insertRowBuild(row)

			resourceID := setup.resourceIDs.ID(row.Resource)

			_, err := setup.psql.Insert("build_resource_config_version_inputs").
				Columns("build_id", "resource_id", "version_md5", "name", "first_occurrence").
				Values(row.BuildID, resourceID, sq.Expr("md5(?)", row.Version), row.Resource, false).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		for _, row := range example.DB.BuildOutputs {
			setup.insertRowVersion(row)
			setup.insertRowBuild(row)

			resourceID := setup.resourceIDs.ID(row.Resource)

			_, err := setup.psql.Insert("build_resource_config_version_outputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(row.BuildID, resourceID, sq.Expr("md5(?)", row.Version), row.Resource).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		for _, row := range example.DB.BuildPipes {
			setup.insertBuildPipe(row)
		}
	}

	for _, input := range example.Inputs {
		setup.insertResource(input.Resource)
	}

	inputConfigs := make(algorithm.InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := algorithm.JobSet{}
		for _, jobName := range input.Passed {
			passed[setup.jobIDs.ID(jobName)] = struct{}{}
		}

		var versionID int
		if input.Version.Pinned != "" {
			versionID = setup.versionIDs.ID(input.Version.Pinned)
		}

		inputConfigs[i] = algorithm.InputConfig{
			Name:            input.Name,
			Passed:          passed,
			ResourceID:      setup.resourceIDs.ID(input.Resource),
			UseEveryVersion: input.Version.Every,
			PinnedVersionID: versionID,
			JobID:           setup.jobIDs.ID(CurrentJobName),
		}
	}

	rows, err := setup.psql.Select("rcv.id").
		From("resource_config_versions rcv").
		RightJoin("resource_disabled_versions rdv ON rdv.version_md5 = rcv.version_md5").
		Join("resources r ON r.resource_config_scope_id = rcv.resource_config_scope_id AND r.id = rdv.resource_id").
		Where(sq.Eq{
			"r.pipeline_id": 1,
		}).
		Query()
	Expect(err).ToNot(HaveOccurred())

	for rows.Next() {
		var versionID int

		err = rows.Scan(&versionID)
		Expect(err).ToNot(HaveOccurred())

		db.DisabledVersionIDs[versionID] = true
	}

	resolved, ok, err := inputConfigs.Resolve(db)
	Expect(err).ToNot(HaveOccurred())

	prettyValues := map[string]string{}
	for name, inputSource := range resolved {
		prettyValues[name] = setup.versionIDs.Name(inputSource.InputVersion.VersionID)
	}

	actualResult := Result{OK: ok, Values: prettyValues}

	Expect(actualResult).To(Equal(example.Result))
}

type setupDB struct {
	teamID     int
	pipelineID int

	jobIDs      StringMapping
	resourceIDs StringMapping
	versionIDs  StringMapping

	psql sq.StatementBuilderType
}

func (s setupDB) insertTeamsPipelines() {
	_, err := s.psql.Insert("teams").
		Columns("id", "name").
		Values(s.teamID, "algorithm").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("pipelines").
		Columns("id", "team_id", "name").
		Values(s.pipelineID, s.teamID, "algorithm").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())
}

func (s setupDB) insertJob(jobName string) int {
	jobID := s.jobIDs.ID(jobName)

	_, err := s.psql.Insert("jobs").
		Columns("id", "pipeline_id", "name", "config").
		Values(jobID, s.pipelineID, jobName, "{}").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	return jobID
}

func (s setupDB) insertResource(name string) int {
	resourceID := s.resourceIDs.ID(name)

	_, err := s.psql.Insert("resource_configs").
		Columns("id", "source_hash").
		Values(resourceID, "bogus-hash").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("resource_config_scopes").
		Columns("id", "resource_config_id").
		Values(resourceID, resourceID).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("resources").
		Columns("id", "name", "config", "pipeline_id", "resource_config_id", "resource_config_scope_id").
		Values(resourceID, name, "{}", s.pipelineID, resourceID, resourceID).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	return resourceID
}

func (s setupDB) insertRowVersion(row DBRow) {
	versionID := s.versionIDs.ID(row.Version)

	resourceID := s.insertResource(row.Resource)

	_, err := s.psql.Insert("resource_config_versions").
		Columns("id", "resource_config_scope_id", "version", "version_md5", "check_order").
		Values(versionID, resourceID, "{}", sq.Expr("md5(?)", row.Version), row.CheckOrder).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	if row.Disabled {
		_, err = s.psql.Insert("resource_disabled_versions").
			Columns("resource_id", "version_md5").
			Values(resourceID, sq.Expr("md5(?)", row.Version)).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())
	}
}

func (s setupDB) insertRowBuild(row DBRow) {
	jobID := s.insertJob(row.Job)

	var existingJobID int
	err := s.psql.Insert("builds").
		Columns("team_id", "id", "job_id", "name", "status", "scheduled").
		Values(s.teamID, row.BuildID, jobID, "some-name", "succeeded", true).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = excluded.name").
		Suffix("RETURNING job_id").
		QueryRow().
		Scan(&existingJobID)
	Expect(err).ToNot(HaveOccurred())

	Expect(existingJobID).To(Equal(jobID), fmt.Sprintf("build ID %d already used by job other than %s", row.BuildID, row.Job))
}

func (s setupDB) insertBuildPipe(row DBRow) {
	_, err := s.psql.Insert("build_pipes").
		Columns("from_build_id", "to_build_id").
		Values(row.FromBuildID, row.ToBuildID).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())
}
