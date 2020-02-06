package atc

type DebugVersionsDB struct {
	Jobs             []DebugJob
	Resources        []DebugResource
	ResourceVersions []DebugResourceVersion
	BuildOutputs     []DebugBuildOutput
	BuildInputs      []DebugBuildInput
	BuildReruns      []DebugBuildRerun

	// backwards-compatibility with pre-6.0 VersionsDB
	LegacyJobIDs      map[string]int `json:"JobIDs,omitempty"`
	LegacyResourceIDs map[string]int `json:"ResourceIDs,omitempty"`
}

type DebugResourceVersion struct {
	VersionID  int
	ResourceID int
	CheckOrder int

	// not present pre-6.0
	ScopeID int
}

type DebugBuildOutput struct {
	DebugResourceVersion
	BuildID int
	JobID   int
}

type DebugBuildInput struct {
	DebugResourceVersion
	BuildID   int
	JobID     int
	InputName string
}

type DebugBuildRerun struct {
	BuildID int
	JobID   int
	RerunOf int
}

type DebugJob struct {
	Name string
	ID   int
}

type DebugResource struct {
	Name    string
	ID      int
	ScopeID *int
}
