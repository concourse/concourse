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

type LogV30 struct {
	Origin  OriginV30 `json:"origin"`
	Payload string    `json:"payload"`
}

func (LogV30) EventType() atc.EventType  { return "log" }
func (LogV30) Version() atc.EventVersion { return "3.0" }

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

type OriginV30 struct {
	Name     string            `json:"name"`
	Type     OriginV30Type     `json:"type"`
	Source   OriginV30Source   `json:"source"`
	Location OriginV30Location `json:"location,omitempty"`
	Hook     string            `json:"hook"`
}

type OriginV30Type string

const (
	OriginV30TypeInvalid OriginV30Type = ""
	OriginV30TypeGet     OriginV30Type = "get"
	OriginV30TypePut     OriginV30Type = "put"
	OriginV30TypeTask    OriginV30Type = "task"
)

type OriginV30Source string

const (
	OriginV30SourceStdout OriginV30Source = "stdout"
	OriginV30SourceStderr OriginV30Source = "stderr"
)

type OriginV30Location struct {
	ParentID      uint   `json:"parent_id"`
	ID            uint   `json:"id"`
	ParallelGroup uint   `json:"parallel_group"`
	SerialGroup   uint   `json:"serial_group"`
	Hook          string `json:"hook"`
}

type OriginV30LocationIncrement uint

const (
	NoIncrementV20     OriginV30LocationIncrement = 0
	SingleIncrementV20 OriginV30LocationIncrement = 1
)

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

type FinishTaskV20 struct {
	Time       int64     `json:"time"`
	ExitStatus int       `json:"exit_status"`
	Origin     OriginV30 `json:"origin"`
}

func (FinishTaskV20) EventType() atc.EventType  { return "finish-task" }
func (FinishTaskV20) Version() atc.EventVersion { return "2.0" }

type StartV10 struct {
	Time int64 `json:"time"`
}

type FinishGetV10 struct {
	Origin          OriginV20           `json:"origin"`
	Plan            GetPlanV40          `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGetV10) EventType() atc.EventType  { return "finish-get" }
func (FinishGetV10) Version() atc.EventVersion { return "1.0" }

type FinishGetV20 struct {
	Origin          OriginV30           `json:"origin"`
	Plan            GetPlanV40          `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGetV20) EventType() atc.EventType  { return "finish-get" }
func (FinishGetV20) Version() atc.EventVersion { return "2.0" }

type FinishPutV10 struct {
	Origin          OriginV20           `json:"origin"`
	Plan            PutPlanV40          `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
	ExitStatus      int                 `json:"exit_status"`
}

func (FinishPutV10) EventType() atc.EventType  { return "finish-put" }
func (FinishPutV10) Version() atc.EventVersion { return "1.0" }

type FinishPutV20 struct {
	Origin          OriginV30           `json:"origin"`
	Plan            PutPlanV40          `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
	ExitStatus      int                 `json:"exit_status"`
}

func (FinishPutV20) EventType() atc.EventType  { return "finish-put" }
func (FinishPutV20) Version() atc.EventVersion { return "2.0" }

func (StartV10) EventType() atc.EventType  { return "start" }
func (StartV10) Version() atc.EventVersion { return "1.0" }

type StartTaskV10 struct {
	Time   int64     `json:"time"`
	Origin OriginV20 `json:"origin"`
}

func (StartTaskV10) EventType() atc.EventType  { return "start-task" }
func (StartTaskV10) Version() atc.EventVersion { return "1.0" }

type StartTaskV20 struct {
	Time   int64     `json:"time"`
	Origin OriginV30 `json:"origin"`
}

func (StartTaskV20) EventType() atc.EventType  { return "start-task" }
func (StartTaskV20) Version() atc.EventVersion { return "2.0" }

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

type InitializeTaskV20 struct {
	TaskConfig TaskConfig `json:"config"`
	Origin     OriginV30  `json:"origin"`
}

func (InitializeTaskV20) EventType() atc.EventType  { return "initialize-task" }
func (InitializeTaskV20) Version() atc.EventVersion { return "2.0" }

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
	On []string `json:"on,omitempty"`
}

type ErrorV10 struct {
	Message string    `json:"message"`
	Origin  OriginV20 `json:"origin,omitempty"`
}

func (ErrorV10) EventType() atc.EventType  { return "error" }
func (ErrorV10) Version() atc.EventVersion { return "1.0" }

type ErrorV20 struct {
	Message string    `json:"message"`
	Origin  OriginV30 `json:"origin,omitempty"`
}

func (ErrorV20) EventType() atc.EventType  { return "error" }
func (ErrorV20) Version() atc.EventVersion { return "2.0" }

// [Tracker: #97774988] Location -> PlanID changed Origin, which changes a bunch of stuff

type ErrorV30 struct {
	Message string    `json:"message"`
	Origin  OriginV40 `json:"origin,omitempty"`
}

func (ErrorV30) EventType() atc.EventType  { return "error" }
func (ErrorV30) Version() atc.EventVersion { return "3.0" }

