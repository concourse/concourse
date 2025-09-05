package algorithm_test

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/tracing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gocache "github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DB struct {
	BuildInputs      []DBRow
	BuildOutputs     []DBRow
	BuildPipes       []DBRow
	Resources        []DBRow
	NeedsV6Migration bool
}

type DBRow struct {
	Job                   string
	BuildID               int
	Resource              string
	Version               string
	CheckOrder            int
	VersionID             int
	Disabled              bool
	FromBuildID           int
	ToBuildID             int
	RerunOfBuildID        int
	BuildStatus           string
	NoResourceConfigScope bool
	DoNotInsertVersion    bool
}

type Example struct {
	LoadDB     string
	DB         DB
	Inputs     Inputs
	Result     Result
	Iterations int
	Error      error
}

type Inputs []Input

type Input struct {
	Name                  string
	Resource              string
	Passed                []string
	Version               Version
	NoResourceConfigScope bool
}

type Version struct {
	Every  bool
	Latest bool
	Pinned string
}

type Result struct {
	OK               bool
	Values           map[string]string
	PassedBuildIDs   map[string][]int
	Errors           map[string]string
	ExpectedMigrated map[int]map[int][]string
	HasNext          bool
	NoNext           bool
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
	currentTest := ginkgo.CurrentSpecReport()

	ctx, span := tracing.StartSpan(context.Background(), currentTest.LeafNodeText, tracing.Attrs{})
	defer span.End()

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

	pipeline, _, err := team.SavePipeline(atc.PipelineRef{Name: "algorithm"}, atc.Config{}, db.ConfigVersion(0), false)
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

	cache := gocache.New(10*time.Second, 10*time.Second)

	var versionsDB db.VersionsDB
	if example.LoadDB != "" {
		if os.Getenv("ALGORITHM_REGRESSION") == "" {
			ginkgo.Skip("skipping; to run, set $ALGORITHM_REGRESSION")
		}

		versionsDB = example.importVersionsDB(ctx, setup, cache, resources)
	} else {
		versionsDB = example.setupVersionsDB(ctx, setup, cache, resources)
	}

	inputConfigs := make(db.InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := db.JobSet{}
		for _, jobName := range input.Passed {
			setup.insertJob(jobName)
			passed[setup.jobIDs.ID(jobName)] = true
		}

		inputConfigs[i] = db.InputConfig{
			Name:            input.Name,
			Passed:          passed,
			ResourceID:      setup.resourceIDs.ID(input.Resource),
			UseEveryVersion: input.Version.Every,
			JobID:           setup.jobIDs.ID(CurrentJobName),
		}

		if len(input.Version.Pinned) != 0 {
			inputConfigs[i].PinnedVersion = atc.Version{"ver": input.Version.Pinned}

			setup.insertPinned(setup.resourceIDs.ID(input.Resource), atc.Version{"ver": input.Version.Pinned})
		}
	}

	setup.insertJob("current")

	job, found, err := pipeline.Job("current")
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())

	alg := algorithm.New(versionsDB)

	iterations := 1
	if example.Iterations != 0 {
		iterations = example.Iterations
	}

	for i := 0; i < iterations; i++ {
		example.assert(ctx, setup, alg, job, inputConfigs)
		cache.Flush()
	}
}

