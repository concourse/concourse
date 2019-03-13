package metric

import (
	"strconv"
	"time"

	"github.com/concourse/concourse/atc/db/lock"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

var Databases []db.Conn
var DatabaseQueries = Meter(0)

var ContainersCreated = Meter(0)
var VolumesCreated = Meter(0)

var FailedContainers = Meter(0)
var FailedVolumes = Meter(0)

var ContainersDeleted = Meter(0)
var VolumesDeleted = Meter(0)

type SchedulingFullDuration struct {
	PipelineName string
	Duration     time.Duration
}

func (event SchedulingFullDuration) Emit(logger lager.Logger) {
	state := EventStateOK

	if event.Duration > time.Second {
		state = EventStateWarning
	}

	if event.Duration > 5*time.Second {
		state = EventStateCritical
	}

	emit(
		logger.Session("full-scheduling-duration"),
		Event{
			Name:  "scheduling: full duration (ms)",
			Value: ms(event.Duration),
			State: state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
			},
		},
	)
}

type SchedulingLoadVersionsDuration struct {
	PipelineName string
	Duration     time.Duration
}

func (event SchedulingLoadVersionsDuration) Emit(logger lager.Logger) {
	state := EventStateOK

	if event.Duration > time.Second {
		state = EventStateWarning
	}

	if event.Duration > 5*time.Second {
		state = EventStateCritical
	}

	emit(
		logger.Session("loading-versions-duration"),
		Event{
			Name:  "scheduling: loading versions duration (ms)",
			Value: ms(event.Duration),
			State: state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
			},
		},
	)
}

type SchedulingJobDuration struct {
	PipelineName string
	JobName      string
	Duration     time.Duration
}

func (event SchedulingJobDuration) Emit(logger lager.Logger) {
	state := EventStateOK

	if event.Duration > time.Second {
		state = EventStateWarning
	}

	if event.Duration > 5*time.Second {
		state = EventStateCritical
	}

	emit(
		logger.Session("job-scheduling-duration"),
		Event{
			Name:  "scheduling: job duration (ms)",
			Value: ms(event.Duration),
			State: state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
				"job":      event.JobName,
			},
		},
	)
}

type WorkerContainers struct {
	WorkerName string
	Platform   string
	Containers int
}

func (event WorkerContainers) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-containers"),
		Event{
			Name:  "worker containers",
			Value: event.Containers,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker":   event.WorkerName,
				"platform": event.Platform,
			},
		},
	)
}

type WorkerVolumes struct {
	WorkerName string
	Platform   string
	Volumes    int
}

func (event WorkerVolumes) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-volumes"),
		Event{
			Name:  "worker volumes",
			Value: event.Volumes,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker":   event.WorkerName,
				"platform": event.Platform,
			},
		},
	)
}

type VolumesToBeGarbageCollected struct {
	Volumes int
}

