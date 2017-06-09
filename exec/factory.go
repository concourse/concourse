package exec

import (
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . Factory

// Factory is used when building up the steps for a build.
type Factory interface {
	// Get constructs a ActionsStep factory for Get.
	Get(
		lager.Logger,
		int, // teamID
		int, // buildID
		atc.Plan,
		StepMetadata,
		db.ContainerMetadata,
		BuildDelegate,
	) StepFactory

	// Put constructs a ActionsStep factory for Put.
	Put(
		lager.Logger,
		int, // teamID
		int, // buildID
		atc.Plan,
		StepMetadata,
		db.ContainerMetadata,
		BuildDelegate,
	) StepFactory

	// Task constructs a ActionsStep factory for Task.
	Task(
		logger lager.Logger,
		plan atc.Plan,
		teamID int,
		buildID int,
		containerMetadata db.ContainerMetadata,
		delegate BuildDelegate,
	) StepFactory
}

// StepMetadata is used to inject metadata to make available to the step when
// it's running.
type StepMetadata interface {
	Env() []string
}

type BuildDelegate interface {
	GetBuildEventsDelegate(atc.PlanID, atc.GetPlan) BuildEventsDelegate
	PutBuildEventsDelegate(atc.PlanID, atc.PutPlan) BuildEventsDelegate
	TaskBuildEventsDelegate(atc.PlanID, atc.TaskPlan) BuildEventsDelegate
	ImageFetchingDelegate(atc.PlanID) ImageFetchingDelegate
}

type ImageFetchingDelegate interface {
	ImageVersionDetermined(*db.UsedResourceCache) error
	Stdout() io.Writer
	Stderr() io.Writer
}

//go:generate counterfeiter . TaskDelegate

// TaskDelegate is used to record events related to a TaskStep's runtime
// behavior.
type TaskDelegate interface {
	Initializing(atc.TaskConfig)
	Started()

	Finished(ExitStatus)
	Failed(error)

	ImageVersionDetermined(*db.UsedResourceCache) error

	Stdout() io.Writer
	Stderr() io.Writer
}

// ResourceDelegate is used to record events related to a resource's runtime
// behavior.
type ResourceDelegate interface {
	Initializing()

	Completed(ExitStatus, *VersionInfo)
	Failed(error)

	ImageVersionDetermined(*db.UsedResourceCache) error

	Stdout() io.Writer
	Stderr() io.Writer
}

//go:generate counterfeiter . PutDelegate

// PutDelegate is used to record events related to a PutStep's runtime
// behavior.
type PutDelegate interface {
	ResourceDelegate
}

// Privileged is used to indicate whether the given step should run with
// special privileges (i.e. as an administrator user).
type Privileged bool
