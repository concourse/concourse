package event

import "github.com/concourse/atc"

// move events here as they cease to be emitted by new code.
//
// denormalize the any external consts (e.g. type) so these are self-contained.

type InputV10 struct {
	Input LegacyTurbineInput `json:"input"`
}

type LegacyTurbineInput struct {
	Name     string                 `json:"name"`
	Resource string                 `json:"resource"`
	Type     string                 `json:"type"`
	Version  map[string]interface{} `json:"version,omitempty"`
	Metadata []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"metadata,omitempty"`
	ConfigPath string `json:"config_path"`
}

func (InputV10) EventType() atc.EventType  { return "input" }
func (InputV10) Version() atc.EventVersion { return "1.0" }

type OutputV10 struct {
	Output LegacyTurbineOutput `json:"output"`
}

type LegacyTurbineOutput struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	On       []string               `json:"on,omitempty"`
	Version  map[string]interface{} `json:"version,omitempty"`
	Metadata []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"metadata,omitempty"`
}

func (OutputV10) EventType() atc.EventType  { return "output" }
func (OutputV10) Version() atc.EventVersion { return "1.0" }

type LogV10 struct {
	Origin  OriginV10 `json:"origin"`
	Payload string    `json:"payload"`
}

func (LogV10) EventType() atc.EventType  { return "log" }
func (LogV10) Version() atc.EventVersion { return "1.0" }

type LogV20 struct {
	Origin  OriginV20 `json:"origin"`
	Payload string    `json:"payload"`
}

func (LogV20) EventType() atc.EventType  { return "log" }
func (LogV20) Version() atc.EventVersion { return "2.0" }

type OriginV10 struct {
	Type OriginV10Type `json:"type"`
	Name string        `json:"name"`
}

type OriginV10Type string

const (
	OriginV10TypeInvalid OriginV10Type = ""
	OriginV10TypeInput   OriginV10Type = "input"
	OriginV10TypeOutput  OriginV10Type = "output"
	OriginV10TypeRun     OriginV10Type = "run"
)

type OriginV20 struct {
	Name     string            `json:"name"`
	Type     OriginV20Type     `json:"type"`
	Source   OriginV20Source   `json:"source"`
	Location OriginV20Location `json:"location,omitempty"`
	Substep  bool              `json:"substep"`
	Hook     string            `json:"hook"`
}

type OriginV20Type string

const (
	OriginV20TypeInvalid OriginV20Type = ""
	OriginV20TypeGet     OriginV20Type = "get"
	OriginV20TypePut     OriginV20Type = "put"
	OriginV20TypeTask    OriginV20Type = "task"
)

type OriginV20Source string

const (
	OriginV20SourceStdout OriginV20Source = "stdout"
	OriginV20SourceStderr OriginV20Source = "stderr"
)

type OriginV20LocationIncrement uint

const (
	NoIncrementV10     OriginV20LocationIncrement = 0
	SingleIncrementV10 OriginV20LocationIncrement = 1
)

type OriginV20Location []uint

type FinishV10 struct {
	Time       int64 `json:"time"`
	ExitStatus int   `json:"exit_status"`
}

func (FinishV10) EventType() atc.EventType  { return "finish" }
func (FinishV10) Version() atc.EventVersion { return "1.0" }

type FinishTaskV10 struct {
	Time       int64     `json:"time"`
	ExitStatus int       `json:"exit_status"`
	Origin     OriginV20 `json:"origin"`
}

func (FinishTaskV10) EventType() atc.EventType  { return "finish-task" }
func (FinishTaskV10) Version() atc.EventVersion { return "1.0" }

type StartV10 struct {
	Time int64 `json:"time"`
}

type FinishGetV10 struct {
	Origin          OriginV20           `json:"origin"`
	Plan            GetPlan             `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGetV10) EventType() atc.EventType  { return "finish-get" }
func (FinishGetV10) Version() atc.EventVersion { return "1.0" }

type FinishPutV10 struct {
	Origin          OriginV20           `json:"origin"`
	Plan            PutPlan             `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
	ExitStatus      int                 `json:"exit_status"`
}

func (FinishPutV10) EventType() atc.EventType  { return "finish-put" }
func (FinishPutV10) Version() atc.EventVersion { return "1.0" }

func (StartV10) EventType() atc.EventType  { return "start" }
func (StartV10) Version() atc.EventVersion { return "1.0" }

type StartTaskV10 struct {
	Time   int64     `json:"time"`
	Origin OriginV20 `json:"origin"`
}

func (StartTaskV10) EventType() atc.EventType  { return "start-task" }
func (StartTaskV10) Version() atc.EventVersion { return "1.0" }

type InitializeV10 struct {
	TaskConfig TaskConfig `json:"config"`
}

func (InitializeV10) EventType() atc.EventType  { return "initialize" }
func (InitializeV10) Version() atc.EventVersion { return "1.0" }

type InitializeTaskV10 struct {
	TaskConfig TaskConfig `json:"config"`
	Origin     OriginV20  `json:"origin"`
}

func (InitializeTaskV10) EventType() atc.EventType  { return "initialize-task" }
func (InitializeTaskV10) Version() atc.EventVersion { return "1.0" }

type InputV20 struct {
	Plan            InputV20InputPlan   `json:"plan"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (InputV20) EventType() atc.EventType  { return "input" }
func (InputV20) Version() atc.EventVersion { return "2.0" }

type OutputV20 struct {
	Plan            OutputV20OutputPlan `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (OutputV20) EventType() atc.EventType  { return "output" }
func (OutputV20) Version() atc.EventVersion { return "2.0" }

type InputV20InputPlan struct {
	// logical name of the input with respect to the task's config
	Name string `json:"name"`

	// name of resource providing the input
	Resource string `json:"resource"`

	// type of resource
	Type string `json:"type"`

	// e.g. sha
	Version atc.Version `json:"version,omitempty"`
}

type OutputV20OutputPlan struct {
	Name string `json:"name"`

	Type string `json:"type"`

	// e.g. [success, failure]
	On atc.Conditions `json:"on,omitempty"`
}

type ErrorV10 struct {
	Message string    `json:"message"`
	Origin  OriginV20 `json:"origin,omitempty"`
}

func (ErrorV10) EventType() atc.EventType  { return "error" }
func (ErrorV10) Version() atc.EventVersion { return "1.0" }