func (event VolumesToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-orphaned-volumes-for-deletion"),
		Event{
			Name:       "orphaned volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type CreatingContainersToBeGarbageCollected struct {
	Containers int
}

func (event CreatingContainersToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-creating-containers-for-deletion"),
		Event{
			Name:       "creating containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type CreatedContainersToBeGarbageCollected struct {
	Containers int
}

func (event CreatedContainersToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-created-ccontainers-for-deletion"),
		Event{
			Name:       "created containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type DestroyingContainersToBeGarbageCollected struct {
	Containers int
}

func (event DestroyingContainersToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-destroying-containers-for-deletion"),
		Event{
			Name:       "destroying containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type FailedContainersToBeGarbageCollected struct {
	Containers int
}

func (event FailedContainersToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-failed-containers-for-deletion"),
		Event{
			Name:       "failed containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type CreatedVolumesToBeGarbageCollected struct {
	Volumes int
}

func (event CreatedVolumesToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-created-volumes-for-deletion"),
		Event{
			Name:       "created volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type DestroyingVolumesToBeGarbageCollected struct {
	Volumes int
}

func (event DestroyingVolumesToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-destroying-volumes-for-deletion"),
		Event{
			Name:       "destroying volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type FailedVolumesToBeGarbageCollected struct {
	Volumes int
}

func (event FailedVolumesToBeGarbageCollected) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-found-failed-volumes-for-deletion"),
		Event{
			Name:       "failed volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	)
}

type GarbageCollectionContainerCollectorJobDropped struct {
	WorkerName string
}

func (event GarbageCollectionContainerCollectorJobDropped) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-container-collector-dropped"),
		Event{
			Name:  "GC container collector job dropped",
			Value: 1,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker": event.WorkerName,
			},
		},
	)
}

type BuildStarted struct {
	PipelineName string
	JobName      string
	BuildName    string
	BuildID      int
	TeamName     string
}

func (event BuildStarted) Emit(logger lager.Logger) {
	emit(
		logger.Session("build-started"),
		Event{
			Name:  "build started",
			Value: event.BuildID,
			State: EventStateOK,
			Attributes: map[string]string{
				"pipeline":   event.PipelineName,
				"job":        event.JobName,
				"build_name": event.BuildName,
				"build_id":   strconv.Itoa(event.BuildID),
				"team_name":  event.TeamName,
			},
		},
	)
}

type BuildFinished struct {
	PipelineName  string
	JobName       string
	BuildName     string
	BuildID       int
	BuildStatus   db.BuildStatus
	BuildDuration time.Duration
	TeamName      string
}

func (event BuildFinished) Emit(logger lager.Logger) {
	emit(
		logger.Session("build-finished"),
		Event{
			Name:  "build finished",
			Value: ms(event.BuildDuration),
			State: EventStateOK,
			Attributes: map[string]string{
				"pipeline":     event.PipelineName,
				"job":          event.JobName,
				"build_name":   event.BuildName,
				"build_id":     strconv.Itoa(event.BuildID),
				"build_status": string(event.BuildStatus),
				"team_name":    event.TeamName,
			},
		},
	)
}

func ms(duration time.Duration) float64 {
	return float64(duration) / 1000000
}

type ErrorLog struct {
	Message string
	Value   int
}

func (e ErrorLog) Emit(logger lager.Logger) {
	emit(
		logger.Session("error-log"),
		Event{
			Name:  "error log",
			Value: e.Value,
			State: EventStateWarning,
			Attributes: map[string]string{
				"message": e.Message,
			},
		},
	)
}

type HTTPResponseTime struct {
	Route      string
	Path       string
	Method     string
	StatusCode int
	Duration   time.Duration
}

func (event HTTPResponseTime) Emit(logger lager.Logger) {
	state := EventStateOK

	if event.Duration > 100*time.Millisecond {
		state = EventStateWarning
	}

	if event.Duration > 1*time.Second {
		state = EventStateCritical
	}

	emit(
		logger.Session("http-response-time"),
		Event{
			Name:  "http response time",
			Value: ms(event.Duration),
			State: state,
			Attributes: map[string]string{
				"route":  event.Route,
				"path":   event.Path,
				"method": event.Method,
				"status": strconv.Itoa(event.StatusCode),
			},
		},
	)
}

type ResourceCheck struct {
	PipelineName string
	ResourceName string
	TeamName     string
	Success      bool
}

func (event ResourceCheck) Emit(logger lager.Logger) {
	state := EventStateOK
	if !event.Success {
		state = EventStateWarning
	}
	emit(
		logger.Session("resource-check"),
		Event{
			Name:  "resource checked",
			Value: 1,
			State: state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
				"resource": event.ResourceName,
				"team":     event.TeamName,
			},
		},
	)
}

var lockTypeNames = map[int]string{
	lock.LockTypeResourceConfigChecking: "ResourceConfigChecking",
	lock.LockTypeBuildTracking:          "BuildTracking",
	lock.LockTypePipelineScheduling:     "PipelineScheduling",
	lock.LockTypeBatch:                  "Batch",
	lock.LockTypeVolumeCreating:         "VolumeCreating",
	lock.LockTypeContainerCreating:      "ContainerCreating",
	lock.LockTypeDatabaseMigration:      "DatabaseMigration",
}

type LockAcquired struct {
	LockType string
}

func (event LockAcquired) Emit(logger lager.Logger) {
	emit(
		logger.Session("lock-acquired"),
		Event{
			Name:  "lock held",
			Value: 1,
			State: EventStateOK,
			Attributes: map[string]string{
				"type": event.LockType,
			},
		},
	)
}

type LockReleased struct {
	LockType string
}

func (event LockReleased) Emit(logger lager.Logger) {
	emit(
		logger.Session("lock-released"),
		Event{
			Name:  "lock held",
			Value: 0,
			State: EventStateOK,
			Attributes: map[string]string{
				"type": event.LockType,
			},
		},
	)
}

func LogLockAcquired(logger lager.Logger, lockID lock.LockID) {
	logger.Debug("acquired")

	if len(lockID) == 0 {
		return
	}

	if lockType, ok := lockTypeNames[lockID[0]]; ok {
		LockAcquired{LockType: lockType}.Emit(logger)
	}
}

func LogLockReleased(logger lager.Logger, lockID lock.LockID) {
	logger.Debug("released")

	if len(lockID) == 0 {
		return
	}

	if lockType, ok := lockTypeNames[lockID[0]]; ok {
		LockReleased{LockType: lockType}.Emit(logger)
	}
}

type WorkersState struct {
	WorkerStateByName map[string]db.WorkerState
}

func (event WorkersState) Emit(logger lager.Logger) {
	var (
		perStateCounter = map[db.WorkerState]int{}
		eventState      EventState
	)

	for _, workerState := range event.WorkerStateByName {
		_, exists := perStateCounter[workerState]
		if !exists {
			perStateCounter[workerState] = 1
			continue
		}

		perStateCounter[workerState] += 1
	}

	for state, count := range perStateCounter {
		if state == db.WorkerStateStalled && count > 0 {
			eventState = EventStateWarning
		} else {
			eventState = EventStateOK
		}

		emit(
			logger.Session("worker-state"),
			Event{
				Name:  "worker state",
				Value: count,
				State: eventState,
				Attributes: map[string]string{
					"state": string(state),
				},
			},
		)
	}
}
