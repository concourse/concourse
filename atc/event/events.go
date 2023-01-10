package event

import (
	"encoding/json"

	"github.com/concourse/concourse/atc"
)

type Error struct {
	Message string `json:"message"`
	Origin  Origin `json:"origin"`
	Time    int64  `json:"time"`
}

func (Error) EventType() atc.EventType  { return EventTypeError }
func (Error) Version() atc.EventVersion { return "4.1" }

type FinishTask struct {
	Time       int64  `json:"time"`
	ExitStatus int    `json:"exit_status"`
	Origin     Origin `json:"origin"`
}

func (FinishTask) EventType() atc.EventType  { return EventTypeFinishTask }
func (FinishTask) Version() atc.EventVersion { return "4.0" }

type InitializeTask struct {
	Time       int64      `json:"time"`
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

type WaitingForWorker struct {
	Time   int64  `json:"time"`
	Origin Origin `json:"origin"`
}

func (WaitingForWorker) EventType() atc.EventType  { return EventTypeWaitingForWorker }
func (WaitingForWorker) Version() atc.EventVersion { return "1.0" }

type SelectedWorker struct {
	Time       int64  `json:"time"`
	Origin     Origin `json:"origin"`
	WorkerName string `json:"selected_worker"`
}

func (SelectedWorker) EventType() atc.EventType  { return EventTypeSelectedWorker }
func (SelectedWorker) Version() atc.EventVersion { return "1.0" }

type StreamingVolume struct {
	Time         int64  `json:"time"`
	Origin       Origin `json:"origin"`
	Volume       string `json:"volume"`
	SourceWorker string `json:"source_worker"`
	DestWorker   string `json:"dest_worker"`
}

func (StreamingVolume) EventType() atc.EventType  { return EventTypeStreamingVolume }
func (StreamingVolume) Version() atc.EventVersion { return "1.0" }

type WaitingForStreamedVolume struct {
	Time       int64  `json:"time"`
	Origin     Origin `json:"origin"`
	Volume     string `json:"volume"`
	DestWorker string `json:"dest_worker"`
}

func (WaitingForStreamedVolume) EventType() atc.EventType  { return EventTypeWaitingForStreamedVolume }
func (WaitingForStreamedVolume) Version() atc.EventVersion { return "1.0" }

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

func (id OriginID) String() string {
	return string(id)
}

type OriginSource string

const (
	OriginSourceStdout OriginSource = "stdout"
	OriginSourceStderr OriginSource = "stderr"
)

type InitializeCheck struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
	Name   string `json:"name"`
}

func (InitializeCheck) EventType() atc.EventType  { return EventTypeInitializeCheck }
func (InitializeCheck) Version() atc.EventVersion { return "1.0" }

type InitializeGet struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
}

func (InitializeGet) EventType() atc.EventType  { return EventTypeInitializeGet }
func (InitializeGet) Version() atc.EventVersion { return "2.0" }

type StartGet struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
}

func (StartGet) EventType() atc.EventType  { return EventTypeStartGet }
func (StartGet) Version() atc.EventVersion { return "1.0" }

type FinishGet struct {
	Origin          Origin              `json:"origin"`
	Time            int64               `json:"time"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGet) EventType() atc.EventType  { return EventTypeFinishGet }
func (FinishGet) Version() atc.EventVersion { return "5.1" }

type InitializePut struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
}

func (InitializePut) EventType() atc.EventType  { return EventTypeInitializePut }
func (InitializePut) Version() atc.EventVersion { return "2.0" }

type StartPut struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
}

func (StartPut) EventType() atc.EventType  { return EventTypeStartPut }
func (StartPut) Version() atc.EventVersion { return "1.0" }

type FinishPut struct {
	Origin          Origin              `json:"origin"`
	Time            int64               `json:"time"`
	ExitStatus      int                 `json:"exit_status"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishPut) EventType() atc.EventType  { return EventTypeFinishPut }
func (FinishPut) Version() atc.EventVersion { return "5.1" }

type SetPipelineChanged struct {
	Origin  Origin `json:"origin"`
	Changed bool   `json:"changed"`
}

func (SetPipelineChanged) EventType() atc.EventType  { return EventTypeSetPipelineChanged }
func (SetPipelineChanged) Version() atc.EventVersion { return "1.0" }

type Initialize struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
}

func (Initialize) EventType() atc.EventType  { return EventTypeInitialize }
func (Initialize) Version() atc.EventVersion { return "1.0" }

type Start struct {
	Origin Origin `json:"origin"`
	Time   int64  `json:"time,omitempty"`
}

func (Start) EventType() atc.EventType  { return EventTypeStart }
func (Start) Version() atc.EventVersion { return "1.0" }

type Finish struct {
	Origin    Origin `json:"origin"`
	Time      int64  `json:"time"`
	Succeeded bool   `json:"succeeded"`
}

func (Finish) EventType() atc.EventType  { return EventTypeFinish }
func (Finish) Version() atc.EventVersion { return "1.0" }

type ImageCheck struct {
	Time       int64            `json:"time"`
	Origin     Origin           `json:"origin"`
	PublicPlan *json.RawMessage `json:"plan"`
}

func (ImageCheck) EventType() atc.EventType  { return EventTypeImageCheck }
func (ImageCheck) Version() atc.EventVersion { return "1.1" }

type ImageGet struct {
	Time       int64            `json:"time"`
	Origin     Origin           `json:"origin"`
	PublicPlan *json.RawMessage `json:"plan"`
}

func (ImageGet) EventType() atc.EventType  { return EventTypeImageGet }
func (ImageGet) Version() atc.EventVersion { return "1.1" }

type AcrossSubsteps struct {
	Time     int64              `json:"time"`
	Origin   Origin             `json:"origin"`
	Substeps []*json.RawMessage `json:"substeps"`
}

func (AcrossSubsteps) EventType() atc.EventType  { return EventTypeAcrossSubsteps }
func (AcrossSubsteps) Version() atc.EventVersion { return "1.0" }