func (example Example) importVersionsDB(ctx context.Context, setup setupDB, cache *gocache.Cache, resources map[string]atc.ResourceConfig) db.VersionsDB {
	ctx, span := tracing.StartSpan(ctx, "importVersionsDB", tracing.Attrs{
		"db": example.LoadDB,
	})
	defer span.End()

	versionsDB := db.NewVersionsDB(dbConn, 100, cache)

	dbFile, err := os.Open(example.LoadDB)
	Expect(err).ToNot(HaveOccurred())

	gr, err := gzip.NewReader(dbFile)
	Expect(err).ToNot(HaveOccurred())

	var debugDB atc.DebugVersionsDB
	err = func(ctx context.Context) error {
		_, span = tracing.StartSpan(ctx, "Decode", tracing.Attrs{})
		defer span.End()
		return json.NewDecoder(gr).Decode(&debugDB)
	}(ctx)
	Expect(err).ToNot(HaveOccurred())

	span.AddEvent("decoded", trace.WithAttributes(
		attribute.Int("Jobs", len(debugDB.Jobs)),
		attribute.Int("Resources", len(debugDB.Resources)),
		attribute.Int("LegacyJobIDs", len(debugDB.LegacyJobIDs)),
		attribute.Int("LegacyResourceIDs", len(debugDB.LegacyResourceIDs)),
		attribute.Int("ResourceVersions", len(debugDB.ResourceVersions)),
		attribute.Int("BuildInputs", len(debugDB.BuildInputs)),
		attribute.Int("BuildOutputs", len(debugDB.BuildOutputs)),
		attribute.Int("BuildReruns", len(debugDB.BuildReruns)),
	))

	// legacy, pre-6.0
	for name, id := range debugDB.LegacyJobIDs {
		setup.jobIDs[name] = id
		setup.insertJob(name)
	}

	for name, id := range debugDB.LegacyResourceIDs {
		setup.resourceIDs[name] = id

		setup.insertResource(name, &id)
		resources[name] = atc.ResourceConfig{
			Name: name,
			Type: "some-base-type",
			Source: atc.Source{
				name: "source",
			},
		}
	}

	// 6.0+
	for _, job := range debugDB.Jobs {
		setup.jobIDs[job.Name] = job.ID
		setup.insertJob(job.Name)
	}

	for _, resource := range debugDB.Resources {
		setup.resourceIDs[resource.Name] = resource.ID

		setup.insertResource(resource.Name, resource.ScopeID)
		resources[resource.Name] = atc.ResourceConfig{
			Name: resource.Name,
			Type: "some-base-type",
			Source: atc.Source{
				resource.Name: "source",
			},
		}
	}

	err = func(ctx context.Context) error {
		_, span = tracing.StartSpan(ctx, "import versions", tracing.Attrs{})
		defer span.End()

		conn, err := dbConn.Conn(ctx)
		Expect(err).ToNot(HaveOccurred())

		err = conn.Raw(func(driverConn any) error {
			pgxConn := driverConn.(*stdlib.Conn).Conn() // conn is a *pgx.Conn now
			txn, err := pgxConn.Begin(ctx)
			Expect(err).ToNot(HaveOccurred())

			cols := []string{"id", "resource_config_scope_id", "version", "version_md5", "check_order"}
			copyCount, err := txn.CopyFrom(ctx,
				pgx.Identifier{"resource_config_versions"},
				cols, pgx.CopyFromSlice(len(debugDB.ResourceVersions), func(i int) (row []any, err error) {
					resource := debugDB.ResourceVersions[i]
					name := fmt.Sprintf("imported-r%dv%d", resource.ResourceID, resource.VersionID)
					setup.versionIDs[name] = resource.VersionID
					scope := resource.ScopeID
					if scope == 0 {
						// pre-6.0
						scope = resource.ResourceID
					}

					row = []any{
						resource.VersionID,
						scope,
						"{}",
						strconv.Itoa(resource.VersionID),
						resource.CheckOrder,
					}
					return row, nil
				}))
			Expect(err).ToNot(HaveOccurred())
			Expect(int(copyCount)).To(Equal(len(debugDB.ResourceVersions)))

			err = txn.Commit(ctx)
			Expect(err).ToNot(HaveOccurred())

			return nil
		})
		Expect(err).ToNot(HaveOccurred())
		err = conn.Close()
		Expect(err).ToNot(HaveOccurred())

		return nil
	}(ctx)
	Expect(err).ToNot(HaveOccurred())

	err = func(ctx context.Context) error {
		_, span = tracing.StartSpan(ctx, "import builds", tracing.Attrs{})
		defer span.End()

		conn, err := dbConn.Conn(ctx)
		Expect(err).ToNot(HaveOccurred())

		imported := map[int]bool{}

		err = conn.Raw(func(driverConn any) error {
			pgxConn := driverConn.(*stdlib.Conn).Conn() // conn is a *pgx.Conn now
			txn, err := pgxConn.Begin(ctx)
			Expect(err).ToNot(HaveOccurred())

			cols := []string{"team_id", "id", "job_id", "name", "status"}
			var copyCount int64

			var outputs [][]any
			for _, row := range debugDB.BuildOutputs {
				if imported[row.BuildID] {
					continue
				}
				r := []any{
					setup.teamID,
					row.BuildID,
					row.JobID,
					strconv.Itoa(row.BuildID),
					"succeeded",
				}
				outputs = append(outputs, r)
				imported[row.BuildID] = true
			}

			copyCount, err = txn.CopyFrom(ctx,
				pgx.Identifier{"builds"},
				cols, pgx.CopyFromRows(outputs))
			Expect(err).ToNot(HaveOccurred())
			Expect(int(copyCount)).To(Equal(len(outputs)))

			// any builds not created at this point must have failed as they weren't
			// present via outputs
			var inputs [][]any
			for _, row := range debugDB.BuildInputs {
				if imported[row.BuildID] {
					continue
				}
				r := []any{
					setup.teamID,
					row.BuildID,
					row.JobID,
					strconv.Itoa(row.BuildID),
					"failed",
				}
				inputs = append(inputs, r)
				imported[row.BuildID] = true
			}
			copyCount, err = txn.CopyFrom(ctx,
				pgx.Identifier{"builds"},
				cols, pgx.CopyFromRows(inputs))
			Expect(err).ToNot(HaveOccurred())
			Expect(int(copyCount)).To(Equal(len(inputs)))

			// any builds not created at this point must have failed as they weren't
			// present via outputs
			var reruns [][]any
			for _, row := range debugDB.BuildReruns {
				if imported[row.RerunOf] {
					continue
				}
				r := []any{
					setup.teamID,
					row.RerunOf,
					row.JobID,
					"some-name",
					"failed",
				}
				reruns = append(reruns, r)
				imported[row.RerunOf] = true
			}
			copyCount, err = txn.CopyFrom(ctx,
				pgx.Identifier{"builds"},
				cols, pgx.CopyFromRows(reruns))
			Expect(err).ToNot(HaveOccurred())
			Expect(int(copyCount)).To(Equal(len(reruns)))

			err = txn.Commit(ctx)
			Expect(err).ToNot(HaveOccurred())

			return nil
		})
		Expect(err).ToNot(HaveOccurred())
		err = conn.Close()
		Expect(err).ToNot(HaveOccurred())

		for _, row := range debugDB.BuildReruns {
			if !imported[row.BuildID] {
				// probably a build we don't care about
				continue
			}

			_, err = setup.psql.Update("builds").
				Set("rerun_of", row.RerunOf).
				Where(sq.Eq{"id": row.BuildID}).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		return nil
	}(ctx)
	Expect(err).ToNot(HaveOccurred())

	err = func(ctx context.Context) error {
		_, span = tracing.StartSpan(ctx, "import inputs", tracing.Attrs{})
		defer span.End()

		conn, err := dbConn.Conn(ctx)
		Expect(err).ToNot(HaveOccurred())

		err = conn.Raw(func(driverConn any) error {
			pgxConn := driverConn.(*stdlib.Conn).Conn() // conn is a *pgx.Conn now
			txn, err := pgxConn.Begin(ctx)
			Expect(err).ToNot(HaveOccurred())

			cols := []string{"build_id", "resource_id", "version_md5", "name", "first_occurrence"}
			copyCount, err := txn.CopyFrom(ctx,
				pgx.Identifier{"build_resource_config_version_inputs"},
				cols, pgx.CopyFromSlice(len(debugDB.BuildInputs), func(i int) (row []any, err error) {
					r := debugDB.BuildInputs[i]
					row = []any{
						r.BuildID,
						r.ResourceID,
						strconv.Itoa(r.VersionID),
						strconv.Itoa(i),
						false,
					}
					return row, nil
				}))
			Expect(err).ToNot(HaveOccurred())
			Expect(int(copyCount)).To(Equal(len(debugDB.BuildInputs)))

			err = txn.Commit(ctx)
			Expect(err).ToNot(HaveOccurred())

			return nil
		})
		Expect(err).ToNot(HaveOccurred())
		err = conn.Close()
		Expect(err).ToNot(HaveOccurred())

		return nil
	}(ctx)
	Expect(err).ToNot(HaveOccurred())

	err = func(ctx context.Context) error {
		_, span = tracing.StartSpan(ctx, "import outputs", tracing.Attrs{})
		defer span.End()

		conn, err := dbConn.Conn(ctx)
		Expect(err).ToNot(HaveOccurred())

		err = conn.Raw(func(driverConn any) error {
			pgxConn := driverConn.(*stdlib.Conn).Conn() // conn is a *pgx.Conn now
			txn, err := pgxConn.Begin(ctx)
			Expect(err).ToNot(HaveOccurred())

			cols := []string{"build_id", "resource_id", "version_md5", "name"}
			copyCount, err := txn.CopyFrom(ctx,
				pgx.Identifier{"build_resource_config_version_outputs"},
				cols, pgx.CopyFromSlice(len(debugDB.BuildOutputs), func(i int) (row []any, err error) {
					r := debugDB.BuildOutputs[i]
					row = []any{
						r.BuildID,
						r.ResourceID,
						strconv.Itoa(r.VersionID),
						strconv.Itoa(i),
					}
					return row, nil
				}))
			Expect(err).ToNot(HaveOccurred())
			Expect(int(copyCount)).To(Equal(len(debugDB.BuildOutputs)))

			err = txn.Commit(ctx)
			Expect(err).ToNot(HaveOccurred())

			return nil
		})
		Expect(err).ToNot(HaveOccurred())
		err = conn.Close()
		Expect(err).ToNot(HaveOccurred())

		return nil
	}(ctx)
	Expect(err).ToNot(HaveOccurred())

	return versionsDB
}

func (example Example) setupVersionsDB(ctx context.Context, setup setupDB, cache *gocache.Cache, resources map[string]atc.ResourceConfig) db.VersionsDB {
	_, span := tracing.StartSpan(ctx, "setupVersionsDB", tracing.Attrs{})
	defer span.End()

	versionsDB := db.NewVersionsDB(dbConn, 2, cache)

	for _, row := range example.DB.Resources {
		setup.insertRowVersion(resources, row)
	}

	buildOutputs := map[int]map[string][]string{}
	buildToJobID := map[int]int{}
	buildToRerunOf := map[int]int{}
	for _, row := range example.DB.BuildOutputs {
		setup.insertRowVersion(resources, row)
		setup.insertRowBuild(row, example.DB.NeedsV6Migration)

		resourceID := setup.resourceIDs.ID(row.Resource)

		versionJSON, err := json.Marshal(atc.Version{"ver": row.Version})
		Expect(err).ToNot(HaveOccurred())

		_, err = setup.psql.Insert("build_resource_config_version_outputs").
			Columns("build_id", "resource_id", "version_md5", "name").
			Values(row.BuildID, resourceID, sq.Expr("md5(?)", versionJSON), row.Resource).
			Exec()
		Expect(err).ToNot(HaveOccurred())

		if !example.DB.NeedsV6Migration {
			outputs, ok := buildOutputs[row.BuildID]
			if !ok {
				outputs = map[string][]string{}
				buildOutputs[row.BuildID] = outputs
			}

			key := strconv.Itoa(resourceID)

			outputs[key] = append(outputs[key], convertToMD5(row.Version))
			buildToJobID[row.BuildID] = setup.jobIDs.ID(row.Job)

			if row.RerunOfBuildID != 0 {
				buildToRerunOf[row.BuildID] = row.RerunOfBuildID
			}
		}
	}

	for _, row := range example.DB.BuildInputs {
		setup.insertRowVersion(resources, row)
		setup.insertRowBuild(row, example.DB.NeedsV6Migration)

		resourceID := setup.resourceIDs.ID(row.Resource)

		versionJSON, err := json.Marshal(atc.Version{"ver": row.Version})
		Expect(err).ToNot(HaveOccurred())

		_, err = setup.psql.Insert("build_resource_config_version_inputs").
			Columns("build_id", "resource_id", "version_md5", "name", "first_occurrence").
			Values(row.BuildID, resourceID, sq.Expr("md5(?)", versionJSON), row.Resource, false).
			Exec()
		Expect(err).ToNot(HaveOccurred())

		if !example.DB.NeedsV6Migration {
			outputs, ok := buildOutputs[row.BuildID]
			if !ok {
				outputs = map[string][]string{}
				buildOutputs[row.BuildID] = outputs
			}

			key := strconv.Itoa(resourceID)

			outputs[key] = append(outputs[key], convertToMD5(row.Version))
			buildToJobID[row.BuildID] = setup.jobIDs.ID(row.Job)

			if row.RerunOfBuildID != 0 {
				buildToRerunOf[row.BuildID] = row.RerunOfBuildID
			}
		}
	}

	for buildID, outputs := range buildOutputs {
		outputsJSON, err := json.Marshal(outputs)
		Expect(err).ToNot(HaveOccurred())

		var rerunOf sql.NullInt64
		if buildToRerunOf[buildID] != 0 {
			rerunOf.Int64 = int64(buildToRerunOf[buildID])
		}

		_, err = setup.psql.Insert("successful_build_outputs").
			Columns("build_id", "job_id", "rerun_of", "outputs").
			Values(buildID, buildToJobID[buildID], rerunOf, outputsJSON).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())
	}

	for _, row := range example.DB.BuildPipes {
		setup.insertBuildPipe(row)
	}

	for _, input := range example.Inputs {
		resourceID := setup.resourceIDs.ID(input.Resource)

		var scope *int
		if !input.NoResourceConfigScope {
			scope = &resourceID
		}

		setup.insertResource(input.Resource, scope)

		resources[input.Resource] = atc.ResourceConfig{
			Name: input.Resource,
			Type: "some-base-type",
			Source: atc.Source{
				input.Resource: "source",
			},
		}
	}

	return versionsDB
}

