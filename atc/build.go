package atc

type BuildStatus string

const (
	StatusStarted   BuildStatus = "started"
	StatusPending   BuildStatus = "pending"
	StatusSucceeded BuildStatus = "succeeded"
	StatusFailed    BuildStatus = "failed"
	StatusErrored   BuildStatus = "errored"
	StatusAborted   BuildStatus = "aborted"
)

func (status BuildStatus) String() string {
	return string(status)
}

type Build struct {
	ID                   int           `json:"id"`
	TeamName             string        `json:"team_name"`
	Name                 string        `json:"name"`
	Status               BuildStatus   `json:"status"`
	APIURL               string        `json:"api_url"`
	Comment              string        `json:"comment,omitempty"`
	JobName              string        `json:"job_name,omitempty"`
	ResourceName         string        `json:"resource_name,omitempty"`
	PipelineID           int           `json:"pipeline_id,omitempty"`
	PipelineName         string        `json:"pipeline_name,omitempty"`
	PipelineInstanceVars InstanceVars  `json:"pipeline_instance_vars,omitempty"`
	StartTime            int64         `json:"start_time,omitempty"`
	EndTime              int64         `json:"end_time,omitempty"`
	ReapTime             int64         `json:"reap_time,omitempty"`
	RerunNumber          int           `json:"rerun_number,omitempty"`
	RerunOf              *RerunOfBuild `json:"rerun_of,omitempty"`
	CreatedBy            *string       `json:"created_by,omitempty"`
}

type RerunOfBuild struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (b Build) IsRunning() bool {
	switch BuildStatus(b.Status) {
	case StatusPending, StatusStarted:
		return true
	default:
		return false
	}
}

func (b Build) Abortable() bool {
	return b.IsRunning()
}

func (b Build) OneOff() bool {
	return b.JobName == ""
}

type BuildPreparationStatus string

const (
	BuildPreparationStatusUnknown     BuildPreparationStatus = "unknown"
	BuildPreparationStatusBlocking    BuildPreparationStatus = "blocking"
	BuildPreparationStatusNotBlocking BuildPreparationStatus = "not_blocking"
)

type MissingInputReasons map[string]string

type BuildPreparation struct {
	BuildID             int                               `json:"build_id"`
	PausedPipeline      BuildPreparationStatus            `json:"paused_pipeline"`
	PausedJob           BuildPreparationStatus            `json:"paused_job"`
	MaxRunningBuilds    BuildPreparationStatus            `json:"max_running_builds"`
	Inputs              map[string]BuildPreparationStatus `json:"inputs"`
	InputsSatisfied     BuildPreparationStatus            `json:"inputs_satisfied"`
	MissingInputReasons MissingInputReasons               `json:"missing_input_reasons"`
}
