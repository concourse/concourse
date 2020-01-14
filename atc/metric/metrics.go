package metric

import (
	"strconv"
	"strings"
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
var ChecksDeleted = Meter(0)

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
	TeamName   string
	Tags       []string
}

func (event WorkerContainers) Events() []Event {
	return []Event{
		Event{
			Name:  "worker containers",
			Value: event.Containers,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker":    event.WorkerName,
				"platform":  event.Platform,
				"team_name": event.TeamName,
				"tags":      strings.Join(event.Tags[:], "/"),
			},
		},
	}
}

func (event WorkerContainers) Name() string {
	return "worker-containers"
}

type WorkerUnknownContainers struct {
	WorkerName string
	Containers int
}

func (event WorkerUnknownContainers) Events() []Event {
	return []Event{
		Event{
			Name:  "worker unknown containers",
			Value: event.Containers,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker": event.WorkerName,
			},
		},
	}
}

func (event WorkerUnknownContainers) Name() string {
	return "worker-unknown-containers"
}

type WorkerVolumes struct {
	WorkerName string
	Platform   string
	Volumes    int
	TeamName   string
	Tags       []string
}

func (event WorkerVolumes) Events() []Event {
	return []Event{
		Event{
			Name:  "worker volumes",
			Value: event.Volumes,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker":    event.WorkerName,
				"platform":  event.Platform,
				"team_name": event.TeamName,
				"tags":      strings.Join(event.Tags[:], "/"),
			},
		},
	}
}

func (event WorkerVolumes) Name() string {
	return "worker-volumes"
}

type WorkerUnknownVolumes struct {
	WorkerName string
	Volumes    int
}

func (event WorkerUnknownVolumes) Events() []Event {
	return []Event{
		Event{
			Name:  "worker unknown volumes",
			Value: event.Volumes,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker": event.WorkerName,
			},
		},
	}
}

func (event WorkerUnknownVolumes) Name() string {
	return "worker-unknown-volumes"
}

type WorkerTasks struct {
	WorkerName string
	Platform   string
	Tasks      int
}

func (event WorkerTasks) Events() []Event {
	return []Event{
		Event{
			Name:  "worker tasks",
			Value: event.Tasks,
			State: EventStateOK,
			Attributes: map[string]string{
				"worker":   event.WorkerName,
				"platform": event.Platform,
			},
		},
	}
}

func (event WorkerTasks) Name() string {
	return "worker-tasks"
}

type VolumesToBeGarbageCollected struct {
	Volumes int
}

func (event VolumesToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "orphaned volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event VolumesToBeGarbageCollected) Name() string {
	return "gc-found-orphaned-volumes-for-deletion"
}

type CreatingContainersToBeGarbageCollected struct {
	Containers int
}

func (event CreatingContainersToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "creating containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event CreatingContainersToBeGarbageCollected) Name() string {
	return "gc-found-creating-containers-for-deletion"
}

type CreatedContainersToBeGarbageCollected struct {
	Containers int
}

func (event CreatedContainersToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "created containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event CreatedContainersToBeGarbageCollected) Name() string {
	return "gc-found-created-ccontainers-for-deletion"
}

type DestroyingContainersToBeGarbageCollected struct {
	Containers int
}

func (event DestroyingContainersToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "destroying containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event DestroyingContainersToBeGarbageCollected) Name() string {
	return "gc-found-destroying-containers-for-deletion"
}

type FailedContainersToBeGarbageCollected struct {
	Containers int
}

func (event FailedContainersToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "failed containers to be garbage collected",
			Value:      event.Containers,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event FailedContainersToBeGarbageCollected) Name() string {
	return "gc-found-failed-containers-for-deletion"
}

type CreatedVolumesToBeGarbageCollected struct {
	Volumes int
}

func (event CreatedVolumesToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "created volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event CreatedVolumesToBeGarbageCollected) Name() string {
	return "gc-found-created-volumes-for-deletion"
}

type DestroyingVolumesToBeGarbageCollected struct {
	Volumes int
}

func (event DestroyingVolumesToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "destroying volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event DestroyingVolumesToBeGarbageCollected) Name() string {
	return "gc-found-destroying-volumes-for-deletion"
}

type FailedVolumesToBeGarbageCollected struct {
	Volumes int
}

