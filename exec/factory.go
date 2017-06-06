package exec

import (
	"io"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
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

	// Task constructs a TaskStep factory.
	Task(
		lager.Logger,
		int, // teamID
		int, // buildID
		atc.PlanID,
		worker.ArtifactName,
		db.ContainerMetadata,
		TaskDelegate,
		Privileged,
		atc.Tags,
		TaskConfigSource,
		atc.VersionedResourceTypes,
		map[string]string,
		map[string]string,
		string,
		clock.Clock,
	) StepFactory
}

// StepMetadata is used to inject metadata to make available to the step when
// it's running.
type StepMetadata interface {
	Env() []string
}

type GetResultAction interface {
	Result() (VersionInfo, bool)
}

type PutResultAction interface {
	Result() (VersionInfo, bool)
}

type BuildDelegate interface {
	GetBuildEventsDelegate(atc.PlanID, atc.GetPlan, GetResultAction) BuildEventsDelegate
	PutBuildEventsDelegate(atc.PlanID, atc.PutPlan, PutResultAction) BuildEventsDelegate
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
