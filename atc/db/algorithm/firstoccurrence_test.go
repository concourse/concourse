package algorithm_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/algorithm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		versionsDB   *algorithm.VersionsDB
		inputConfigs algorithm.InputConfigs
		inputMapping algorithm.InputMapping
		buildInputs  []buildInput
		buildOutputs []buildOutput
	)

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
		setup.insertTeamsPipelines()

		// insert two jobs
		setup.insertJob("j1")
		setup.insertJob("j2")

		// insert resource and two resource versions
		setup.insertRowVersion(DBRow{
			Resource:   "r1",
			Version:    "v1",
			CheckOrder: 1,
			Disabled:   false,
		})
		setup.insertRowVersion(DBRow{
			Resource:   "r1",
			Version:    "v2",
			CheckOrder: 2,
			Disabled:   false,
		})

		// Set up build inputs
		for _, buildInput := range buildInputs {
			setup.insertRowBuild(DBRow{
				Job:     buildInput.JobName,
				BuildID: buildInput.BuildID,
			})

			setup.insertRowVersion(DBRow{
				Resource:   buildInput.ResourceName,
				Version:    buildInput.Version,
				CheckOrder: buildInput.CheckOrder,
				Disabled:   false,
			})

			resourceID := setup.resourceIDs.ID(buildInput.ResourceName)
			_, err := setup.psql.Insert("build_resource_config_version_inputs").
				Columns("build_id", "resource_id", "version_md5", "name", "first_occurrence").
				Values(buildInput.BuildID, resourceID, sq.Expr("md5(?)", buildInput.Version), buildInput.InputName, false).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		// Set up build outputs
		for _, buildOutput := range buildOutputs {
			setup.insertRowBuild(DBRow{
				Job:     buildOutput.JobName,
				BuildID: buildOutput.BuildID,
			})

			setup.insertRowVersion(DBRow{
				Resource:   buildOutput.ResourceName,
				Version:    buildOutput.Version,
				CheckOrder: buildOutput.CheckOrder,
				Disabled:   false,
			})

			resourceID := setup.resourceIDs.ID(buildOutput.ResourceName)
			_, err := setup.psql.Insert("build_resource_config_version_outputs").
				Columns("build_id", "resource_id", "version_md5", "name").
				Values(buildOutput.BuildID, resourceID, sq.Expr("md5(?)", buildOutput.Version), buildOutput.ResourceName).
				Exec()
			Expect(err).ToNot(HaveOccurred())
		}

		versionsDB = &algorithm.VersionsDB{
			Runner:      dbConn,
			JobIDs:      setup.jobIDs,
			ResourceIDs: setup.resourceIDs,
		}

		inputConfigs = algorithm.InputConfigs{
			{
				Name:       "some-input",
				JobName:    "j1",
				Passed:     algorithm.JobSet{},
				ResourceID: 1,
				JobID:      1,
			},
		}

		var ok bool
		var err error
		inputMapping, ok, err = inputConfigs.Resolve(versionsDB)
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
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputSource{
					InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 1, FirstOccurrence: false},
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
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputSource{
					InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 1, FirstOccurrence: true},
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
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputSource{
					InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 1, FirstOccurrence: true},
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
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputSource{
					InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 1, FirstOccurrence: true},
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
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputSource{
					InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 1, FirstOccurrence: true},
					PassedBuildIDs: []int{},
				},
			}))
		})
	})
})