func (event FailedVolumesToBeGarbageCollected) Events() []Event {
	return []Event{
		Event{
			Name:       "failed volumes to be garbage collected",
			Value:      event.Volumes,
			State:      EventStateOK,
			Attributes: map[string]string{},
		},
	}
}

func (event FailedVolumesToBeGarbageCollected) Name() string {
	return "gc-found-failed-volumes-for-deletion"
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

func (event BuildStarted) Events() []Event {
	return []Event{
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
	}
}

func (event BuildStarted) Name() string {
	return "build-started"
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

func (event BuildFinished) Events() []Event {
	return []Event{
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
	}
}

func (event BuildFinished) Name() string {
	return "build-finished"
}

func ms(duration time.Duration) float64 {
	return float64(duration) / 1000000
}

type ErrorLog struct {
	Message string
	Value   int
}

func (e ErrorLog) Events() []Event {
	return []Event{
		Event{
			Name:  "error log",
			Value: e.Value,
			State: EventStateWarning,
			Attributes: map[string]string{
				"message": e.Message,
			},
		},
	}
}

func (e ErrorLog) Name() string {
	return "error-log"
}

type HTTPResponseTime struct {
	Route      string
	Path       string
	Method     string
	StatusCode int
	Duration   time.Duration
}

func (event HTTPResponseTime) Events() []Event {
	state := EventStateOK

	if event.Duration > 100*time.Millisecond {
		state = EventStateWarning
	}

	if event.Duration > 1*time.Second {
		state = EventStateCritical
	}

	return []Event{
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
	}
}

func (event HTTPResponseTime) Name() string {
	return "http-response-time"
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
				"pipeline":  event.PipelineName,
				"resource":  event.ResourceName,
				"team_name": event.TeamName,
			},
		},
	)
}

type CheckStarted struct {
	ResourceConfigScopeID int
	CheckName             string
	CheckStatus           db.CheckStatus
	CheckPendingDuration  time.Duration
}

func (event CheckStarted) Events() []Event {
	return []Event{
		Event{
			Name:  "check started",
			Value: ms(event.CheckPendingDuration),
			State: EventStateOK,
			Attributes: map[string]string{
				"scope_id":     strconv.Itoa(event.ResourceConfigScopeID),
				"check_name":   event.CheckName,
				"check_status": string(event.CheckStatus),
			},
		},
	}
}

func (event CheckStarted) Name() string {
	return "check-started"
}

type CheckFinished struct {
	ResourceConfigScopeID int
	CheckName             string
	CheckStatus           db.CheckStatus
	CheckDuration         time.Duration
}

func (event CheckFinished) Events() []Event {
	return []Event{
		Event{
			Name:  "check finished",
			Value: ms(event.CheckDuration),
			State: EventStateOK,
			Attributes: map[string]string{
				"scope_id":     strconv.Itoa(event.ResourceConfigScopeID),
				"check_name":   event.CheckName,
				"check_status": string(event.CheckStatus),
			},
		},
	}
}

func (event CheckFinished) Name() string {
	return "check-finished"
}

type CheckEnqueue struct {
	CheckName             string
	ResourceConfigScopeID int
}

func (event CheckEnqueue) Emit(logger lager.Logger) {
	emit(
		logger.Session("check-enqueued"),
		Event{
			Name:  "check enqueued",
			Value: 1,
			State: EventStateOK,
			Attributes: map[string]string{
				"scope_id":   strconv.Itoa(event.ResourceConfigScopeID),
				"check_name": event.CheckName,
			},
		},
	)
}

type CheckQueueSize struct {
	Checks int
}

func (event CheckQueueSize) Emit(logger lager.Logger) {
	emit(
		logger.Session("check-queue-size"),
		Event{
			Name:       "check queue size",
			Value:      event.Checks,
			State:      EventStateOK,
			Attributes: map[string]string{},
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
	lock.LockTypeActiveTasks:            "ActiveTasks",
	lock.LockTypeResourceScanning:       "ResourceScanning",
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

func (event WorkersState) Events() []Event {
	var eventState EventState

	events := []Event{}
	for _, state := range db.AllWorkerStates() {
		count := 0
		for _, workerState := range event.WorkerStateByName {
			if workerState == state {
				count += 1
			}
		}

		if state == db.WorkerStateStalled && count > 0 {
			eventState = EventStateWarning
		} else {
			eventState = EventStateOK
		}

		events = append(events,
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
	return events
}

func (event WorkersState) Name() string {
	return "worker-state"
}
