package scheduler_test

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("Job Scheduling",
	(Example).Run,

	Entry("one pending build that can be successfully started", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1},
			},
		},

		Result: Result{
			StartedBuilds: []int{1},
			NeedsRetry:    false,
		},
	}),

	Entry("one pending build that is aborted", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, Aborted: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    false,
		},
	}),

	Entry("one pending build that has reached max in flight", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, MaxInFlightReached: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    true,
		},
	}),

	Entry("one manually triggered pending build that does not have resources checked", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, ManuallyTriggered: true, ResourcesNotChecked: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    true,
		},
	}),

	Entry("one pending build that does not have inputs determined", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, InputsNotDetermined: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    false,
		},
	}),

	Entry("one pending build that cannot create build plan", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, CreatingBuildPlanFails: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    false,
		},
	}),

	Entry("one pending build that is unable to start", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, UnableToStart: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    false,
		},
	}),

	Entry("one scheduler build, one manually triggered build and one rerun build", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 4, RerunOfBuildID: 1},
				{ID: 2},
				{ID: 3, ManuallyTriggered: true},
			},
		},

		Result: Result{
			StartedBuilds: []int{4, 2, 3},
			NeedsRetry:    false,
		},
	}),

	Entry("if pending builds is aborted, next build will continue to schedule", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, Aborted: true},
				{ID: 2},
			},
		},

		Result: Result{
			StartedBuilds: []int{2},
			NeedsRetry:    false,
		},
	}),

	Entry("if max in flight is reached, next builds will not schedule", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, MaxInFlightReached: true},
				{ID: 2},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    true,
		},
	}),

	Entry("if resources have not checked for a manually triggered build, next builds will not schedule", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, ManuallyTriggered: true, ResourcesNotChecked: true},
				{ID: 2},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    true,
		},
	}),

	Entry("if the rerun build has no inputs determined, the normal build will continue to get scheduled", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 3, RerunOfBuildID: 1, InputsNotDetermined: true},
				{ID: 2},
			},
		},

		Result: Result{
			StartedBuilds: []int{2},
			NeedsRetry:    false,
		},
	}),

	Entry("if inputs are not determined on a regular build, next builds will not schedule", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 1, InputsNotDetermined: true},
				{ID: 2},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    false,
		},
	}),

	Entry("if both rerun builds cannot determine inputs, next build will continue to schedule", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 4, RerunOfBuildID: 1, InputsNotDetermined: true},
				{ID: 3, RerunOfBuildID: 1, InputsNotDetermined: true},
				{ID: 2},
			},
		},

		Result: Result{
			StartedBuilds: []int{2},
			NeedsRetry:    false,
		},
	}),

	Entry("if regular build fails to schedule, next rerun build will not schedule", Example{
		Job: DBJob{
			Builds: []DBBuild{
				{ID: 2, InputsNotDetermined: true},
				{ID: 4, RerunOfBuildID: 3},
			},
		},

		Result: Result{
			StartedBuilds: []int{},
			NeedsRetry:    false,
		},
	}),
)

type Example struct {
	Job    DBJob
	Result Result
}

type DBJob struct {
	Paused         bool
	PipelinePaused bool

	Builds []DBBuild
}

type DBBuild struct {
	ID                int
	RerunOfBuildID    int
	ManuallyTriggered bool

	Aborted bool

	InputsNotDetermined bool
	ResourcesNotChecked bool
	MaxInFlightReached  bool

	CreatingBuildPlanFails bool
	UnableToStart          bool
}

type Result struct {
	StartedBuilds []int
	NeedsRetry    bool
	Errored       bool
}

func (example Example) Run() {
	fakePlanner := new(schedulerfakes.FakeBuildPlanner)
	fakeAlgorithm := new(schedulerfakes.FakeAlgorithm)
	fakeAlgorithm.ComputeReturns(nil, true, false, nil)

	buildStarter := scheduler.NewBuildStarter(fakePlanner, fakeAlgorithm)

	fakeJob := new(dbfakes.FakeJob)
	fakeJob.ConfigReturns(atc.JobConfig{}, nil)
	fakeJob.SaveNextInputMappingReturns(nil)

	var expectedScheduledBuilds []*dbfakes.FakeBuild
	var pendingBuilds []db.Build
	for i, build := range example.Job.Builds {
		fakeBuild := new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(build.ID)
		fakeBuild.NameReturns(fmt.Sprint(build.ID))
		fakeBuild.IsAbortedReturns(build.Aborted)
		fakeBuild.RerunOfReturns(build.RerunOfBuildID)
		fakeBuild.IsManuallyTriggeredReturns(build.ManuallyTriggered)
		fakeBuild.FinishReturns(nil)

		if build.MaxInFlightReached {
			fakeJob.ScheduleBuildReturnsOnCall(i, false, nil)
		} else {
			fakeJob.ScheduleBuildReturnsOnCall(i, true, nil)
		}

		if build.ResourcesNotChecked {
			fakeBuild.ResourcesCheckedReturns(false, nil)
		} else {
			fakeBuild.ResourcesCheckedReturns(true, nil)
		}

		if build.InputsNotDetermined {
			fakeBuild.AdoptInputsAndPipesReturns(nil, false, nil)
			fakeBuild.AdoptRerunInputsAndPipesReturns(nil, false, nil)
		} else {
			fakeBuild.AdoptInputsAndPipesReturns(nil, true, nil)
			fakeBuild.AdoptRerunInputsAndPipesReturns(nil, true, nil)
		}

		if build.CreatingBuildPlanFails {
			fakePlanner.CreateReturns(atc.Plan{}, errors.New("disaster"))
		} else {
			fakePlanner.CreateReturns(atc.Plan{}, nil)
		}

		if build.UnableToStart {
			fakeBuild.StartReturns(false, nil)
		} else {
			fakeBuild.StartReturns(true, nil)
		}

		expectedScheduledBuilds = append(expectedScheduledBuilds, fakeBuild)
		pendingBuilds = append(pendingBuilds, fakeBuild)
	}

	fakeJob.GetPendingBuildsReturns(pendingBuilds, nil)

	jobInputs := db.InputConfigs{
		{
			Name: "fake-resource",
		},
	}

	needsRetry, err := buildStarter.TryStartPendingBuildsForJob(lager.NewLogger("job-scheduling-tests"), db.SchedulerJob{
		Job: fakeJob,
		Resources: db.SchedulerResources{
			{
				Name: "fake-resource",
				Type: "fake-resource-type",
				Source: atc.Source{
					"some": "source",
				},
			},
		},
	},
		jobInputs)
	if err != nil {
		Expect(example.Result.Errored).To(BeTrue())
	} else {
		Expect(example.Result.Errored).To(BeFalse())
		Expect(needsRetry).To(Equal(example.Result.NeedsRetry))
		for i, buildID := range example.Result.StartedBuilds {
			if expectedScheduledBuilds[i].StartCallCount() > 0 {
				Expect(expectedScheduledBuilds[i].ID()).To(Equal(buildID))
				Expect(expectedScheduledBuilds[i].StartCallCount()).To(Equal(1))
			}
		}
	}
}
