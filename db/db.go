package db

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . Conn

type Conn interface {
	Begin() (*sql.Tx, error)
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

type DB interface {
	GetBuild(buildID int) (Build, bool, error)
	GetAllBuilds() ([]Build, error)
	GetAllStartedBuilds() ([]Build, error)

	CreatePipe(pipeGUID string, url string) error
	GetPipe(pipeGUID string) (Pipe, error)

	CreateOneOffBuild() (Build, error)

	LeaseBuildTracking(buildID int, interval time.Duration) (Lease, bool, error)
	LeaseBuildScheduling(buildID int, interval time.Duration) (Lease, bool, error)

	StartBuild(buildID int, engineName, engineMetadata string) (bool, error)
	FinishBuild(buildID int, status Status) error
	ErrorBuild(buildID int, cause error) error

	SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error)
	SaveBuildOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error)

	GetBuildEvents(buildID int, from uint) (EventSource, error)
	SaveBuildEvent(buildID int, event atc.Event) error

	SaveBuildEngineMetadata(buildID int, engineMetadata string) error

	AbortBuild(buildID int) error
	AbortNotifier(buildID int) (Notifier, error)

	Workers() ([]WorkerInfo, error) // auto-expires workers based on ttl
	GetWorker(workerName string) (WorkerInfo, bool, error)
	SaveWorker(WorkerInfo, time.Duration) error

	FindContainerInfosByIdentifier(ContainerIdentifier) ([]ContainerInfo, error)
	GetContainerInfo(string) (ContainerInfo, bool, error)
	CreateContainerInfo(ContainerInfo, time.Duration) error
	FindContainerInfoByIdentifier(ContainerIdentifier) (ContainerInfo, bool, error)
	UpdateExpiresAtOnContainerInfo(handle string, ttl time.Duration) error
	ReapContainer(handle string) error

	DeleteContainerInfo(string) error

	GetConfigByBuildID(buildID int) (atc.Config, ConfigVersion, error)
}

//go:generate counterfeiter . Notifier

type Notifier interface {
	Notify() <-chan struct{}
	Close() error
}

//go:generate counterfeiter . PipelinesDB

type PipelinesDB interface {
	GetAllActivePipelines() ([]SavedPipeline, error)
	GetPipelineByName(pipelineName string) (SavedPipeline, error)

	OrderPipelines([]string) error
}

//go:generate counterfeiter . ConfigDB

type ConfigDB interface {
	GetConfig(pipelineName string) (atc.Config, ConfigVersion, error)
	SaveConfig(string, atc.Config, ConfigVersion, PipelinePausedState) (bool, error)
}

// sequence identifier used for compare-and-swap
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
	Next() (atc.Event, error)
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

type WorkerInfo struct {
	GardenAddr      string
	BaggageclaimURL string

	ActiveContainers int
	ResourceTypes    []atc.WorkerResourceType
	Platform         string
	Tags             []string
	Name             string
}
