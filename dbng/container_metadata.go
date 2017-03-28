package dbng

import "fmt"

type ContainerMetadata struct {
	Type ContainerType

	StepName string
	Attempt  string

	WorkingDirectory string
	User             string

	PipelineID     int
	JobID          int
	BuildID        int
	ResourceID     int
	ResourceTypeID int

	PipelineName     string
	JobName          string
	BuildName        string
	ResourceName     string
	ResourceTypeName string
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
	m := map[string]interface{}{}

	if metadata.Type != "" {
		m["meta_type"] = string(metadata.Type)
	}

	if metadata.StepName != "" {
		m["meta_step_name"] = metadata.StepName
	}

	if metadata.Attempt != "" {
		m["meta_attempt"] = metadata.Attempt
	}

	if metadata.WorkingDirectory != "" {
		m["meta_working_directory"] = metadata.WorkingDirectory
	}

	if metadata.User != "" {
		m["meta_process_user"] = metadata.User
	}

	if metadata.PipelineID != 0 {
		m["meta_pipeline_id"] = metadata.PipelineID
	}

	if metadata.JobID != 0 {
		m["meta_job_id"] = metadata.JobID
	}

	if metadata.BuildID != 0 {
		m["meta_build_id"] = metadata.BuildID
	}

	if metadata.ResourceID != 0 {
		m["meta_resource_id"] = metadata.ResourceID
	}

	if metadata.ResourceTypeID != 0 {
		m["meta_resource_type_id"] = metadata.ResourceTypeID
	}

	if metadata.PipelineName != "" {
		m["meta_pipeline_name"] = metadata.PipelineName
	}

	if metadata.JobName != "" {
		m["meta_job_name"] = metadata.JobName
	}

	if metadata.BuildName != "" {
		m["meta_build_name"] = metadata.BuildName
	}

	if metadata.ResourceName != "" {
		m["meta_resource_name"] = metadata.ResourceName
	}

	if metadata.ResourceTypeName != "" {
		m["meta_resource_type_name"] = metadata.ResourceTypeName
	}

	return m
}

var containerMetadataColumns = []string{
	"meta_type",
	"meta_step_name",
	"meta_attempt",
	"meta_working_directory",
	"meta_process_user",
	"meta_pipeline_id",
	"meta_job_id",
	"meta_build_id",
	"meta_resource_id",
	"meta_resource_type_id",
	"meta_pipeline_name",
	"meta_job_name",
	"meta_build_name",
	"meta_resource_name",
	"meta_resource_type_name",
}

func (metadata *ContainerMetadata) ScanTargets() []interface{} {
	return []interface{}{
		&metadata.Type,
		&metadata.StepName,
		&metadata.Attempt,
		&metadata.WorkingDirectory,
		&metadata.User,
		&metadata.PipelineID,
		&metadata.JobID,
		&metadata.BuildID,
		&metadata.ResourceID,
		&metadata.ResourceTypeID,
		&metadata.PipelineName,
		&metadata.JobName,
		&metadata.BuildName,
		&metadata.ResourceName,
		&metadata.ResourceTypeName,
	}
}
