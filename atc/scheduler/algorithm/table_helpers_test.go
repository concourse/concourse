package algorithm_test

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/lib/pq"
	. "github.com/onsi/gomega"
	gocache "github.com/patrickmn/go-cache"
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
	Pinned      bool
}

type Example struct {
	LoadDB string
	DB     DB
	Inputs Inputs
	Result Result
	Error  error
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
	OK      bool
	Values  map[string]string
	Errors  map[string]string
	Skipped map[string]bool
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

type LegacyVersionsDB struct {
	ResourceVersions []LegacyResourceVersion
	BuildOutputs     []LegacyBuildOutput
	BuildInputs      []LegacyBuildInput
	JobIDs           map[string]int
	ResourceIDs      map[string]int
}

type LegacyResourceVersion struct {
	VersionID  int
	ResourceID int
	CheckOrder int
	Disabled   bool
}

type LegacyBuildOutput struct {
	LegacyResourceVersion
	BuildID int
	JobID   int
}

type LegacyBuildInput struct {
	LegacyResourceVersion
	BuildID         int
	JobID           int
	InputName       string
	FirstOccurrence bool
}

const CurrentJobName = "current"

func (example Example) Run() {
	schedulerCache := gocache.New(10*time.Second, 10*time.Second)

	versionsDB := &db.VersionsDB{
		Conn:             dbConn,
		Cache:            schedulerCache,
		DisabledVersions: map[int]map[string]bool{},
	}

	setup := setupDB{
		teamID:      1,
		pipelineID:  1,
		psql:        sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(dbConn),
		jobIDs:      StringMapping{},
		resourceIDs: StringMapping{},
		versionIDs:  StringMapping{},
	}

	team, err := teamFactory.CreateTeam(atc.Team{Name: "algorithm"})
	Expect(err).NotTo(HaveOccurred())

	pipeline, _, err := team.SavePipeline("algorithm", atc.Config{}, db.ConfigVersion(0), db.PipelineUnpaused)
	Expect(err).NotTo(HaveOccurred())

	setupTx, err := dbConn.Begin()
	Expect(err).ToNot(HaveOccurred())

	brt := db.BaseResourceType{
		Name: "some-base-type",
	}

	_, err = brt.FindOrCreate(setupTx, false)
	Expect(err).NotTo(HaveOccurred())
	Expect(setupTx.Commit()).To(Succeed())

	resources := map[string]atc.ResourceConfig{}

	if example.LoadDB != "" {
		dbFile, err := os.Open(example.LoadDB)
		Expect(err).ToNot(HaveOccurred())

		gr, err := gzip.NewReader(dbFile)
		Expect(err).ToNot(HaveOccurred())

		log.Println("LOADING DB", example.LoadDB)
		var legacyDB LegacyVersionsDB
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
			resources[name] = atc.ResourceConfig{
				Name: name,
				Type: "some-base-type",
				Source: atc.Source{
					name: "source",
				},
			}
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

		log.Println("IMPORTING INPUTS TO SUCCESSFUL BUILD VERSIONS")

		inputs := map[int]map[string][]string{}
		buildToJobID := map[int]int{}
		for _, row := range legacyDB.BuildInputs {
			outputs, ok := inputs[row.BuildID]
			if !ok {
				outputs = map[string][]string{}
				inputs[row.BuildID] = outputs
			}

			key := strconv.Itoa(row.ResourceID)

			outputs[key] = append(outputs[key], strconv.Itoa(row.VersionID))
			buildToJobID[row.BuildID] = row.JobID
		}

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err = tx.Prepare(pq.CopyIn("successful_build_outputs", "build_id", "job_id", "outputs"))
		Expect(err).ToNot(HaveOccurred())

		for buildID, outputs := range inputs {
			outputsJSON, err := json.Marshal(outputs)
			Expect(err).ToNot(HaveOccurred())

			_, err = stmt.Exec(buildID, buildToJobID[buildID], outputsJSON)
			Expect(err).ToNot(HaveOccurred())
		}

		_, err = stmt.Exec()
		Expect(err).ToNot(HaveOccurred())

		err = stmt.Close()
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		log.Println("IMPORTING OUTPUTS TO SUCCESSFUL BUILD VERSIONS")

		buildOutputs := map[int]map[string][]string{}
		for _, row := range legacyDB.BuildOutputs {
			out, ok := inputs[row.BuildID]
			if !ok {
				out = map[string][]string{}
				buildOutputs[row.BuildID] = out
			}

			key := strconv.Itoa(row.ResourceID)

			out[key] = append(out[key], strconv.Itoa(row.VersionID))
			buildToJobID[row.BuildID] = row.JobID
		}

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		stmt, err = tx.Prepare(pq.CopyIn("successful_build_outputs", "build_id", "job_id", "outputs"))
		Expect(err).ToNot(HaveOccurred())

		for buildID, outputs := range buildOutputs {
			outputsJSON, err := json.Marshal(outputs)
			Expect(err).ToNot(HaveOccurred())

			_, err = stmt.Exec(buildID, buildToJobID[buildID], outputsJSON)
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
			setup.insertRowVersion(resources, row)
		}

		buildOutputs := map[int]map[string][]string{}
		buildToJobID := map[int]int{}
		for _, row := range example.DB.BuildOutputs {
			setup.insertRowVersion(resources, row)
			setup.insertRowBuild(row)

			resourceID := setup.resourceIDs.ID(row.Resource)

			versionJSON, err := json.Marshal(atc.Version{"ver": row.Version})
			Expect(err).ToNot(HaveOccurred())

			_, err = setup.psql.Insert("build_resource_config_version_outputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(row.BuildID, resourceID, sq.Expr("md5(?)", versionJSON), row.Resource).
				Exec()
			Expect(err).ToNot(HaveOccurred())

			outputs, ok := buildOutputs[row.BuildID]
			if !ok {
				outputs = map[string][]string{}
				buildOutputs[row.BuildID] = outputs
			}

			key := strconv.Itoa(resourceID)

			outputs[key] = append(outputs[key], convertToMD5(row.Version))
			buildToJobID[row.BuildID] = setup.jobIDs.ID(row.Job)
		}

		for _, row := range example.DB.BuildInputs {
			setup.insertRowVersion(resources, row)
			setup.insertRowBuild(row)

			resourceID := setup.resourceIDs.ID(row.Resource)

			versionJSON, err := json.Marshal(atc.Version{"ver": row.Version})
			Expect(err).ToNot(HaveOccurred())

			_, err = setup.psql.Insert("build_resource_config_version_inputs").
				Columns("build_id", "resource_id", "version_md5", "name", "first_occurrence").
				Values(row.BuildID, resourceID, sq.Expr("md5(?)", versionJSON), row.Resource, false).
				Exec()
			Expect(err).ToNot(HaveOccurred())

			outputs, ok := buildOutputs[row.BuildID]
			if !ok {
				outputs = map[string][]string{}
				buildOutputs[row.BuildID] = outputs
			}

			key := strconv.Itoa(resourceID)

			outputs[key] = append(outputs[key], convertToMD5(row.Version))
			buildToJobID[row.BuildID] = setup.jobIDs.ID(row.Job)
		}

		for buildID, outputs := range buildOutputs {
			outputsJSON, err := json.Marshal(outputs)
			Expect(err).ToNot(HaveOccurred())

			_, err = setup.psql.Insert("successful_build_outputs").
				Columns("build_id", "job_id", "outputs").
				Values(buildID, buildToJobID[buildID], outputsJSON).
				Suffix("ON CONFLICT DO NOTHING").
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		for _, row := range example.DB.BuildPipes {
			setup.insertBuildPipe(row)
		}
	}

	for _, input := range example.Inputs {
		setup.insertResource(input.Resource)

		resources[input.Resource] = atc.ResourceConfig{
			Name: input.Resource,
			Type: "some-base-type",
			Source: atc.Source{
				input.Resource: "source",
			},
		}
	}

	inputConfigs := make(algorithm.InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := db.JobSet{}
		for _, jobName := range input.Passed {
			setup.insertJob(jobName)
			passed[setup.jobIDs.ID(jobName)] = true
		}

		inputConfigs[i] = algorithm.InputConfig{
			Name:            input.Name,
			Passed:          passed,
			ResourceID:      setup.resourceIDs.ID(input.Resource),
			UseEveryVersion: input.Version.Every,
			JobID:           setup.jobIDs.ID(CurrentJobName),
		}

		if len(input.Version.Pinned) != 0 {
			versionJSON, err := json.Marshal(atc.Version{"ver": input.Version.Pinned})
			Expect(err).ToNot(HaveOccurred())

			var pinnedVersion db.ResourceVersion
			rows, err := setup.psql.Select("rcv.version_md5").
				From("resource_config_versions rcv").
				Join("resources r ON r.resource_config_scope_id = rcv.resource_config_scope_id").
				Where(sq.Eq{
					"rcv.version": versionJSON,
					"r.id":        inputConfigs[i].ResourceID,
				}).
				Query()
			Expect(err).ToNot(HaveOccurred())

			if rows.Next() {
				err = rows.Scan(&pinnedVersion)
				Expect(err).ToNot(HaveOccurred())
			}

			rows.Close()

			if pinnedVersion != "" {
				inputConfigs[i].PinnedVersion = pinnedVersion
			} else {
				// Pinning to an non existant version id
				inputConfigs[i].PinnedVersion = "non-existant-version"
			}
		}
	}

	inputs := atc.PlanSequence{}
	for i, input := range inputConfigs {
		var version *atc.VersionConfig
		if input.UseEveryVersion {
			version = &atc.VersionConfig{Every: true}
		} else if input.PinnedVersion != "" {
			version = &atc.VersionConfig{Pinned: atc.Version{"ver": example.Inputs[i].Version.Pinned}}
		} else {
			version = &atc.VersionConfig{Latest: true}
		}

		passed := []string{}
		for job, _ := range input.Passed {
			passed = append(passed, setup.jobIDs.Name(job))
		}

		inputs = append(inputs, atc.PlanConfig{
			Get:      input.Name,
			Resource: setup.resourceIDs.Name(input.ResourceID),
			Passed:   passed,
			Version:  version,
		})
	}

	rows, err := setup.psql.Select("rdv.resource_id, rdv.version_md5").
		From("resource_disabled_versions rdv").
		Join("resources r ON r.id = rdv.resource_id").
		Where(sq.Eq{
			"r.pipeline_id": 1,
		}).
		Query()
	Expect(err).ToNot(HaveOccurred())

	for rows.Next() {
		var versionMD5 string
		var resourceID int

		err = rows.Scan(&resourceID, &versionMD5)
		Expect(err).ToNot(HaveOccurred())

		md5s, found := versionsDB.DisabledVersions[resourceID]
		if !found {
			md5s = map[string]bool{}
			versionsDB.DisabledVersions[resourceID] = md5s
		}

		md5s[versionMD5] = true
	}

	resourceConfigs := atc.ResourceConfigs{}
	for _, resource := range resources {
		resourceConfigs = append(resourceConfigs, resource)
	}

	jobs := atc.JobConfigs{}
	for jobName, _ := range setup.jobIDs {
		jobs = append(jobs, atc.JobConfig{
			Name: jobName,
			Plan: inputs,
		})
	}

	setup.insertJob("current")

	pipeline, _, err = team.SavePipeline("algorithm", atc.Config{
		Jobs:      jobs,
		Resources: resourceConfigs,
	}, db.ConfigVersion(1), db.PipelineUnpaused)
	Expect(err).NotTo(HaveOccurred())

	dbResources := db.Resources{}
	for name, _ := range setup.resourceIDs {
		resource, found, err := pipeline.Resource(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		dbResources = append(dbResources, resource)
	}

	job, found, err := pipeline.Job("current")
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())

	versionsDB.JobIDs = setup.jobIDs
	versionsDB.ResourceIDs = setup.resourceIDs

	inputMapper := algorithm.NewInputMapper()
	resolved, ok, err := inputMapper.MapInputs(versionsDB, job, dbResources)
	if example.Error != nil {
		Expect(err).To(Equal(example.Error))
	} else {
		Expect(err).ToNot(HaveOccurred())

		prettyValues := map[string]string{}
		erroredValues := map[string]string{}
		skippedValues := map[string]bool{}
		for name, inputSource := range resolved {
			if inputSource.ResolveSkipped == true {
				skippedValues[name] = true
			} else if inputSource.ResolveError != nil {
				erroredValues[name] = inputSource.ResolveError.Error()
			} else {
				var versionID int
				err := setup.psql.Select("v.id").
					From("resource_config_versions v").
					Join("resources r ON r.resource_config_scope_id = v.resource_config_scope_id").
					Where(sq.Eq{
						"v.version_md5": inputSource.Input.AlgorithmVersion.Version,
						"r.id":          inputSource.Input.ResourceID,
					}).
					QueryRow().
					Scan(&versionID)
				Expect(err).ToNot(HaveOccurred())

				prettyValues[name] = setup.versionIDs.Name(versionID)
			}
		}

		actualResult := Result{OK: ok}
		if len(erroredValues) != 0 {
			actualResult.Errors = erroredValues
		}

		if len(skippedValues) != 0 {
			actualResult.Skipped = skippedValues
		}

		actualResult.Values = prettyValues

		Expect(actualResult).To(Equal(example.Result))
	}
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
	id := s.jobIDs.ID(jobName)
	_, err := s.psql.Insert("jobs").
		Columns("id", "pipeline_id", "name", "config").
		Values(id, s.pipelineID, jobName, "{}").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	return id
}

func (s setupDB) insertResource(name string) int {
	resourceID := s.resourceIDs.ID(name)

	j, err := json.Marshal(atc.Source{name: "source"})
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("resource_configs").
		Columns("id", "source_hash", "base_resource_type_id").
		Values(resourceID, fmt.Sprintf("%x", sha256.Sum256(j)), 1).
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

func (s setupDB) insertRowVersion(resources map[string]atc.ResourceConfig, row DBRow) {
	versionID := s.versionIDs.ID(row.Version)

	resourceID := s.insertResource(row.Resource)
	resources[row.Resource] = atc.ResourceConfig{
		Name: row.Resource,
		Type: "some-base-type",
		Source: atc.Source{
			row.Resource: "source",
		},
	}

	versionJSON, err := json.Marshal(atc.Version{"ver": row.Version})
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("resource_config_versions").
		Columns("id", "resource_config_scope_id", "version", "version_md5", "check_order").
		Values(versionID, resourceID, versionJSON, sq.Expr("md5(?)", versionJSON), row.CheckOrder).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	if row.Disabled {
		_, err = s.psql.Insert("resource_disabled_versions").
			Columns("resource_id", "version_md5").
			Values(resourceID, sq.Expr("md5(?)", versionJSON)).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())
	}

	if row.Pinned {
		_, err = s.psql.Insert("resource_pins").
			Columns("resource_id", "version", "comment_text").
			Values(resourceID, versionJSON, "").
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

	_, err = s.psql.Update("jobs").
		Set("latest_completed_build_id", row.BuildID).
		Where(sq.Eq{
			"id": jobID,
		}).
		Exec()
	Expect(err).ToNot(HaveOccurred())
}

func (s setupDB) insertBuildPipe(row DBRow) {
	_, err := s.psql.Insert("build_pipes").
		Columns("from_build_id", "to_build_id").
		Values(row.FromBuildID, row.ToBuildID).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())
}
