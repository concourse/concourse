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
var DatabaseQueries = &Counter{}

var ContainersCreated = &Counter{}
var VolumesCreated = &Counter{}

var FailedContainers = &Counter{}
var FailedVolumes = &Counter{}

var ContainersDeleted = &Counter{}
var VolumesDeleted = &Counter{}
var ChecksDeleted = &Counter{}

var JobsScheduled = &Counter{}
var JobsScheduling = &Gauge{}

var BuildsStarted = &Counter{}
var BuildsRunning = &Gauge{}

var TasksWaiting = &Gauge{}

var ChecksFinishedWithError = &Counter{}
var ChecksFinishedWithSuccess = &Counter{}
var ChecksQueueSize = &Gauge{}
var ChecksStarted = &Counter{}
var ChecksEnqueued = &Counter{}

var ConcurrentRequests = map[string]*Gauge{}
var ConcurrentRequestsLimitHit = map[string]*Counter{}

type BuildCollectorDuration struct {
	Duration time.Duration
}

func (event BuildCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-build-collector-duration"),
		Event{
			Name:  "gc: build collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type WorkerCollectorDuration struct {
	Duration time.Duration
}

func (event WorkerCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-worker-collector-duration"),
		Event{
			Name:  "gc: worker collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type ResourceCacheUseCollectorDuration struct {
	Duration time.Duration
}

func (event ResourceCacheUseCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-resource-cache-use-collector-duration"),
		Event{
			Name:  "gc: resource cache use collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type ResourceConfigCollectorDuration struct {
	Duration time.Duration
}

func (event ResourceConfigCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-resource-config-collector-duration"),
		Event{
			Name:  "gc: resource config collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type ResourceCacheCollectorDuration struct {
	Duration time.Duration
}

func (event ResourceCacheCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-resource-cache-collector-duration"),
		Event{
			Name:  "gc: resource cache collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type ResourceConfigCheckSessionCollectorDuration struct {
	Duration time.Duration
}

func (event ResourceConfigCheckSessionCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-resource-config-check-session-collector-duration"),
		Event{
			Name:  "gc: resource config check session collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type ArtifactCollectorDuration struct {
	Duration time.Duration
}

func (event ArtifactCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-artifact-collector-duration"),
		Event{
			Name:  "gc: artifact collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type ContainerCollectorDuration struct {
	Duration time.Duration
}

func (event ContainerCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-container-collector-duration"),
		Event{
			Name:  "gc: container collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type VolumeCollectorDuration struct {
	Duration time.Duration
}

func (event VolumeCollectorDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("gc-volume-collector-duration"),
		Event{
			Name:  "gc: volume collector duration (ms)",
			Value: ms(event.Duration),
		},
	)
}

type SchedulingJobDuration struct {
	PipelineName string
	JobName      string
	JobID        int
	Duration     time.Duration
}

func (event SchedulingJobDuration) Emit(logger lager.Logger) {
	emit(
		logger.Session("job-scheduling-duration"),
		Event{
			Name:  "scheduling: job duration (ms)",
			Value: ms(event.Duration),
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
				"job":      event.JobName,
				"job_id":   strconv.Itoa(event.JobID),
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

func (event WorkerContainers) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-containers"),
		Event{
			Name:  "worker containers",
			Value: float64(event.Containers),
			Attributes: map[string]string{
				"worker":    event.WorkerName,
				"platform":  event.Platform,
				"team_name": event.TeamName,
				"tags":      strings.Join(event.Tags[:], "/"),
			},
		},
	)
}

type WorkerUnknownContainers struct {
	WorkerName string
	Containers int
}

func (event WorkerUnknownContainers) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-unknown-containers"),
		Event{
			Name:  "worker unknown containers",
			Value: float64(event.Containers),
			Attributes: map[string]string{
				"worker": event.WorkerName,
			},
		},
	)
}

type WorkerVolumes struct {
	WorkerName string
	Platform   string
	Volumes    int
	TeamName   string
	Tags       []string
}

func (event WorkerVolumes) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-volumes"),
		Event{
			Name:  "worker volumes",
			Value: float64(event.Volumes),
			Attributes: map[string]string{
				"worker":    event.WorkerName,
				"platform":  event.Platform,
				"team_name": event.TeamName,
				"tags":      strings.Join(event.Tags[:], "/"),
			},
		},
	)
}

type WorkerUnknownVolumes struct {
	WorkerName string
	Volumes    int
}

func (event WorkerUnknownVolumes) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-unknown-volumes"),
		Event{
			Name:  "worker unknown volumes",
			Value: float64(event.Volumes),
			Attributes: map[string]string{
				"worker": event.WorkerName,
			},
		},
	)
}

type WorkerTasks struct {
	WorkerName string
	Platform   string
	Tasks      int
}

func (event WorkerTasks) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-tasks"),
		Event{
			Name:  "worker tasks",
			Value: float64(event.Tasks),
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
			Value:      float64(event.Volumes),
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
			Value:      float64(event.Containers),
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
			Value:      float64(event.Containers),
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
			Value:      float64(event.Containers),
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
			Value:      float64(event.Containers),
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
			Value:      float64(event.Volumes),
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
			Value:      float64(event.Volumes),
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
			Value:      float64(event.Volumes),
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
			Value: float64(event.BuildID),
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
			Value: float64(e.Value),
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
	emit(
		logger.Session("http-response-time"),
		Event{
			Name:  "http response time",
			Value: ms(event.Duration),
			Attributes: map[string]string{
				"route":  event.Route,
				"path":   event.Path,
				"method": event.Method,
				"status": strconv.Itoa(event.StatusCode),
			},
		},
	)
}

var lockTypeNames = map[int]string{
	lock.LockTypeResourceConfigChecking: "ResourceConfigChecking",
	lock.LockTypeBuildTracking:          "BuildTracking",
	lock.LockTypeJobScheduling:          "JobScheduling",
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
	for _, state := range db.AllWorkerStates() {
		count := 0
		for _, workerState := range event.WorkerStateByName {
			if workerState == state {
				count += 1
			}
		}

		emit(
			logger.Session("worker-state"),
			Event{
				Name:  "worker state",
				Value: float64(count),
				Attributes: map[string]string{
					"state": string(state),
				},
			},
		)
	}
}