func (example Example) assert(
	ctx context.Context,
	setup setupDB,
	alg *algorithm.Algorithm,
	job db.Job,
	inputConfigs db.InputConfigs,
) {
	ctx, span := tracing.StartSpan(ctx, "assert", tracing.Attrs{})
	defer span.End()

	span.SetAttributes(attribute.Int64("seed", ginkgo.GinkgoRandomSeed()))

	resolved, ok, hasNext, resolvedErr := alg.Compute(ctx, job, inputConfigs)
	if example.Error != nil {
		Expect(resolvedErr).To(Equal(example.Error))
	} else {
		Expect(resolvedErr).ToNot(HaveOccurred())

		prettyValues := map[string]string{}
		erroredValues := map[string]string{}
		passedJobs := map[string][]int{}
		for name, inputSource := range resolved {
			if inputSource.ResolveError != "" {
				erroredValues[name] = string(inputSource.ResolveError)
			} else {
				if ok {
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

					passedJobs[name] = inputSource.PassedBuildIDs
				}
			}
		}

		actualResult := Result{OK: ok}
		if len(erroredValues) != 0 {
			actualResult.Errors = erroredValues
		}

		if example.Result.PassedBuildIDs != nil {
			actualResult.PassedBuildIDs = passedJobs
		}

		if ok {
			actualResult.Values = prettyValues
		}

		Expect(actualResult.OK).To(Equal(example.Result.OK))
		Expect(actualResult.Errors).To(Equal(example.Result.Errors))
		Expect(actualResult.Values).To(Equal(example.Result.Values))

		for input, buildIDs := range example.Result.PassedBuildIDs {
			Expect(actualResult.PassedBuildIDs[input]).To(ConsistOf(buildIDs))
		}

		if example.Result.ExpectedMigrated != nil {
			rows, err := setup.psql.Select("build_id", "job_id", "outputs", "rerun_of").
				From("successful_build_outputs").
				Query()
			Expect(err).ToNot(HaveOccurred())

			actualMigrated := map[int]map[int][]string{}
			jobToBuilds := map[int]int{}
			rerunOfBuilds := map[int]int{}
			for rows.Next() {
				var buildID, jobID int
				var rerunOf sql.NullInt64
				var outputs string

				err = rows.Scan(&buildID, &jobID, &outputs, &rerunOf)
				Expect(err).ToNot(HaveOccurred())

				_, exists := actualMigrated[buildID]
				Expect(exists).To(BeFalse())

				buildOutputs := map[int][]string{}
				err = json.Unmarshal([]byte(outputs), &buildOutputs)
				Expect(err).ToNot(HaveOccurred())
				actualMigrated[buildID] = buildOutputs

				jobToBuilds[buildID] = jobID

				if rerunOf.Valid {
					rerunOfBuilds[buildID] = int(rerunOf.Int64)
				}
			}

			Expect(actualMigrated).To(Equal(example.Result.ExpectedMigrated))

			for buildID, jobID := range jobToBuilds {
				var actualJobID int

				err = setup.psql.Select("job_id").
					From("builds").
					Where(sq.Eq{
						"id": buildID,
					}).
					QueryRow().
					Scan(&actualJobID)
				Expect(err).ToNot(HaveOccurred())
				Expect(jobID).To(Equal(actualJobID))
			}

			for buildID, rerunBuildID := range rerunOfBuilds {
				var actualRerunOfBuildID int

				err = setup.psql.Select("rerun_of").
					From("builds").
					Where(sq.Eq{
						"id": buildID,
					}).
					QueryRow().
					Scan(&actualRerunOfBuildID)
				Expect(err).ToNot(HaveOccurred())
				Expect(rerunBuildID).To(Equal(actualRerunOfBuildID))
			}
		}

		if example.Result.HasNext == true {
			Expect(hasNext).To(Equal(true))
		}

		if example.Result.NoNext == true {
			Expect(hasNext).To(Equal(false))
		}
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

func (s setupDB) insertJob(jobName string) int {
	id := s.jobIDs.ID(jobName)
	_, err := s.psql.Insert("jobs").
		Columns("id", "pipeline_id", "name", "config", "active").
		Values(id, s.pipelineID, jobName, "{}", true).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	return id
}

func (s setupDB) insertResource(name string, scope *int) int {
	resourceID := s.resourceIDs.ID(name)

	// just make them one-to-one
	resourceConfigID := resourceID

	j, err := json.Marshal(atc.Source{name: "source"})
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("resource_configs").
		Columns("id", "source_hash", "base_resource_type_id").
		Values(resourceConfigID, fmt.Sprintf("%x", sha256.Sum256(j)), 1).
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	var rcsID sql.NullInt64
	if scope != nil {
		_, err = s.psql.Insert("resource_config_scopes").
			Columns("id", "resource_config_id").
			Values(*scope, resourceConfigID).
			Suffix("ON CONFLICT DO NOTHING").
			Exec()
		Expect(err).ToNot(HaveOccurred())

		rcsID = sql.NullInt64{
			Int64: int64(*scope),
			Valid: true,
		}
	}

	_, err = s.psql.Insert("resources").
		Columns("id", "name", "type", "config", "pipeline_id", "resource_config_id", "resource_config_scope_id").
		Values(resourceID, name, fmt.Sprintf("%s-type", name), "{}", s.pipelineID, resourceID, rcsID).
		Suffix("ON CONFLICT (name, pipeline_id) DO UPDATE SET id = EXCLUDED.id, resource_config_id = EXCLUDED.resource_config_id, resource_config_scope_id = EXCLUDED.resource_config_scope_id").
		Exec()
	Expect(err).ToNot(HaveOccurred())

	return resourceID
}

func (s setupDB) insertRowVersion(resources map[string]atc.ResourceConfig, row DBRow) {
	resourceID := s.resourceIDs.ID(row.Resource)
	versionID := s.versionIDs.ID(row.Version)

	var scope *int
	if !row.NoResourceConfigScope {
		scope = &resourceID
	}

	s.insertResource(row.Resource, scope)
	resources[row.Resource] = atc.ResourceConfig{
		Name: row.Resource,
		Type: "some-base-type",
		Source: atc.Source{
			row.Resource: "source",
		},
	}

	if row.NoResourceConfigScope || row.DoNotInsertVersion {
		// there's no version to insert if it has no scope
		return
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
}

func (s setupDB) insertRowBuild(row DBRow, needsV6Migration bool) {
	jobID := s.insertJob(row.Job)

	var rerunOf sql.NullInt64
	if row.RerunOfBuildID != 0 {
		rerunOf = sql.NullInt64{Int64: int64(row.RerunOfBuildID), Valid: true}
	}

	buildStatus := "succeeded"
	if len(row.BuildStatus) != 0 {
		buildStatus = row.BuildStatus
	}

	var existingJobID int
	err := s.psql.Insert("builds").
		Columns("team_id", "id", "job_id", "name", "status", "scheduled", "inputs_ready", "rerun_of", "needs_v6_migration").
		Values(s.teamID, row.BuildID, jobID, strconv.Itoa(row.BuildID), buildStatus, true, true, rerunOf, needsV6Migration).
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

func (s setupDB) insertPinned(resourceID int, version atc.Version) {
	versionJSON, err := json.Marshal(version)
	Expect(err).ToNot(HaveOccurred())

	_, err = s.psql.Insert("resource_pins").
		Columns("resource_id", "version", "comment_text").
		Values(resourceID, versionJSON, "").
		Suffix("ON CONFLICT DO NOTHING").
		Exec()
	Expect(err).ToNot(HaveOccurred())
}