type FinishTaskV30 struct {
	Time       int64     `json:"time"`
	ExitStatus int       `json:"exit_status"`
	Origin     OriginV40 `json:"origin"`
}

func (FinishTaskV30) EventType() atc.EventType  { return "finish-task" }
func (FinishTaskV30) Version() atc.EventVersion { return "3.0" }

type InitializeTaskV30 struct {
	TaskConfig TaskConfig `json:"config"`
	Origin     OriginV40  `json:"origin"`
}

func (InitializeTaskV30) EventType() atc.EventType  { return "initialize-task" }
func (InitializeTaskV30) Version() atc.EventVersion { return "3.0" }

type StartTaskV30 struct {
	Time   int64     `json:"time"`
	Origin OriginV40 `json:"origin"`
}

func (StartTaskV30) EventType() atc.EventType  { return "start-task" }
func (StartTaskV30) Version() atc.EventVersion { return "3.0" }

type LogV40 struct {
	Origin  OriginV40 `json:"origin"`
	Payload string    `json:"payload"`
}

func (LogV40) EventType() atc.EventType  { return "log" }
func (LogV40) Version() atc.EventVersion { return "4.0" }

type OriginV40 struct {
	Name     string            `json:"name"`
	Type     OriginV40Type     `json:"type"`
	Source   OriginSource      `json:"source"`
	Location OriginV40Location `json:"location,omitempty"`
}

type OriginV40Type string

const (
	OriginV40TypeInvalid OriginV40Type = ""
	OriginV40TypeGet     OriginV40Type = "get"
	OriginV40TypePut     OriginV40Type = "put"
	OriginV40TypeTask    OriginV40Type = "task"
)

type OriginV40Location struct {
	ParentID      uint   `json:"parent_id"`
	ID            uint   `json:"id"`
	ParallelGroup uint   `json:"parallel_group"`
	SerialGroup   uint   `json:"serial_group"`
	Hook          string `json:"hook"`
}

func (ol OriginV40Location) Incr(by OriginV40LocationIncrement) OriginV40Location {
	ol.ID += uint(by)
	return ol
}

func (ol OriginV40Location) SetParentID(id uint) OriginV40Location {
	ol.ParentID = id
	return ol
}

type OriginV40LocationIncrement uint

const (
	NoIncrementV30     OriginV40LocationIncrement = 0
	SingleIncrementV30 OriginV40LocationIncrement = 1
)

type LogV50 struct {
	Origin  Origin `json:"origin"`
	Payload string `json:"payload"`
}

func (LogV50) EventType() atc.EventType  { return "log" }
func (LogV50) Version() atc.EventVersion { return "5.0" }

type FinishGetV30 struct {
	Origin          OriginV40           `json:"origin"`
	Plan            GetPlanV40          `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGetV30) EventType() atc.EventType  { return "finish-get" }
func (FinishGetV30) Version() atc.EventVersion { return "3.0" }

type FinishPutV30 struct {
	Origin          OriginV40           `json:"origin"`
	Plan            PutPlanV40          `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
	ExitStatus      int                 `json:"exit_status"`
}

func (FinishPutV30) EventType() atc.EventType  { return "finish-put" }
func (FinishPutV30) Version() atc.EventVersion { return "3.0" }

type InitializeGetV10 struct {
	Origin Origin `json:"origin"`
}

func (InitializeGetV10) EventType() atc.EventType  { return "initialize-get" }
func (InitializeGetV10) Version() atc.EventVersion { return "1.0" }

type InitializePutV10 struct {
	Origin Origin `json:"origin"`
}

func (InitializePutV10) EventType() atc.EventType  { return "initialize-put" }
func (InitializePutV10) Version() atc.EventVersion { return "1.0" }

type StartTaskV40 struct {
	Time   int64  `json:"time"`
	Origin Origin `json:"origin"`
}

func (StartTaskV40) EventType() atc.EventType  { return "start-task" }
func (StartTaskV40) Version() atc.EventVersion { return "4.0" }

type GetPlanV40 struct {
	Name     string      `json:"name"`
	Resource string      `json:"resource"`
	Type     string      `json:"type"`
	Version  atc.Version `json:"version"`
}

type PutPlanV40 struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Type     string `json:"type"`
}

type FinishGetV40 struct {
	Origin          Origin              `json:"origin"`
	Plan            GetPlanV40          `json:"plan"`
	ExitStatus      int                 `json:"exit_status"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGetV40) EventType() atc.EventType  { return EventTypeFinishGet }
func (FinishGetV40) Version() atc.EventVersion { return "4.0" }

type FinishPutV40 struct {
	Origin          Origin              `json:"origin"`
	Plan            PutPlanV40          `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
	ExitStatus      int                 `json:"exit_status"`
}

func (FinishPutV40) EventType() atc.EventType  { return EventTypeFinishPut }
func (FinishPutV40) Version() atc.EventVersion { return "4.0" }
