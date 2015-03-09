package event

import "github.com/concourse/atc"

type Error struct {
	Message string `json:"message"`
	Origin  Origin `json:"origin,omitempty"`
}

func (Error) EventType() atc.EventType  { return EventTypeError }
func (Error) Version() atc.EventVersion { return "1.0" }
func (e Error) Censored() atc.Event     { return e }

type FinishExecute struct {
	Time       int64  `json:"time"`
	ExitStatus int    `json:"exit_status"`
	Origin     Origin `json:"origin"`
}

func (FinishExecute) EventType() atc.EventType  { return EventTypeFinishExecute }
func (FinishExecute) Version() atc.EventVersion { return "1.0" }
func (e FinishExecute) Censored() atc.Event     { return e }

type InitializeExecute struct {
	BuildConfig atc.BuildConfig `json:"config"`
	Origin      Origin          `json:"origin"`
}

func (InitializeExecute) EventType() atc.EventType  { return EventTypeInitializeExecute }
func (InitializeExecute) Version() atc.EventVersion { return "1.0" }
func (e InitializeExecute) Censored() atc.Event {
	e.BuildConfig.Params = nil
	return e
}

type StartExecute struct {
	Time   int64  `json:"time"`
	Origin Origin `json:"origin"`
}

func (StartExecute) EventType() atc.EventType  { return EventTypeStartExecute }
func (StartExecute) Version() atc.EventVersion { return "1.0" }
func (e StartExecute) Censored() atc.Event     { return e }

type Status struct {
	Status atc.BuildStatus `json:"status"`
	Time   int64           `json:"time"`
}

func (Status) EventType() atc.EventType  { return EventTypeStatus }
func (Status) Version() atc.EventVersion { return "1.0" }
func (e Status) Censored() atc.Event     { return e }

type Log struct {
	Origin  Origin `json:"origin"`
	Payload string `json:"payload"`
}

func (Log) EventType() atc.EventType  { return EventTypeLog }
func (Log) Version() atc.EventVersion { return "2.0" }
func (e Log) Censored() atc.Event     { return e }

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
	OriginTypeExecute OriginType = "execute"
)

type OriginSource string

const (
	OriginSourceStdout OriginSource = "stdout"
	OriginSourceStderr OriginSource = "stderr"
)

type OriginLocation []uint

func (chain OriginLocation) Chain(id uint) OriginLocation {
	chainedID := make(OriginLocation, len(chain))
	copy(chainedID, chain)
	chainedID = append(chainedID, id)
	return chainedID
}

func (chain OriginLocation) Incr(by uint) OriginLocation {
	incredID := make(OriginLocation, len(chain))
	copy(incredID, chain)
	incredID[len(chain)-1] += by
	return incredID
}

type FinishGet struct {
	Origin          Origin              `json:"origin"`
	Plan            GetPlan             `json:"plan"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishGet) EventType() atc.EventType  { return EventTypeFinishGet }
func (FinishGet) Version() atc.EventVersion { return "1.0" }
func (e FinishGet) Censored() atc.Event {
	e.Plan.Source = nil
	e.Plan.Params = nil
	return e
}

type GetPlan struct {
	Name     string      `json:"name"`
	Resource string      `json:"resource"`
	Type     string      `json:"type"`
	Source   atc.Source  `json:"source"`
	Params   atc.Params  `json:"params"`
	Version  atc.Version `json:"version"`
}

type FinishPut struct {
	Origin          Origin              `json:"origin"`
	Plan            PutPlan             `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (FinishPut) EventType() atc.EventType  { return EventTypeFinishPut }
func (FinishPut) Version() atc.EventVersion { return "1.0" }
func (e FinishPut) Censored() atc.Event {
	e.Plan.Source = nil
	e.Plan.Params = nil
	return e
}

type PutPlan struct {
	Name     string     `json:"name"`
	Resource string     `json:"resource"`
	Type     string     `json:"type"`
	Source   atc.Source `json:"source"`
	Params   atc.Params `json:"params"`
}
