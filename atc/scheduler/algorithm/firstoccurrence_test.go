package algorithm_test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	. "github.com/onsi/ginkgo"
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

		pipeline, _, err := team.SavePipeline("algorithm", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "j1",
					Plan: atc.PlanSequence{
						{
							Get:      "some-input",
							Resource: "r1",
						},
					},
				},
			},
		}, db.ConfigVersion(0), db.PipelineUnpaused)
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
		// Set up build outputs
		for _, buildOutput := range buildOutputs {
			setup.insertRowBuild(DBRow{
				Job:     buildOutput.JobName,
				BuildID: buildOutput.BuildID,
			})

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
				Job:     buildInput.JobName,
				BuildID: buildInput.BuildID,
			})

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
		}

		for buildID, outputs := range succeessfulBuildOutputs {
			outputsJSON, err := json.Marshal(outputs)
			Expect(err).ToNot(HaveOccurred())

			_, err = setup.psql.Insert("successful_build_outputs").
				Columns("build_id", "job_id", "outputs").
				Values(buildID, buildToJobID[buildID], outputsJSON).
				Suffix("ON CONFLICT DO NOTHING").
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		schedulerCache := gocache.New(10*time.Second, 10*time.Second)
		versionsDB := &db.VersionsDB{
			Conn:        dbConn,
			Cache:       schedulerCache,
			JobIDs:      setup.jobIDs,
			ResourceIDs: setup.resourceIDs,
		}

		resourceConfigs := atc.ResourceConfigs{}
		for _, resource := range resources {
			resourceConfigs = append(resourceConfigs, resource)
		}

		pipeline, _, err = team.SavePipeline("algorithm", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "j1",
					Plan: atc.PlanSequence{
						{
							Get:      "some-input",
							Resource: "r1",
						},
					},
				},
			},
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

		job, found, err := pipeline.Job("j1")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		versionsDB.JobIDs = setup.jobIDs
		versionsDB.ResourceIDs = setup.resourceIDs

		algorithm := algorithm.New()

		var ok bool
		inputMapping, ok, err = algorithm.Compute(versionsDB, job, dbResources)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	Context("when the version was an input of the same job with the same name", func() {
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

	Context("when the version was an input of the same job with a different name", func() {
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

	Context("when the version was an input of a different job with the same name", func() {
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

	Context("when a different version was an input of the same job with the same name", func() {
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

	Context("when a different version was an output of the same job", func() {
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
})

func convertToMD5(version string) string {
	versionJSON, err := json.Marshal(atc.Version{"ver": version})
	Expect(err).ToNot(HaveOccurred())

	hasher := md5.New()
	hasher.Write([]byte(versionJSON))
	return hex.EncodeToString(hasher.Sum(nil))
}
