package algorithm_test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/extensions/table"
)

// NOTE: The purpose of these tests are to test the migration of build inputs
// and outputs to the new successful build outputs table. These tests are very
// dependent on the Row Limit that is set in the table helpers test file. The
// value is currently set to 2 and these tests that expect the migrated rows
// are all written to revolve around that number limit.

var _ = DescribeTable("Migrating build inputs and outputs into successful build outputs",
	(Example).Run,

	Entry("migrating all build inputs/outputs and finds a candidate", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1, RerunOfBuildID: 2},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv1", CheckOrder: 1, RerunOfBuildID: 7},
				{Job: "simple-b", BuildID: 10, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2, RerunOfBuildID: 2},
				{Job: "simple-a", BuildID: 5, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-y", Version: "ryv2", CheckOrder: 2, RerunOfBuildID: 7},
				{Job: "simple-b", BuildID: 10, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "simple-c", BuildID: 11, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 6, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 7},
				{Job: "simple-b", BuildID: 10, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Resource: "resource-y", Version: "ryv4", CheckOrder: 4},
				{Resource: "resource-y", Version: "ryv5", CheckOrder: 5},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b", "simple-c"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
				"resource-y": "ryv4",
			},
			ExpectedMigrated: map[int]map[int][]string{
				2: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				3: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv3")},
				},
				4: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				5: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv4")},
				},
				6: map[int][]string{
					1: []string{migratorConvertToMD5("rxv1"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv1")},
				},
				7: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				8: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv3")},
				},
				9: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				10: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv4")},
				},
				11: map[int][]string{
					2: []string{migratorConvertToMD5("ryv4")},
				},
			},
		},
	}),

	Entry("migrates all build inputs/outputs and does not find a candidate", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 7},
				{Job: "simple-b", BuildID: 10, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2, RerunOfBuildID: 2},
				{Job: "simple-a", BuildID: 5, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-y", Version: "ryv2", CheckOrder: 2, RerunOfBuildID: 7},
				{Job: "simple-b", BuildID: 10, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "simple-c", BuildID: 11, Resource: "resource-y", Version: "ryv5", CheckOrder: 4},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 6, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 7},
				{Job: "simple-b", BuildID: 10, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Resource: "resource-y", Version: "ryv4", CheckOrder: 4},
				{Resource: "resource-y", Version: "ryv5", CheckOrder: 5},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b", "simple-c"},
			},
		},

		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-y": "no satisfiable builds from passed jobs found for set of inputs",
			},
			ExpectedMigrated: map[int]map[int][]string{
				1: map[int][]string{
					1: []string{migratorConvertToMD5("rxv1"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv1")},
				},
				2: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv2")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				3: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3"), migratorConvertToMD5("rxv3")},
					2: []string{migratorConvertToMD5("ryv3")},
				},
				4: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv2")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				5: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4"), migratorConvertToMD5("rxv4")},
					2: []string{migratorConvertToMD5("ryv4")},
				},
				6: map[int][]string{
					1: []string{migratorConvertToMD5("rxv1"), migratorConvertToMD5("rxv1")},
					2: []string{migratorConvertToMD5("ryv1")},
				},
				7: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv2")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				8: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3"), migratorConvertToMD5("rxv3")},
					2: []string{migratorConvertToMD5("ryv3")},
				},
				9: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv2")},
					2: []string{migratorConvertToMD5("ryv2")},
				},
				10: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4"), migratorConvertToMD5("rxv4")},
					2: []string{migratorConvertToMD5("ryv4")},
				},
				11: map[int][]string{
					2: []string{migratorConvertToMD5("ryv5")},
				},
			},
		},
	}),

	Entry("migrates part of the build inputs/outputs and finds a candidate with version every", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 6, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv3", CheckOrder: 4},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
			ExpectedMigrated: map[int]map[int][]string{
				2: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2")},
				},
				3: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
				6: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
			},
		},
	}),

	Entry("migrates older build inputs/outputs and finds a candidate with version every", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 4, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 5, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
			ExpectedMigrated: map[int]map[int][]string{
				4: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4")},
				},
				3: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
				5: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
			},
		},
	}),

	Entry("migrates part of the build inputs/outputs and finds a candidate with version latest", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
			ExpectedMigrated: map[int]map[int][]string{
				3: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
				4: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4")},
				},
				6: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
			},
		},
	}),

	Entry("migrating preserves outputs over inputs", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
			ExpectedMigrated: map[int]map[int][]string{
				1: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2"), migratorConvertToMD5("rxv1")},
				},
			},
		},
	}),

	Entry("migrates only successful build inputs/outputs", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 6, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3, BuildStatus: "failed"},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, BuildStatus: "pending"},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
			},
			ExpectedMigrated: map[int]map[int][]string{
				2: map[int][]string{
					1: []string{migratorConvertToMD5("rxv2")},
				},
				4: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4")},
				},
				6: map[int][]string{
					1: []string{migratorConvertToMD5("rxv1")},
				},
				8: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
				9: map[int][]string{
					1: []string{migratorConvertToMD5("rxv4")},
				},
			},
		},
	}),

	Entry("migrates build inputs/outputs with rerun builds in the right order", Example{
		DB: DB{
			NeedsV6Migration: true,

			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv5", CheckOrder: 5},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv5", CheckOrder: 5, RerunOfBuildID: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
				{Resource: "resource-x", Version: "rxv5", CheckOrder: 5},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
			ExpectedMigrated: map[int]map[int][]string{
				4: map[int][]string{
					1: []string{migratorConvertToMD5("rxv3")},
				},
			},
		},
	}),
)

func migratorConvertToMD5(version string) string {
	versionJSON, _ := json.Marshal(atc.Version{"ver": version})

	hasher := md5.New()
	hasher.Write([]byte(versionJSON))
	return hex.EncodeToString(hasher.Sum(nil))
}
