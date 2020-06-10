package algorithm_test

import (
	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = DescribeTable("Input resolving",
	(Example).Run,

	Entry("can fan-in", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				// pass a and b
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				// pass a but not b
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		// no v2 as it hasn't passed b
		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("propagates resources together", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
			{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a"}},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("correlates inputs by build, allowing resources to skip jobs", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "fan-in", BuildID: 3, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a", "fan-in"}},
			{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a"}},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",

				// not ryv2, as it didn't make it through build relating simple-a to fan-in
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("resolve a resource when it has versions", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Latest: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("does not resolve a resource when it does not have any versions", Example{
		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Pinned: "rxv2"},
			},
		},

		Result: Result{
			OK:     false,
			Errors: map[string]string{"resource-x": "pinned version ver:rxv2 not found"},
		},
	}),

	Entry("finds only versions that passed through together", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 2, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv3", CheckOrder: 2},
				{Job: "simple-b", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 2},
				{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "ryv3", CheckOrder: 2},

				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
				{Job: "simple-a", BuildID: 5, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "rxv4", CheckOrder: 1},

				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "rxv2", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x", Passed: []string{"simple-a", "simple-b"}},
			{Name: "resource-y", Resource: "resource-y", Passed: []string{"simple-a", "simple-b"}},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
				"resource-y": "ryv3",
			},
			PassedBuildIDs: map[string][]int{
				"resource-x": []int{3, 4},
				"resource-y": []int{3, 4},
			},
		},
	}),

	Entry("can collect distinct versions of resources without correlating by job", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{Name: "simple-a-resource-x", Resource: "resource-x", Passed: []string{"simple-a"}},
			{Name: "simple-b-resource-x", Resource: "resource-x", Passed: []string{"simple-b"}},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"simple-a-resource-x": "rxv1",
				"simple-b-resource-x": "rxv2",
			},
		},
	}),

	Entry("resolves passed constraints with common jobs", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},
				{Job: "job-1", BuildID: 2, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
				{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},
			},
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
			OK: true,
			Values: map[string]string{
				"input-1": "r1-common-to-shared-and-j1",
				"input-2": "r2-common-to-shared-and-j2",
			},
		},
	}),

	Entry("resolves passed constraints with common jobs, skipping versions that are not common to builds of all jobs", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},
				{Job: "job-1", BuildID: 2, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
				{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},

				{Job: "shared-job", BuildID: 4, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1", CheckOrder: 2},
				{Job: "shared-job", BuildID: 4, Resource: "resource-2", Version: "new-r2-common-to-shared-and-j2", CheckOrder: 2},
				{Job: "job-1", BuildID: 5, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1", CheckOrder: 2},
			},
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
			OK: true,
			Values: map[string]string{
				"input-1": "r1-common-to-shared-and-j1",
				"input-2": "r2-common-to-shared-and-j2",
			},
		},
	}),

	Entry("finds the latest version for inputs with no passed constraints", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				// build outputs
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			Resources: []DBRow{
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
			OK: true,
			Values: map[string]string{
				"resource-x":               "rxv1",
				"resource-x-unconstrained": "rxv5",
				"resource-y-unconstrained": "ryv5",
			},
			// IC: map[string]bool{},
		},
	}),

	Entry("finds the non-disabled latest version for inputs with no passed constraints", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
				{Resource: "resource-x", Version: "rxv5", CheckOrder: 5, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
			},
		},
	}),

	Entry("returns a missing input reason when no input version satisfies the passed constraint", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"simple-a", "simple-b"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		// only one reason since skipping algorithm if resource does not satisfy passed constraints by itself
		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-y": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("finds next version for inputs that use every version when there is a build for that resource", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
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
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("finds next non-disabled version for inputs that use every version when there is a build for that resource", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2, Disabled: true},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("finds current non-disabled version if all later versions are disabled for inputs that use every version when there is a build for that resource", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("finds last non-disabled version if all later and current versions are disabled for inputs that use every version when there is a build for that resource", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3, Disabled: true},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("finds last enabled version for inputs that use every version when there is no builds for that resource", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Resource: "resource-z", Version: "rzv1", CheckOrder: 1},
				{Resource: "resource-z", Version: "rzv2", CheckOrder: 2, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
			},
			{
				Name:     "resource-z",
				Resource: "resource-z",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv1",
				"resource-z": "rzv1",
			},
		},
	}),

	Entry("finds last version for inputs that use every version when there is no builds for that resource", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv3",
			},
		},
	}),

	Entry("finds next version that passed constraints for inputs that use every version", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
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
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("returns the first common version when the current job has no builds and there are multiple passed constraints with version every", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Job: "simple-b", BuildID: 3, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
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
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("does not find candidates when the current job has no builds, there are multiple passed constraints with version every, and a passed job has no builds", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
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
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("returns the next version when there is a passed constraint with version every", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("returns current version if there is no version after it that satisifies constraints", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 3, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
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
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("returns the common version when there are multiple passed constraints with version every", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
				{FromBuildID: 5, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 5, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
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
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("returns the first version that satisfies constraints when using every version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv2", CheckOrder: 3},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 3, ToBuildID: 1},
				{FromBuildID: 5, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				// only ran for resource-x, not any version of resource-y
				{Job: "shared-job", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				// ran for resource-x and resource-y
				{Job: "shared-job", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 3, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("does not find candidates when there are multiple passed constraints with version every, and one passed job has no builds", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
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
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("returns the latest enabled version when the current job has no builds, and there is a passed constraint with version every", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("returns the current enabled version when there is a passed constraint with version every, and all later verisons are disabled", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2, Disabled: true},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("returns the latest set of versions that satisfy all passed constraint with version every, and the current job has no builds", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Job: "simple-b", BuildID: 3, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "shared-job", BuildID: 5, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 7, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv2",
			},
		},
	}),

	Entry("returns the latest enabled set of versions that satisfy all passed constraint with version every, and the current job has no builds", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Job: "simple-b", BuildID: 3, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "shared-job", BuildID: 5, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 6, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2, Disabled: true},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("returns latest build outputs for the passed job that has not run with the current job when using every version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1}, // resource-y does not have build already
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 1},

				{Job: "shared-job", BuildID: 11, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
				{Job: "shared-job", BuildID: 11, Resource: "resource-y", Version: "ryv3", CheckOrder: 1},

				{Job: "shared-job", BuildID: 12, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
				{Job: "shared-job", BuildID: 12, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},

				{Job: "shared-job", BuildID: 13, Resource: "resource-x", Version: "rxv3", CheckOrder: 1},
				{Job: "shared-job", BuildID: 13, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},
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
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv4",
			},
		},
	}),

	Entry("finds next version that satisfies common constraints when using every version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 4, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Job: "shared-job", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 3, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 6, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 9, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("returns the only set of versions that satisfy constraints when the set is one that has already run", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 5, ToBuildID: 100},
				{FromBuildID: 9, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 1},

				{Job: "shared-job", BuildID: 11, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 11, Resource: "resource-y", Version: "ryv3", CheckOrder: 1},

				{Job: "shared-job", BuildID: 12, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 12, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},
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
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Passed:   []string{"shared-job", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
			PassedBuildIDs: map[string][]int{
				"resource-x": []int{1, 9},
				"resource-y": []int{5, 9},
			},
		},
	}),

	Entry("returns the next set of versions that satisfy constraints when using every version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 5, ToBuildID: 100},
				{FromBuildID: 9, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 1},

				{Job: "shared-job", BuildID: 11, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 11, Resource: "resource-y", Version: "ryv3", CheckOrder: 1},

				{Job: "shared-job", BuildID: 12, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 12, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},
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
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
				"resource-y": "ryv2",
			},
		},
	}),

	Entry("returns earliest set of versions that satisfy the multiple passed constraints with version every when the current job latest build has un-ordered versions independent of the ordering (build ids ordered lowest to highest starting with shared-job)", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 10, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 4, ToBuildID: 10},
				{FromBuildID: 8, ToBuildID: 10},
			},

			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 2, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 3, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},

				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 6, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 9, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv2",
			},
		},
	}),

	Entry("returns earliest set of versions that satisfy the multiple passed constraints with version every when the current job latest build has un-ordered versions independent of the ordering (build ids ordered lowest to highest starting with simple-a)", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
				{FromBuildID: 6, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},

				{Job: "shared-job", BuildID: 8, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 8, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv2",
			},
		},
	}),

	Entry("returns earliest set of versions that satisfy the multiple passed constraints with version every when one of the passed jobs skipped a version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
				{FromBuildID: 5, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},

				{Job: "shared-job", BuildID: 8, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 8, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
				"resource-y": "ryv3",
			},
		},
	}),

	Entry("returns the current set of versions that satisfy the multiple passed constraints with version every when one of the passed job has no newer versions", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
				{FromBuildID: 5, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 8, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 8, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("returns an older set of versions that satisfy the multiple passed constraints with version every when the passed job versions are older than the current set", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv2", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-y", Version: "ryv2", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 1},
				{FromBuildID: 5, ToBuildID: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},

				{Job: "shared-job", BuildID: 8, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 8, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-b", "shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("returns the earliest non-disabled version that satisfies constraints when several versions do not satisfy when using every version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 5, ToBuildID: 100},
				{FromBuildID: 9, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 1},

				{Job: "shared-job", BuildID: 11, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 11, Resource: "resource-y", Version: "ryv3", CheckOrder: 1},

				{Job: "shared-job", BuildID: 12, Resource: "resource-x", Version: "rxv4", CheckOrder: 1},
				{Job: "shared-job", BuildID: 12, Resource: "resource-y", Version: "ryv4", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2, Disabled: true},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Resource: "resource-y", Version: "ryv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-b"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
				"resource-y": "ryv3",
			},
		},
	}),

	Entry("when a passed constraint is added to a job that has already run before, it finds the latest", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("returns a missing input reason when no input version satisfies the shared passed constraints", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-1", Version: "r1-common-to-shared-and-j1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 1, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},

				// resource-1 did not pass job-2 with r1-common-to-shared-and-j1
				{Job: "job-2", BuildID: 3, Resource: "resource-2", Version: "r2-common-to-shared-and-j2", CheckOrder: 1},

				{Job: "shared-job", BuildID: 4, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1", CheckOrder: 2},
				{Job: "shared-job", BuildID: 4, Resource: "resource-2", Version: "new-r2-common-to-shared-and-j2", CheckOrder: 2},

				// resource-2 did not pass job-1 with new-r2-common-to-shared-and-j2
				{Job: "job-1", BuildID: 5, Resource: "resource-1", Version: "new-r1-common-to-shared-and-j1", CheckOrder: 2},
			},
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
			OK: false,
			Errors: map[string]string{
				"input-1": "no satisfiable builds from passed jobs found for set of inputs",
				"input-2": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("resolves to the pinned version when it exists", Example{
		DB: DB{
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
				Version:  Version{Pinned: "rxv2"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("does not resolve a version when the pinned version is not in Versions DB (version is disabled or no builds succeeded)", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				// rxv2 was here
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Pinned: "rxv2"},
			},
		},

		Result: Result{
			OK:     false,
			Errors: map[string]string{"resource-x": "pinned version ver:rxv2 not found"},
		},
	}),

	Entry("resolves the version that is pinned with passed", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "some-job", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "some-job", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "some-job", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "some-job", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
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
				Version:  Version{Pinned: "rxv2"},
				Passed:   []string{"some-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("does not resolve a version when the pinned version has not passed the constraint", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "some-job", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
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
				Version:  Version{Pinned: "rxv2"},
				Passed:   []string{"some-job"},
			},
		},

		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("uses the build that includes the pinned with passed while there are multiple inputs", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 2, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "shared-job", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 3, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 4, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 4, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

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
				Version:  Version{Pinned: "rxv3"},
				Passed:   []string{"shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Pinned: "ryv1"},
				Passed:   []string{"shared-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv3",
				"resource-y": "ryv1",
			},
		},
	}),

	Entry("check orders take precedence over version ID", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{Name: "resource-x", Resource: "resource-x"},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("waiting on upstream job for shared version (ryv3)", Example{
		DB: DB{
			BuildOutputs: []DBRow{
				{Job: "shared-job", BuildID: 1, Resource: "resource-x", Version: "rxv3", CheckOrder: 1},
				{Job: "shared-job", BuildID: 1, Resource: "resource-y", Version: "ryv3", CheckOrder: 1},

				{Job: "simple-a", BuildID: 2, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 3, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"shared-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Passed:   []string{"shared-job", "simple-a"},
			},
		},

		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-y": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("reconfigure passed constraints for job with missing upstream dependency (simple-c)", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-w", Version: "rwv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-v", Version: "rvv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 7, ToBuildID: 100},
				{FromBuildID: 9, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Job: "simple-b", BuildID: 3, Resource: "resource-z", Version: "rzv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 4, Resource: "resource-z", Version: "rzv2", CheckOrder: 2},

				{Job: "simple-c", BuildID: 5, Resource: "resource-w", Version: "rwv3", CheckOrder: 1},
				{Job: "simple-c", BuildID: 6, Resource: "resource-w", Version: "rwv4", CheckOrder: 2},

				{Job: "shared-job", BuildID: 7, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 7, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 8, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "shared-job", BuildID: 8, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "shared-b", BuildID: 9, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-b", BuildID: 9, Resource: "resource-w", Version: "rwv1", CheckOrder: 1},
				{Job: "shared-b", BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "shared-b", BuildID: 10, Resource: "resource-w", Version: "rwv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},

				{Resource: "resource-w", Version: "rwv1", CheckOrder: 1},
				{Resource: "resource-w", Version: "rwv2", CheckOrder: 2},
				{Resource: "resource-w", Version: "rwv3", CheckOrder: 3},
				{Resource: "resource-w", Version: "rwv4", CheckOrder: 4},

				{Resource: "resource-z", Version: "rzv1", CheckOrder: 1},
				{Resource: "resource-z", Version: "rzv2", CheckOrder: 2},

				{Resource: "resource-v", Version: "rvv1", CheckOrder: 1},
				{Resource: "resource-v", Version: "rvv2", CheckOrder: 2},
				{Resource: "resource-v", Version: "rvv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Passed:   []string{"shared-job", "shared-b"},
			},
			{
				Name:     "resource-z",
				Resource: "resource-z",
				Passed:   []string{"simple-b"},
			},
			{
				Name:     "resource-w",
				Resource: "resource-w",
				Passed:   []string{"shared-b", "simple-c"},
			},
			{
				Name:     "resource-v",
				Resource: "resource-v",
				Version:  Version{Latest: true},
			},
		},

		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-y": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-w": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("finds a suitable candidate for any inputs resolved before an unresolveable candidates", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-c", Version: "rcv2", CheckOrder: 2},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 9, ToBuildID: 100},
				{FromBuildID: 14, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 6, Resource: "resource-d", Version: "rdv1", CheckOrder: 1},

				{Job: "simple-b", BuildID: 7, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 8, Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 7, Resource: "resource-d", Version: "rdv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-d", Version: "rdv4", CheckOrder: 4},

				{Job: "shared-job", BuildID: 9, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "shared-job", BuildID: 9, Resource: "resource-d", Version: "rdv1", CheckOrder: 1},

				{Job: "shared-job", BuildID: 10, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 10, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "shared-job", BuildID: 11, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 11, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "shared-job", BuildID: 12, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "shared-job", BuildID: 12, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "simple-1", BuildID: 13, Resource: "resource-a", Version: "rav1", CheckOrder: 1},
				{Job: "simple-1", BuildID: 13, Resource: "resource-c", Version: "rcv1", CheckOrder: 1},
				{Job: "simple-1", BuildID: 14, Resource: "resource-a", Version: "rav2", CheckOrder: 2},
				{Job: "simple-1", BuildID: 14, Resource: "resource-c", Version: "rcv2", CheckOrder: 2},
				{Job: "simple-1", BuildID: 15, Resource: "resource-a", Version: "rav3", CheckOrder: 3},
				{Job: "simple-1", BuildID: 15, Resource: "resource-c", Version: "rcv3", CheckOrder: 3},

				{Job: "simple-3", BuildID: 17, Resource: "resource-d", Version: "rdv1", CheckOrder: 1},
				{Job: "simple-3", BuildID: 18, Resource: "resource-d", Version: "rdv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2, Disabled: true},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
				{Resource: "resource-y", Version: "ryv4", CheckOrder: 4},

				{Resource: "resource-a", Version: "rav1", CheckOrder: 1},
				{Resource: "resource-a", Version: "rav2", CheckOrder: 2},
				{Resource: "resource-a", Version: "rav3", CheckOrder: 3},

				{Resource: "resource-b", Version: "rbv1", CheckOrder: 1},
				{Resource: "resource-b", Version: "rbv2", CheckOrder: 2},
				{Resource: "resource-b", Version: "rbv3", CheckOrder: 3, Disabled: true},

				{Resource: "resource-c", Version: "rcv1", CheckOrder: 1},
				{Resource: "resource-c", Version: "rcv2", CheckOrder: 2},
				{Resource: "resource-c", Version: "rcv3", CheckOrder: 3, Disabled: true},
				{Resource: "resource-c", Version: "rcv4", CheckOrder: 4},

				{Resource: "resource-d", Version: "rdv1", CheckOrder: 1},
				{Resource: "resource-d", Version: "rdv2", CheckOrder: 2},
				{Resource: "resource-d", Version: "rdv3", CheckOrder: 3, Disabled: true},
				{Resource: "resource-d", Version: "rdv4", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"shared-job", "simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"shared-job", "simple-b"},
			},
			{
				Name:     "resource-a",
				Resource: "resource-a",
				Version:  Version{Every: true},
				Passed:   []string{"simple-1"},
			},
			{
				Name:     "resource-b",
				Resource: "resource-b",
				Version:  Version{Every: true},
				Passed:   []string{"simple-2"},
			},
			{
				Name:     "resource-c",
				Resource: "resource-c",
				Version:  Version{Every: true},
			},
			{
				Name:     "resource-d",
				Resource: "resource-d",
				Passed:   []string{"shared-job", "simple-b", "simple-3"},
			},
		},

		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-x": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-y": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-d": "no satisfiable builds from passed jobs found for set of inputs",
				"resource-b": "no satisfiable builds from passed jobs found for set of inputs",
			},
		},
	}),

	Entry("uses partially resolved candidates when there is an error with no passed", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: false,
			Errors: map[string]string{
				"resource-y": "version of resource not found",
			},
		},
	}),

	Entry("finds the next every version scoped to a resource", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				// higher check-order but different resource
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("finds successful candidates when there are multiple outputs from passed constraints that are identical", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},

				{Job: "simple-b", BuildID: 5, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
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
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("only uses the first build output/input to set a version candidate and disregards the other (it should use the output version first)", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "version", Version: "5.5.6-rc.23", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-1", Version: "r1v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-2", Version: "r2v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-3", Version: "r3v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-4", Version: "r4v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-5", Version: "r5v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-6", Version: "r6v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-7", Version: "r7v1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 1, Resource: "resource-8", Version: "r8v1", CheckOrder: 1},

				{Job: "simple-a", BuildID: 2, Resource: "version", Version: "5.5.7-rc.23", CheckOrder: 3},
				{Job: "simple-a", BuildID: 2, Resource: "resource-1", Version: "r1v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-2", Version: "r2v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-3", Version: "r3v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-4", Version: "r4v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-5", Version: "r5v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-6", Version: "r6v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-7", Version: "r7v2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "resource-8", Version: "r8v2", CheckOrder: 2},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "version", Version: "5.5.6", CheckOrder: 2},
				{Job: "simple-a", BuildID: 2, Resource: "version", Version: "5.5.7", CheckOrder: 4},
			},

			Resources: []DBRow{
				{Resource: "resource-1", Version: "r1v1", CheckOrder: 1},
				{Resource: "resource-1", Version: "r1v2", CheckOrder: 2},

				{Resource: "resource-2", Version: "r2v1", CheckOrder: 1},
				{Resource: "resource-2", Version: "r2v2", CheckOrder: 2},

				{Resource: "resource-3", Version: "r3v1", CheckOrder: 1},
				{Resource: "resource-3", Version: "r3v2", CheckOrder: 2},

				{Resource: "resource-4", Version: "r4v1", CheckOrder: 1},
				{Resource: "resource-4", Version: "r4v2", CheckOrder: 2},

				{Resource: "resource-5", Version: "r5v1", CheckOrder: 1},
				{Resource: "resource-5", Version: "r5v2", CheckOrder: 2},

				{Resource: "resource-6", Version: "r6v1", CheckOrder: 1},
				{Resource: "resource-6", Version: "r6v2", CheckOrder: 2},

				{Resource: "resource-7", Version: "r7v1", CheckOrder: 1},
				{Resource: "resource-7", Version: "r7v2", CheckOrder: 2},

				{Resource: "resource-8", Version: "r7v1", CheckOrder: 1},
				{Resource: "resource-8", Version: "r7v2", CheckOrder: 2},

				{Resource: "version", Version: "5.5.6-rc.22", CheckOrder: 1},
				{Resource: "version", Version: "5.5.6", CheckOrder: 2},
				{Resource: "version", Version: "5.5.7-rc.23", CheckOrder: 3},
				{Resource: "version", Version: "5.5.7", CheckOrder: 4},
			},
		},

		Inputs: Inputs{
			{
				Name:     "version",
				Resource: "version",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-1",
				Resource: "resource-1",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-2",
				Resource: "resource-2",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-3",
				Resource: "resource-3",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-4",
				Resource: "resource-4",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-5",
				Resource: "resource-5",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-6",
				Resource: "resource-6",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-7",
				Resource: "resource-7",
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-8",
				Resource: "resource-8",
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"version":    "5.5.7",
				"resource-1": "r1v2",
				"resource-2": "r2v2",
				"resource-3": "r3v2",
				"resource-4": "r4v2",
				"resource-5": "r5v2",
				"resource-6": "r6v2",
				"resource-7": "r7v2",
				"resource-8": "r8v2",
			},
		},

		// run this test enough times to shake out any non-deterministic ordering issues
		Iterations: 100,
	}),

	Entry("with very every and passed, it does not use retrigger builds as latest build", Example{
		DB: DB{
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
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},
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
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
			},
		},
	}),

	Entry("with very every and passed, it does not use retrigger builds as latest build when there are multiple passed jobs", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 4, ToBuildID: 100},
				{FromBuildID: 9, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
				{Job: "simple-b", BuildID: 10, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 7},
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
		},
	}),

	Entry("with passed constraints, it does not use the retrigger build as latest build", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},
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
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv4",
			},
		},
	}),

	Entry("with multiple passed constraints, it does not use retrigger builds as latest build when there are multiple passed jobs", Example{
		DB: DB{
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
				{Job: "simple-a", BuildID: 5, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 2},

				{Job: "simple-b", BuildID: 6, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 7, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 8, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-b", BuildID: 9, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
				{Job: "simple-b", BuildID: 10, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, RerunOfBuildID: 7},
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
				"resource-x": "rxv4",
			},
		},
	}),

	Entry("with a build that has a disabled input of the same resource, still uses the other inputs to resolve", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1, Disabled: true},
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
		},
	}),

	Entry("with version every and passed and unused builds, has next is true", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("with version every and passed and no unused builds, has next is false", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 3, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK:     true,
			NoNext: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("with version every and passed and the unused builds is not satisfiable, has next is false", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 2, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3, Disabled: true},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK:     true,
			NoNext: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("with version every and passed and multiple jobs with one that has unused builds, has next is true", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 4, ToBuildID: 100},
				{FromBuildID: 7, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},

				{Job: "simple-b", BuildID: 4, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "simple-b", BuildID: 5, Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Job: "simple-b", BuildID: 6, Resource: "resource-y", Version: "ryv3", CheckOrder: 3},

				{Job: "simple-c", BuildID: 7, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-c", BuildID: 8, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-c"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a", "simple-b"},
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv2",
				"resource-y": "ryv2",
			},
		},
	}),

	Entry("with version every and unused versions, has next is true", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("with version every and no unused versions, has next is false", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
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
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK:     true,
			NoNext: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("with version every but has never used the version before, has next is false", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK:     true,
			NoNext: true,
			Values: map[string]string{
				"resource-x": "rxv2",
			},
		},
	}),

	Entry("with both version every and version every with passed inputs, the has next value is recognized", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Resource: "resource-y", Version: "ryv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv3", CheckOrder: 3},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Version:  Version{Every: true},
				Passed:   []string{"simple-a"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv2",
			},
		},
	}),

	Entry("when the resource does not have it's resource config scope set, it should error", Example{
		DB: DB{
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1, NoResourceConfigScope: true},
			},
		},

		Inputs: Inputs{
			{
				Name:                  "resource-x",
				Resource:              "resource-x",
				NoResourceConfigScope: true,
			},
		},

		Result: Result{
			OK:      false,
			HasNext: false,
			Errors:  map[string]string{"resource-x": "latest version of resource not found"},
		},
	}),

	Entry("with version every and passed using an old version, it finds latest version ran by job", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 101, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: CurrentJobName, BuildID: 102, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},

			BuildPipes: []DBRow{
				{FromBuildID: 1, ToBuildID: 100},
				{FromBuildID: 2, ToBuildID: 101},
				{FromBuildID: 1, ToBuildID: 102},
			},

			BuildOutputs: []DBRow{
				{Job: "simple-a", BuildID: 1, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "simple-a", BuildID: 2, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: "simple-a", BuildID: 3, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
				{Job: "simple-a", BuildID: 4, Resource: "resource-x", Version: "rxv4", CheckOrder: 4},
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
				Passed:   []string{"simple-a"},
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("with version every without passed using an old version, it finds latest version ran by job", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 101, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: CurrentJobName, BuildID: 102, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
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
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("if another job uses the same resource, that does not affect the next version found for the current job", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: CurrentJobName, BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: CurrentJobName, BuildID: 101, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Job: CurrentJobName, BuildID: 102, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},

				{Job: "another-job", BuildID: 103, Resource: "resource-x", Version: "rxv3", CheckOrder: 3},
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
				Version:  Version{Every: true},
			},
		},

		Result: Result{
			OK:      true,
			HasNext: true,
			Values: map[string]string{
				"resource-x": "rxv3",
			},
		},
	}),

	Entry("if the chosen version for an input with passed constraints does not exist, it will not select that version", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: "another-job", BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "another-job", BuildID: 101, Resource: "resource-x", Version: "rxv2", CheckOrder: 2, DoNotInsertVersion: true},
			},
			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"another-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
			},
		},
	}),

	Entry("if there are multiple inputs with the same passed constraint job and the chosen version from a build does not exist, it will not use that build", Example{
		DB: DB{
			BuildInputs: []DBRow{
				{Job: "another-job", BuildID: 100, Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Job: "another-job", BuildID: 101, Resource: "resource-x", Version: "rxv2", CheckOrder: 2},

				{Job: "another-job", BuildID: 100, Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
				{Job: "another-job", BuildID: 101, Resource: "resource-y", Version: "ryv2", CheckOrder: 2, DoNotInsertVersion: true},
			},

			Resources: []DBRow{
				{Resource: "resource-x", Version: "rxv1", CheckOrder: 1},
				{Resource: "resource-x", Version: "rxv2", CheckOrder: 2},
				{Resource: "resource-y", Version: "ryv1", CheckOrder: 1},
			},
		},

		Inputs: Inputs{
			{
				Name:     "resource-x",
				Resource: "resource-x",
				Passed:   []string{"another-job"},
			},
			{
				Name:     "resource-y",
				Resource: "resource-y",
				Passed:   []string{"another-job"},
			},
		},

		Result: Result{
			OK: true,
			Values: map[string]string{
				"resource-x": "rxv1",
				"resource-y": "ryv1",
			},
		},
	}),
)
