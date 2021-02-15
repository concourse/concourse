package atc

type Container struct {
	ID         string `json:"id"`
	WorkerName string `json:"worker_name"`

	State string `json:"state,omitempty"`
	Type  string `json:"type,omitempty"`

	StepName string `json:"step_name,omitempty"`
	Attempt  string `json:"attempt,omitempty"`

	PipelineID     int `json:"pipeline_id,omitempty"`
	JobID          int `json:"job_id,omitempty"`
	BuildID        int `json:"build_id,omitempty"`
	ResourceID     int `json:"resource_id,omitempty"`
	ResourceTypeID int `json:"resource_type_id,omitempty"`

	PipelineName         string       `json:"pipeline_name,omitempty"`
	PipelineInstanceVars InstanceVars `json:"pipeline_instance_vars,omitempty"`

	JobName          string `json:"job_name,omitempty"`
	BuildName        string `json:"build_name,omitempty"`
	ResourceName     string `json:"resource_name,omitempty"`
	ResourceTypeName string `json:"resource_type_name,omitempty"`

	User             string `json:"user,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`

	ExpiresIn string `json:"expires_in,omitempty"`
}

const (
	ContainerStateCreated    = "created"
	ContainerStateCreating   = "creating"
	ContainerStateDestroying = "destroying"
	ContainerStateFailed     = "failed"
)

type ContainerPlacementStrategy string

var ValidContainerPlacementStrategies = []ContainerPlacementStrategy{
	ContainerPlacementVolumeLocality,
	ContainerPlacementRandom,
	ContainerPlacementFewestBuildContainers,
	ContainerPlacementLimitActiveTasks,
	ContainerPlacementLimitActiveContainers,
	ContainerPlacementLimitActiveVolumes,
}

const (
	ContainerPlacementVolumeLocality        ContainerPlacementStrategy = "volume-locality"
	ContainerPlacementRandom                ContainerPlacementStrategy = "random"
	ContainerPlacementFewestBuildContainers ContainerPlacementStrategy = "fewest-build-containers"
	ContainerPlacementLimitActiveTasks      ContainerPlacementStrategy = "limit-active-tasks"
	ContainerPlacementLimitActiveContainers ContainerPlacementStrategy = "limit-active-containers"
	ContainerPlacementLimitActiveVolumes    ContainerPlacementStrategy = "limit-active-volumes"
)

type StreamingArtifactsCompression string

var ValidStreamingArtifactsCompressions = []StreamingArtifactsCompression{
	StreamingArtifactsGzip,
	StreamingArtifactsZstd,
}

const (
	StreamingArtifactsGzip StreamingArtifactsCompression = "gzip"
	StreamingArtifactsZstd StreamingArtifactsCompression = "zstd"
)
