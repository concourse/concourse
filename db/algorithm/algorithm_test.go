package algorithm_test

import (
	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = DescribeTable("Input resolving",
	(Example).Run,

	Entry("can fan-in", Example{
		DB: DB{
			// pass a and b
			{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

			// pass a but not b
			{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		// no v2 as it hasn't passed b
		Result: Result{"resource-x": "rxv1"},
	}),

	Entry("propagates resources together", Example{
		DB: DB{
			{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
			{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a"}},
		},

		Result: Result{
			"resource-x": "rxv1",
			"resource-y": "ryv1",
		},
	}),

	Entry("correlates inputs by build, allowing resources to skip jobs", Example{
		DB: DB{
			{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

			{Job: "fan-in", BuildID: 3, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

			{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			{Job: "simple-a", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a", "fan-in"}},
			{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a"}},
		},

		Result: Result{
			"resource-x": "rxv1",

			// not ryv2, as it didn't make it through build relating simple-a to fan-in
			"resource-y": "ryv1",
		},
	}),

	Entry("finds only versions that passed through together", Example{
		DB: DB{
			{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-b", BuildID: 2, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

			{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 2},
			{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv3", CheckOrder: 2},
			{Job: "simple-b", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 2},
			{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "ryv3", CheckOrder: 2},

			{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
			{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},

			{Job: "simple-b", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
			{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "rxv4", CheckOrder: 1},

			{Job: "simple-b", BuildID: 5, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
			{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "rxv2", CheckOrder: 1},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a", "simple-b"}},
			{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a", "simple-b"}},
		},

		Result: Result{
			"resource-x": "rxv3",
			"resource-y": "ryv3",
		},
	}),

	Entry("can collect distinct versions of resources without correlating by job", Example{
		DB: DB{
			{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
		},

		Inputs: Inputs{
			{Name: "simple-a-resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
			{Name: "simple-b-resource-x", Resource: "resource-x", Passed: []string{"simple-b"}},
		},

		Result: Result{
			"simple-a-resource-x": "rxv1",
			"simple-b-resource-x": "rxv2",
		},
	}),

	Entry("resolves passed constraints with common jobs", Example{
		DB: DB{
			{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
			{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},
			{Job: "job-1", BuildID: 2, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
			{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},
		},

		Inputs: Inputs{
			{
				Name:     "input-1",
				Resource: "resource-1",
				Passed:   []string{"shared-job", "job-1"},
			},
			{
				Name:     "input-2",
				Resource: "resource-2",
				Passed:   []string{"shared-job", "job-2"},
			},
		},

		Result: Result{
			"input-1": "r1-common-to-shared-and-j1",
			"input-2": "r2-common-to-shared-and-j2",
		},
	}),

	Entry("resolves passed constraints with common jobs, skipping versions that are not common to builds of all jobs", Example{
		DB: DB{
			{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
			{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},
			{Job: "job-1", BuildID: 2, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
			{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},

			{Job: "shared-job", BuildID: 4, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1", CheckOrder: 2},
			{Job: "shared-job", BuildID: 4, Resource: "resource-2", Version: "new-r2-common-to-shared-and-j2", CheckOrder: 2},
			{Job: "job-1", BuildID: 5, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1", CheckOrder: 2},
		},

		Inputs: Inputs{
			{
				Name:     "input-1",
				Resource: "resource-1",
				Passed:   []string{"shared-job", "job-1"},
			},
			{
				Name:     "input-2",
				Resource: "resource-2",
				Passed:   []string{"shared-job", "job-2"},
			},
		},

		Result: Result{
			"input-1": "r1-common-to-shared-and-j1",
			"input-2": "r2-common-to-shared-and-j2",
		},
	}),

	Entry("finds the latest version for inputs with no passed constraints", Example{
		DB: DB{
			// build outputs
			{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

			// the versions themselves
			// note: normally there's one of these for each version, including ones
			// that appear as outputs
			{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			{Resource: "resource-y", Version: "ryv4", CheckOrder: 4},
			{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			{Resource: "resource-y", Version: "ryv5", CheckOrder: 5},
			{Resource: "resource-x", Version: "rxv5", CheckOrder: 5},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-x-unconstrained",
				Resource: "resource-x",
			},
			{
				Name:     "resource-y-unconstrained",
				Resource: "resource-y",
			},
		},

		Result: Result{
			"resource-x":               "rxv1",
			"resource-x-unconstrained": "rxv5",
			"resource-y-unconstrained": "ryv5",
		},
	}),

	Entry("check orders take precedence over version ID", Example{
		DB: DB{
			{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x"},
		},

		Result: Result{
			"resource-x": "rxv2",
		},
	}),

	Entry("bosh memory leak regression test", Example{
		LoadDB: "testdata/bosh-versions.json",

		Inputs: Inputs{
			{
				Name:     "bosh-src",
				Resource: "bosh-src",
				Passed: []string{
					"unit-1.9",
					"unit-2.1",
					"integration-2.1-mysql",
					"integration-1.9-postgres",
					"integration-2.1-postgres",
				},
			},
			{
				Name:     "bosh-load-tests",
				Resource: "bosh-load-tests",
			},
		},

		Result: Result{
			"bosh-src":        "imported-r88v9814",
			"bosh-load-tests": "imported-r89v7204",
		},
	}),
)
