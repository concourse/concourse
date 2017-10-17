package event

import "github.com/concourse/atc"

type Error struct {
	Message string `json:"message"`
	Origin  Origin `json:"origin,omitempty"`
}

func (Error) EventType() atc.EventType  { return EventTypeError }
func (Error) Version() atc.EventVersion { return "4.0" }

type FinishTask struct {
	Time       int64  `json:"time"`
	ExitStatus int    `json:"exit_status"`
	Origin     Origin `json:"origin"`
}

func (FinishTask) EventType() atc.EventType  { return EventTypeFinishTask }
func (FinishTask) Version() atc.EventVersion { return "4.0" }

type InitializeTask struct {
	Origin     Origin     `json:"origin"`
	TaskConfig TaskConfig `json:"config"`
}

func (InitializeTask) EventType() atc.EventType  { return EventTypeInitializeTask }
func (InitializeTask) Version() atc.EventVersion { return "4.0" }

// shadow the real atc.TaskConfig
type TaskConfig struct {
	Platform string `json:"platform"`
	Image    string `json:"image"`

	Run    TaskRunConfig     `json:"run"`
	Inputs []TaskInputConfig `json:"inputs"`
}

type TaskRunConfig struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
}

type TaskInputConfig struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func ShadowTaskConfig(config atc.TaskConfig) TaskConfig {
	var inputConfigs []TaskInputConfig

	for _, input := range config.Inputs {
		inputConfigs = append(inputConfigs, TaskInputConfig{
			Name: input.Name,
			Path: input.Path,
		})
	}

	return TaskConfig{
		Platform: config.Platform,
		Image:    config.RootfsURI,
		Run: TaskRunConfig{
			Path: config.Run.Path,
			Args: config.Run.Args,
			Dir:  config.Run.Dir,
		},
		Inputs: inputConfigs,
	}
}

type StartTask struct {
	Time       int64      `json:"time"`
	Origin     Origin     `json:"origin"`
	TaskConfig TaskConfig `json:"config"`
}

func (StartTask) EventType() atc.EventType  { return EventTypeStartTask }
func (StartTask) Version() atc.EventVersion { return "5.0" }

type Status struct {
	Status atc.BuildStatus `json:"status"`
	Time   int64           `json:"time"`
}

func (Status) EventType() atc.EventType  { return EventTypeStatus }
func (Status) Version() atc.EventVersion { return "1.0" }

type Log struct {
	Time    int64  `json:"time"`
	Origin  Origin `json:"origin"`
	Payload string `json:"payload"`
}

func (Log) EventType() atc.EventType  { return EventTypeLog }
func (Log) Version() atc.EventVersion { return "5.1" }

type Origin struct {
	ID     OriginID     `json:"id,omitempty"`
	Source OriginSource `json:"source,omitempty"`
}

type OriginID string

type OriginSource string

const (
	OriginSourceStdout OriginSource = "stdout"
	OriginSourceStderr OriginSource = "stderr"
)

type FinishGet struct {
	Origin          Origin              `json:"origin"`
	Plan            GetPlan             `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGet) EventType() atc.EventType  { return EventTypeFinishGet }
func (FinishGet) Version() atc.EventVersion { return "4.0" }

type GetPlan struct {
	Name     string      `json:"name"`
	Resource string      `json:"resource"`
	Type     string      `json:"type"`
	Version  atc.Version `json:"version"`
}

type FinishPut struct {
	Origin          Origin              `json:"origin"`
	Plan            PutPlan             `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
	ExitStatus      int                 `json:"exit_status"`
}

func (FinishPut) EventType() atc.EventType  { return EventTypeFinishPut }
func (FinishPut) Version() atc.EventVersion { return "4.0" }

type PutPlan struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Type     string `json:"type"`
}
