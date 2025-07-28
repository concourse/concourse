package algorithm_test

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gocache "github.com/patrickmn/go-cache"
)

var _ = Describe("Resolve", func() {
	type buildInput struct {
		BuildID      int
		JobName      string
		InputName    string
		Version      string
		ResourceName string
		CheckOrder   int
		RerunBuildID int
	}

	type buildOutput struct {
		BuildID      int
		JobName      string
		Version      string
		ResourceName string
		CheckOrder   int
	}

	var (
		inputMapping db.InputMapping
		buildInputs  []buildInput
		buildOutputs []buildOutput
	)

	BeforeEach(func() {
		buildInputs = []buildInput{}
		buildOutputs = []buildOutput{}
	})

	JustBeforeEach(func() {
		setup := setupDB{
			teamID:      1,
			pipelineID:  1,
			psql:        sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(dbConn),
			jobIDs:      StringMapping{},
			resourceIDs: StringMapping{},
			versionIDs:  StringMapping{},
		}

		// setup team 1 and pipeline 1
		team, err := teamFactory.CreateTeam(atc.Team{Name: "algorithm"})
		Expect(err).NotTo(HaveOccurred())

		pipeline, _, err := team.SavePipeline(atc.PipelineRef{Name: "algorithm"}, atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name: "r1",
					Type: "r1-type",
				},
			},
			Jobs: atc.JobConfigs{
				{
					Name: "j1",
					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "r1",
							},
						},
					},
				},
			},
		}, db.ConfigVersion(0), false)
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

		// insert two jobs
		setup.insertJob("j1")
		setup.insertJob("j2")

		// insert resource and two resource versions
		setup.insertRowVersion(resources, DBRow{
			Resource:   "r1",
			Version:    "v1",
			CheckOrder: 1,
			Disabled:   false,
		})
		setup.insertRowVersion(resources, DBRow{
			Resource:   "r1",
			Version:    "v2",
			CheckOrder: 2,
			Disabled:   false,
		})

		succeessfulBuildOutputs := map[int]map[string][]string{}
		buildToJobID := map[int]int{}
		buildToRerunOf := map[int]int{}
		// Set up build outputs
		for _, buildOutput := range buildOutputs {
			setup.insertRowBuild(DBRow{
				Job:     buildOutput.JobName,
				BuildID: buildOutput.BuildID,
			}, false)

			setup.insertRowVersion(resources, DBRow{
				Resource:   buildOutput.ResourceName,
				Version:    buildOutput.Version,
				CheckOrder: buildOutput.CheckOrder,
				Disabled:   false,
			})

			versionJSON, err := json.Marshal(atc.Version{"ver": buildOutput.Version})
			Expect(err).ToNot(HaveOccurred())

			resourceID := setup.resourceIDs.ID(buildOutput.ResourceName)
			_, err = setup.psql.Insert("build_resource_config_version_outputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(buildOutput.BuildID, resourceID, sq.Expr("md5(?)", versionJSON), buildOutput.ResourceName).
				Exec()
			Expect(err).ToNot(HaveOccurred())

			outputs, ok := succeessfulBuildOutputs[buildOutput.BuildID]
			if !ok {
				outputs = map[string][]string{}
				succeessfulBuildOutputs[buildOutput.BuildID] = outputs
			}

			key := strconv.Itoa(resourceID)

			outputs[key] = append(outputs[key], convertToMD5(buildOutput.Version))
			buildToJobID[buildOutput.BuildID] = setup.jobIDs.ID(buildOutput.JobName)
		}

		// Set up build inputs
		for _, buildInput := range buildInputs {
			setup.insertRowBuild(DBRow{
				Job:            buildInput.JobName,
				BuildID:        buildInput.BuildID,
				RerunOfBuildID: buildInput.RerunBuildID,
			}, false)

			setup.insertRowVersion(resources, DBRow{
				Resource:   buildInput.ResourceName,
				Version:    buildInput.Version,
				CheckOrder: buildInput.CheckOrder,
				Disabled:   false,
			})

			versionJSON, err := json.Marshal(atc.Version{"ver": buildInput.Version})
			Expect(err).ToNot(HaveOccurred())

			resourceID := setup.resourceIDs.ID(buildInput.ResourceName)
			_, err = setup.psql.Insert("build_resource_config_version_inputs").
				Columns("build_id", "resource_id", "version_md5", "name", "first_occurrence").
				Values(buildInput.BuildID, resourceID, sq.Expr("md5(?)", versionJSON), buildInput.InputName, false).
				Exec()
			Expect(err).ToNot(HaveOccurred())

			outputs, ok := succeessfulBuildOutputs[buildInput.BuildID]
			if !ok {
				outputs = map[string][]string{}
				succeessfulBuildOutputs[buildInput.BuildID] = outputs
			}

			key := strconv.Itoa(resourceID)

			outputs[key] = append(outputs[key], convertToMD5(buildInput.Version))
			buildToJobID[buildInput.BuildID] = setup.jobIDs.ID(buildInput.JobName)

			if buildInput.RerunBuildID != 0 {
				buildToRerunOf[buildInput.BuildID] = buildInput.RerunBuildID
			}
		}

		for buildID, outputs := range succeessfulBuildOutputs {
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

		versionsDB := db.NewVersionsDB(dbConn, 2, gocache.New(10*time.Second, 10*time.Second))

		job, found, err := pipeline.Job("j1")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		r1, found, err := pipeline.Resource("r1")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		jobInputs := db.InputConfigs{
			{
				Name:       "some-input",
				ResourceID: r1.ID(),
				JobID:      job.ID(),
			},
		}

		algorithm := algorithm.New(versionsDB)

		var ok bool
		inputMapping, ok, _, err = algorithm.Compute(context.Background(), job, jobInputs)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	// All these contexts are under the assumption that the algorithm is being
	// run for job "j1" that has a resource called "r1". In the database, there
	// are two jobs ("j1" and j2"), one resource ("r1") and two versions ("v1",
	// "v2") for that resource.

	// The build inputs that is being set in the contexts are the build inputs
	// that the algorithm will see and use to determine whether the computed set
	// of inputs for this job ("r1" with version "v2" because v2 is the latest
	// version) will be first occurrence.

	Context("when the version computed from the algorithm was the same version and resource as an existing input with the same job and the same name", func() {
		BeforeEach(func() {
			buildInputs = []buildInput{
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      31,
					JobName:      "j1",
					InputName:    "some-input",
				},
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      31,
					JobName:      "j1",
					InputName:    "some-other-input",
				},
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      32,
					JobName:      "j2",
					InputName:    "some-input",
				},
			}
		})

		It("sets FirstOccurrence to false", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v2")),
							ResourceID: 1,
						},
						FirstOccurrence: false,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})

	Context("when the version computed from the algorithm was the same version and resource as an existing input with the same job but a different name", func() {
		BeforeEach(func() {
			buildInputs = []buildInput{
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      31,
					JobName:      "j1",
					InputName:    "some-other-input",
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v2")),
							ResourceID: 1},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})

	Context("when the version computed from the algorithm was the same version and resource as existing input but with a different job with the same name", func() {
		BeforeEach(func() {
			buildInputs = []buildInput{
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      32,
					JobName:      "j2",
					InputName:    "some-input",
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v2")),
							ResourceID: 1},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})

	Context("when the version computed from the algorithm was a different version was an existing input of the same job with the same name", func() {
		BeforeEach(func() {
			buildInputs = []buildInput{
				{
					Version:      "v1",
					ResourceName: "r1",
					CheckOrder:   1,
					BuildID:      31,
					JobName:      "j1",
					InputName:    "some-input",
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v2")),
							ResourceID: 1},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})

	Context("when the version computed from the algorithm was not the same version as an existing output of the same job", func() {
		BeforeEach(func() {
			buildOutputs = []buildOutput{
				{
					Version:      "v1",
					ResourceName: "r1",
					CheckOrder:   1,
					BuildID:      31,
					JobName:      "j1",
				},
			}
		})

		It("sets FirstOccurrence to false", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v2")),
							ResourceID: 1},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})

	Context("when a version computed is equal to an existing input from a rerun build", func() {
		BeforeEach(func() {
			buildInputs = []buildInput{
				{
					Version:      "v1",
					ResourceName: "r1",
					CheckOrder:   1,
					BuildID:      30,
					JobName:      "j1",
					InputName:    "some-input",
				},
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   1,
					BuildID:      31,
					JobName:      "j1",
					InputName:    "some-input",
				},
				{
					Version:      "v1",
					ResourceName: "r1",
					CheckOrder:   1,
					BuildID:      32,
					JobName:      "j1",
					InputName:    "some-input",
					RerunBuildID: 30,
				},
			}
		})

		It("sets FirstOccurrence to false", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v2")),
							ResourceID: 1},
						FirstOccurrence: false,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})

	Context("when the version computed from the algorithm was the same version and resource of an old build", func() {
		BeforeEach(func() {
			buildInputs = []buildInput{
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      30,
					JobName:      "j1",
					InputName:    "some-input",
				},
				{
					Version:      "v3",
					ResourceName: "r1",
					CheckOrder:   3,
					BuildID:      31,
					JobName:      "j1",
					InputName:    "some-input",
				},
				{
					Version:      "v2",
					ResourceName: "r1",
					CheckOrder:   2,
					BuildID:      32,
					JobName:      "j1",
					InputName:    "some-input",
				},
			}
		})

		It("sets FirstOccurrence to false", func() {
			Expect(inputMapping).To(Equal(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5("v3")),
							ResourceID: 1,
						},
						FirstOccurrence: false,
					},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})
})

func convertToMD5(version string) string {
	versionJSON, err := json.Marshal(atc.Version{"ver": version})
	Expect(err).ToNot(HaveOccurred())

	hasher := md5.New()
	hasher.Write([]byte(versionJSON))
	return hex.EncodeToString(hasher.Sum(nil))
}
