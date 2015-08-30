package algorithm_test

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Algorithm", func() {
	examples := []Example{
		{
			It: "can fan-in",

			DB: DB{
				// pass a and b
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1"},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-b", BuildID: 2, Resource: "resource-y", Version: "ryv1"},

				// pass a but not b
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2"},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv2"},
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
		},

		{
			It: "chooses versions from the latest build",

			DB: DB{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1"},

				// ran with rxv3 and ryv3 (having detected v2 but skipped it)
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv3"},
				{Job: "simple-a", BuildID: 2, Resource: "resource-y", Version: "ryv3"},

				// user disabled rxv3 and ryv3, back to 2
				// assume user then reenabled for query to even consider it
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2"},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv2"},
			},

			Inputs: Inputs{
				{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
				{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a"}},
			},

			Result: Result{
				// not rxv3 + rxy3 as the latest build is v2
				"resource-x": "rxv2",
				"resource-y": "ryv2",
			},
		},

		{
			It: "propagates resources together",

			DB: DB{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1"},
			},

			Inputs: Inputs{
				{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
				{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a"}},
			},

			Result: Result{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
		},

		{
			It: "correlates inputs by build, allowing resources to skip jobs",

			DB: DB{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1"},

				{Job: "fan-in", BuildID: 3, Resource: "resource-x", Version: "rxv1"},

				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv2"},
				{Job: "simple-a", BuildID: 4, Resource: "resource-y", Version: "ryv2"},
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
		},

		{
			It: "finds only versions that passed through together",

			DB: DB{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1"},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-b", BuildID: 2, Resource: "resource-y", Version: "ryv1"},

				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3"},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv3"},
				{Job: "simple-b", BuildID: 4, Resource: "resource-x", Version: "rxv3"},
				{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "ryv3"},

				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2"},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv4"},

				{Job: "simple-b", BuildID: 4, Resource: "resource-x", Version: "rxv4"},
				{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "rxv4"},

				{Job: "simple-b", BuildID: 5, Resource: "resource-x", Version: "rxv4"},
				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "rxv2"},
			},

			Inputs: Inputs{
				{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a", "simple-b"}},
				{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a", "simple-b"}},
			},

			Result: Result{
				"resource-x": "rxv3",
				"resource-y": "ryv3",
			},
		},

		{
			It: "can collect distinct versions of resources without correlating by job",

			DB: DB{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv2"},
			},

			Inputs: Inputs{
				{Name: "simple-a-resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
				{Name: "simple-b-resource-x", Resource: "resource-x", Passed: []string{"simple-b"}},
			},

			Result: Result{
				"simple-a-resource-x": "rxv1",
				"simple-b-resource-x": "rxv2",
			},
		},

		{
			It: "resolves passed constraints with common jobs",

			DB: DB{
				{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1"},
				{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2"},
				{Job: "job-1", BuildID: 2, Resource: "resource-1", Version: "r1-common-to-shared-and-j1"},
				{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2"},
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
		},

		{
			It: "resolves passed constraints with common jobs, skipping versions that are not common to builds of all jobs",

			DB: DB{
				{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1"},
				{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2"},
				{Job: "job-1", BuildID: 2, Resource: "resource-1", Version: "r1-common-to-shared-and-j1"},
				{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2"},

				{Job: "shared-job", BuildID: 4, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1"},
				{Job: "shared-job", BuildID: 4, Resource: "resource-2", Version: "new-r2-common-to-shared-and-j2"},
				{Job: "job-1", BuildID: 5, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1"},
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
		},

		{
			It: "finds the latest version for inputs with no passed constraints",

			DB: DB{
				// build outputs
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1"},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1"},

				// the versions themselves
				// note: normally there's one of these for each version, including ones
				// that appear as outputs
				{Resource: "resource-x", Version: "rxv1"},
				{Resource: "resource-y", Version: "ryv2"},
				{Resource: "resource-x", Version: "rxv2"},
				{Resource: "resource-y", Version: "ryv3"},
				{Resource: "resource-x", Version: "rxv3"},
				{Resource: "resource-y", Version: "ryv4"},
				{Resource: "resource-x", Version: "rxv4"},
				{Resource: "resource-y", Version: "ryv5"},
				{Resource: "resource-x", Version: "rxv5"},
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
		},
	}

	for _, example := range examples {
		example := example

		if len(example.FIt) != 0 {
			FIt(example.FIt, func() {
				runExample(example)
			})
		} else {
			It(example.It, func() {
				runExample(example)
			})
		}
	}
})
