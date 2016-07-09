package db

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Conn

type Conn interface {
	Begin() (Tx, error)
	Close() error
	Driver() driver.Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	Ping() error
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
}

//go:generate counterfeiter . Tx

type Tx interface {
	Commit() error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Rollback() error
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

func Wrap(sqlDB *sql.DB) Conn {
	return &wrappedDB{DB: sqlDB}
}

func WrapWithError(sqlDB *sql.DB, err error) (Conn, error) {
	return &wrappedDB{DB: sqlDB}, err
}

type wrappedDB struct {
	*sql.DB
}

func (wrapped *wrappedDB) Begin() (Tx, error) {
	return wrapped.DB.Begin()
}

func swallowUniqueViolation(err error) error {
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Code.Class().Name() == "integrity_constraint_violation" {
				return nil
			}
		}

		return err
	}

	return nil
}

func HashResourceConfig(checkType string, source atc.Source) string {
	sourceJSON, _ := json.Marshal(source)
	return checkType + string(sourceJSON)
}

type DB interface {
	SaveTeam(team Team) (SavedTeam, error)
	GetTeamByName(teamName string) (SavedTeam, bool, error)
	UpdateTeamBasicAuth(team Team) (SavedTeam, error)
	UpdateTeamGitHubAuth(team Team) (SavedTeam, error)
	CreateDefaultTeamIfNotExists() error
	DeleteTeamByName(teamName string) error

	GetBuild(buildID int) (Build, bool, error)

	GetBuildVersionedResources(buildID int) (SavedVersionedResources, error)
	GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error)
	GetBuilds(Page) ([]Build, Pagination, error)
	GetAllStartedBuilds() ([]Build, error)

	FindJobIDForBuild(buildID int) (int, bool, error)

	CreatePipe(pipeGUID string, url string) error
	GetPipe(pipeGUID string) (Pipe, error)

	CreateOneOffBuild() (Build, error)
	GetBuildPreparation(buildID int) (BuildPreparation, bool, error)
	UpdateBuildPreparation(buildPreparation BuildPreparation) error
	ResetBuildPreparationsWithPipelinePaused(pipelineID int) error

	LeaseBuildTracking(logger lager.Logger, buildID int, interval time.Duration) (Lease, bool, error)
	LeaseBuildScheduling(logger lager.Logger, buildID int, interval time.Duration) (Lease, bool, error)
	GetLease(logger lager.Logger, taskName string, interval time.Duration) (Lease, bool, error)

	StartBuild(buildID int, pipelineID int, engineName, engineMetadata string) (bool, error)
	FinishBuild(buildID int, pipelineID int, status Status) error
	ErrorBuild(buildID int, pipelineID int, cause error) error

	SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error)
	SaveBuildOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error)

	GetBuildEvents(buildID int, from uint) (EventSource, error)
	SaveBuildEvent(buildID int, pipelineID int, event atc.Event) error
	DeleteBuildEventsByBuildIDs(buildIDs []int) error

	SaveBuildEngineMetadata(buildID int, engineMetadata string) error

	AbortBuild(buildID int) error
	AbortNotifier(buildID int) (Notifier, error)

	Workers() ([]SavedWorker, error) // auto-expires workers based on ttl
	GetWorker(workerName string) (SavedWorker, bool, error)
	SaveWorker(WorkerInfo, time.Duration) (SavedWorker, error)

	FindContainersByDescriptors(Container) ([]SavedContainer, error)
	GetContainer(string) (SavedContainer, bool, error)
	CreateContainer(container Container, ttl time.Duration, maxLifetime time.Duration, volumeHandles []string) (SavedContainer, error)
	FindContainerByIdentifier(ContainerIdentifier) (SavedContainer, bool, error)
	FindLatestSuccessfulBuildsPerJob() (map[int]int, error)
	FindJobContainersFromUnsuccessfulBuilds() ([]SavedContainer, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
	ReapContainer(handle string) error

	DeleteContainer(string) error

	GetConfigByBuildID(buildID int) (atc.Config, ConfigVersion, error)

	InsertVolume(data Volume) error
	GetVolumes() ([]SavedVolume, error)
	GetVolumesByIdentifier(VolumeIdentifier) ([]SavedVolume, error)
	ReapVolume(string) error
	SetVolumeTTL(string, time.Duration) error
	GetVolumeTTL(volumeHandle string) (time.Duration, bool, error)
	SetVolumeSizeInBytes(string, int64) error
	GetVolumesForOneOffBuildImageResources() ([]SavedVolume, error)

	FindWorkerCheckResourceTypeVersion(workerName string, checkType string) (string, bool, error)

	SaveImageResourceVersion(buildID int, planID atc.PlanID, identifier ResourceCacheIdentifier) error
	GetImageResourceCacheIdentifiersByBuildID(buildID int) ([]ResourceCacheIdentifier, error)
}

//go:generate counterfeiter . Notifier

type Notifier interface {
	Notify() <-chan struct{}
	Close() error
}

//go:generate counterfeiter . PipelinesDB

type PipelinesDB interface {
	GetAllPipelines() ([]SavedPipeline, error)
	GetPipelineByID(pipelineID int) (SavedPipeline, error)
	GetPipelineByTeamNameAndName(teamName string, pipelineName string) (SavedPipeline, error)

	OrderPipelines([]string) error
}

//go:generate counterfeiter . ConfigDB

type ConfigDB interface {
	GetConfig(teamName, pipelineName string) (atc.Config, atc.RawConfig, ConfigVersion, error)
	SaveConfig(string, string, atc.Config, ConfigVersion, PipelinePausedState) (SavedPipeline, bool, error)
}

//ConfigVersion is a sequence identifier used for compare-and-swap
type ConfigVersion int

var ErrConfigComparisonFailed = errors.New("comparison with existing config failed during save")

//go:generate counterfeiter . Lock

type Lock interface {
	Release() error
}

var ErrEndOfBuildEventStream = errors.New("end of build event stream")
var ErrBuildEventStreamClosed = errors.New("build event stream closed")

//go:generate counterfeiter . EventSource

type EventSource interface {
	Next() (event.Envelope, error)
	Close() error
}

type BuildInput struct {
	Name string

	VersionedResource

	FirstOccurrence bool
}

type BuildOutput struct {
	VersionedResource
}

type VersionHistory struct {
	VersionedResource SavedVersionedResource
	InputsTo          []*JobHistory
	OutputsOf         []*JobHistory
}

type JobHistory struct {
	JobName string
	Builds  []Build
}

type SavedWorker struct {
	WorkerInfo

	ExpiresIn time.Duration
}

type WorkerInfo struct {
	GardenAddr      string
	BaggageclaimURL string
	HTTPProxyURL    string
	HTTPSProxyURL   string
	NoProxy         string

	ActiveContainers int
	ResourceTypes    []atc.WorkerResourceType
	Platform         string
	Tags             []string
	Name             string
	StartTime        int64
}
