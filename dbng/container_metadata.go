package dbng

import "fmt"

type ContainerMetadata struct {
	Type ContainerType

	StepName string
	Attempt  string

	PipelineID     int
	JobID          int
	BuildID        int
	ResourceID     int
	ResourceTypeID int

	WorkingDirectory string
	User             string
}

type ContainerType string

const (
	ContainerTypeCheck ContainerType = "check"
	ContainerTypeGet   ContainerType = "get"
	ContainerTypePut   ContainerType = "put"
	ContainerTypeTask  ContainerType = "task"
)

func ContainerTypeFromString(containerType string) (ContainerType, error) {
	switch containerType {
	case "check":
		return ContainerTypeCheck, nil
	case "get":
		return ContainerTypeGet, nil
	case "put":
		return ContainerTypePut, nil
	case "task":
		return ContainerTypeTask, nil
	default:
		return "", fmt.Errorf("Unrecognized containerType: %s", containerType)
	}
}

func (metadata ContainerMetadata) SQLMap() map[string]interface{} {
	return map[string]interface{}{
		"meta_type":              string(metadata.Type),
		"meta_step_name":         metadata.StepName,
		"meta_attempt":           metadata.Attempt,
		"meta_pipeline_id":       metadata.PipelineID,
		"meta_job_id":            metadata.JobID,
		"meta_build_id":          metadata.BuildID,
		"meta_resource_id":       metadata.ResourceID,
		"meta_resource_type_id":  metadata.ResourceTypeID,
		"meta_working_directory": metadata.WorkingDirectory,
		"meta_process_user":      metadata.User,
	}
}

var containerMetadataColumns = []string{
	"meta_type",
	"meta_step_name",
	"meta_attempt",
	"meta_pipeline_id",
	"meta_job_id",
	"meta_build_id",
	"meta_resource_id",
	"meta_resource_type_id",
	"meta_working_directory",
	"meta_process_user",
}

func (metadata *ContainerMetadata) ScanTargets() []interface{} {
	return []interface{}{
		&metadata.Type,
		&metadata.StepName,
		&metadata.Attempt,
		&metadata.PipelineID,
		&metadata.JobID,
		&metadata.BuildID,
		&metadata.ResourceID,
		&metadata.ResourceTypeID,
		&metadata.WorkingDirectory,
		&metadata.User,
	}
}
