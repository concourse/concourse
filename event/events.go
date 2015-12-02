package event

import "github.com/concourse/atc"

type Error struct {
	Message string `json:"message"`
	Origin  Origin `json:"origin,omitempty"`
}

func (Error) EventType() atc.EventType  { return EventTypeError }
func (Error) Version() atc.EventVersion { return "3.0" }

type FinishTask struct {
	Time       int64  `json:"time"`
	ExitStatus int    `json:"exit_status"`
	Origin     Origin `json:"origin"`
}

func (FinishTask) EventType() atc.EventType  { return EventTypeFinishTask }
func (FinishTask) Version() atc.EventVersion { return "3.0" }

type InitializeTask struct {
	TaskConfig TaskConfig `json:"config"`
	Origin     Origin     `json:"origin"`
}

func (InitializeTask) EventType() atc.EventType  { return EventTypeInitializeTask }
func (InitializeTask) Version() atc.EventVersion { return "3.0" }

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
		Image:    config.Image,
		Run: TaskRunConfig{
			Path: config.Run.Path,
			Args: config.Run.Args,
		},
		Inputs: inputConfigs,
	}
}

type StartTask struct {
	Time   int64  `json:"time"`
	Origin Origin `json:"origin"`
}

func (StartTask) EventType() atc.EventType  { return EventTypeStartTask }
func (StartTask) Version() atc.EventVersion { return "3.0" }

type Status struct {
	Status atc.BuildStatus `json:"status"`
	Time   int64           `json:"time"`
}

func (Status) EventType() atc.EventType  { return EventTypeStatus }
func (Status) Version() atc.EventVersion { return "1.0" }

type Log struct {
	Origin  Origin `json:"origin"`
	Payload string `json:"payload"`
}

func (Log) EventType() atc.EventType  { return EventTypeLog }
func (Log) Version() atc.EventVersion { return "4.0" }

type Origin struct {
	Name     string         `json:"name"`
	Type     OriginType     `json:"type"`
	Source   OriginSource   `json:"source"`
	Location OriginLocation `json:"location,omitempty"`
}

type OriginType string

const (
	OriginTypeInvalid OriginType = ""
	OriginTypeGet     OriginType = "get"
	OriginTypePut     OriginType = "put"
	OriginTypeTask    OriginType = "task"
)

type OriginSource string

const (
	OriginSourceStdout OriginSource = "stdout"
	OriginSourceStderr OriginSource = "stderr"
)

func OriginLocationFrom(location atc.Location) OriginLocation {
	return OriginLocation{
		ParentID:      location.ParentID,
		ParallelGroup: location.ParallelGroup,
		ID:            location.ID,
		Hook:          location.Hook,
		SerialGroup:   location.SerialGroup,
	}
}

type OriginLocation struct {
	ParentID      uint   `json:"parent_id"`
	ID            uint   `json:"id"`
	ParallelGroup uint   `json:"parallel_group"`
	SerialGroup   uint   `json:"serial_group"`
	Hook          string `json:"hook"`
}

func (ol OriginLocation) Incr(by OriginLocationIncrement) OriginLocation {
	ol.ID += uint(by)
	return ol
}

func (ol OriginLocation) SetParentID(id uint) OriginLocation {
	ol.ParentID = id
	return ol
}

type OriginLocationIncrement uint

const (
	NoIncrement     OriginLocationIncrement = 0
	SingleIncrement OriginLocationIncrement = 1
)

type FinishGet struct {
	Origin          Origin              `json:"origin"`
	Plan            GetPlan             `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGet) EventType() atc.EventType  { return EventTypeFinishGet }
func (FinishGet) Version() atc.EventVersion { return "3.0" }

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
func (FinishPut) Version() atc.EventVersion { return "3.0" }

type PutPlan struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Type     string `json:"type"`
}
