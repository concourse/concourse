package algorithm_test

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("Resolve", func() {
// 	var (
// 		versionsDB   *algorithm.VersionsDB
// 		inputConfigs algorithm.InputConfigs
// 		inputMapping algorithm.InputMapping
// 	)

// 	BeforeEach(func() {
// 		jobID := insertJob(row.Job)

// 		var existingJobID int
// 		err := psql.Insert("builds").
// 			Columns("team_id", "id", "job_id", "name", "status").
// 			Values(teamID, row.BuildID, jobID, "some-name", "succeeded").
// 			Suffix("ON CONFLICT (id) DO UPDATE SET name = excluded.name").
// 			Suffix("RETURNING job_id").
// 			QueryRow().
// 			Scan(&existingJobID)
// 		Expect(err).ToNot(HaveOccurred())

// 		Expect(existingJobID).To(Equal(jobID), fmt.Sprintf("build ID %d already used by job other than %s", row.BuildID, row.Job))
// 	}

// 		resourceID := resourceIDs.ID(name)

// 		_, err := psql.Insert("resource_configs").
// 			Columns("id", "source_hash").
// 			Values(resourceID, "bogus-hash").
// 			Suffix("ON CONFLICT DO NOTHING").
// 			Exec()
// 		Expect(err).ToNot(HaveOccurred())

// 		_, err = psql.Insert("resource_config_scopes").
// 			Columns("id", "resource_config_id").
// 			Values(resourceID, resourceID).
// 			Suffix("ON CONFLICT DO NOTHING").
// 			Exec()
// 		Expect(err).ToNot(HaveOccurred())

// 		_, err = psql.Insert("resources").
// 			Columns("id", "name", "config", "pipeline_id", "resource_config_id", "resource_config_scope_id").
// 			Values(resourceID, name, "{}", pipelineID, resourceID, resourceID).
// 			Suffix("ON CONFLICT DO NOTHING").
// 			Exec()
// 		Expect(err).ToNot(HaveOccurred())

// 		versionID := versionIDs.ID(row.Version)

// 		resourceID := insertResource(row.Resource)

// 		_, err = psql.Insert("resource_config_versions").
// 			Columns("id", "resource_config_scope_id", "version", "version_md5", "check_order").
// 			Values(versionID, resourceID, "{}", sq.Expr("md5(?)", row.Version), row.CheckOrder).
// 			Suffix("ON CONFLICT DO NOTHING").
// 			Exec()
// 		Expect(err).ToNot(HaveOccurred())

// 		if row.Disabled {
// 			_, err = psql.Insert("resource_disabled_versions").
// 				Columns("resource_id", "version_md5").
// 				Values(resourceID, sq.Expr("md5(?)", row.Version)).
// 				Suffix("ON CONFLICT DO NOTHING").
// 				Exec()
// 			Expect(err).ToNot(HaveOccurred())
// 		}

// 		versionsDB = &algorithm.VersionsDB{
// 			ResourceVersions: []algorithm.ResourceVersion{
// 				{VersionID: 1, ResourceID: 21, CheckOrder: 1},
// 				{VersionID: 2, ResourceID: 21, CheckOrder: 2},
// 			},
// 			BuildOutputs: []algorithm.BuildOutput{},
// 			BuildInputs:  []algorithm.BuildInput{},
// 			JobIDs:       map[string]int{"j1": 11, "j2": 12},
// 			ResourceIDs:  map[string]int{"r1": 21},
// 		}

// 		inputConfigs = algorithm.InputConfigs{
// 			{
// 				Name:       "some-input",
// 				JobName:    "j1",
// 				Passed:     algorithm.JobSet{},
// 				ResourceID: 21,
// 				JobID:      11,
// 			},
// 		}
// 	})

// 	JustBeforeEach(func() {
// 		var ok bool
// 		inputMapping, ok = inputConfigs.Resolve(versionsDB)
// 		Expect(ok).To(BeTrue())
// 	})

// 	Context("when the version was an input of the same job with the same name", func() {
// 		BeforeEach(func() {
// 			versionsDB.BuildInputs = []algorithm.BuildInput{
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
// 					BuildID:         31,
// 					JobID:           11,
// 					InputName:       "some-input",
// 				},
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
// 					BuildID:         31,
// 					JobID:           11,
// 					InputName:       "some-other-input",
// 				},
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
// 					BuildID:         32,
// 					JobID:           12,
// 					InputName:       "some-input",
// 				},
// 			}
// 		})

// 		It("sets FirstOccurrence to false", func() {
// 			Expect(inputMapping).To(Equal(algorithm.InputMapping{
// 				"some-input": algorithm.InputVersion{VersionID: 2, ResourceID: 21, FirstOccurrence: false},
// 			}))
// 		})
// 	})

// 	Context("when the version was an input of the same job with a different name", func() {
// 		BeforeEach(func() {
// 			versionsDB.BuildInputs = []algorithm.BuildInput{
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
// 					BuildID:         31,
// 					JobID:           11,
// 					InputName:       "some-other-input",
// 				},
// 			}
// 		})

// 		It("sets FirstOccurrence to true", func() {
// 			Expect(inputMapping).To(Equal(algorithm.InputMapping{
// 				"some-input": algorithm.InputVersion{VersionID: 2, ResourceID: 21, FirstOccurrence: true},
// 			}))
// 		})
// 	})

// 	Context("when the version was an input of a different job with the same name", func() {
// 		BeforeEach(func() {
// 			versionsDB.BuildInputs = []algorithm.BuildInput{
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
// 					BuildID:         32,
// 					JobID:           12,
// 					InputName:       "some-input",
// 				},
// 			}
// 		})

// 		It("sets FirstOccurrence to true", func() {
// 			Expect(inputMapping).To(Equal(algorithm.InputMapping{
// 				"some-input": algorithm.InputVersion{VersionID: 2, ResourceID: 21, FirstOccurrence: true},
// 			}))
// 		})
// 	})

// 	Context("when a different version was an input of the same job with the same name", func() {
// 		BeforeEach(func() {
// 			versionsDB.BuildInputs = []algorithm.BuildInput{
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 1, ResourceID: 21, CheckOrder: 1},
// 					BuildID:         31,
// 					JobID:           11,
// 					InputName:       "some-input",
// 				},
// 			}
// 		})

// 		It("sets FirstOccurrence to true", func() {
// 			Expect(inputMapping).To(Equal(algorithm.InputMapping{
// 				"some-input": algorithm.InputVersion{VersionID: 2, ResourceID: 21, FirstOccurrence: true},
// 			}))
// 		})
// 	})

// 	Context("when a different version was an output of the same job", func() {
// 		BeforeEach(func() {
// 			versionsDB.BuildOutputs = []algorithm.BuildOutput{
// 				{
// 					ResourceVersion: algorithm.ResourceVersion{VersionID: 1, ResourceID: 21, CheckOrder: 1},
// 					BuildID:         31,
// 					JobID:           11,
// 				},
// 			}
// 		})

// 		It("sets FirstOccurrence to true", func() {
// 			Expect(inputMapping).To(Equal(algorithm.InputMapping{
// 				"some-input": algorithm.InputVersion{VersionID: 2, ResourceID: 21, FirstOccurrence: true},
// 			}))
// 		})
// 	})
// })
