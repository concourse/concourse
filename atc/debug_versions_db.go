package atc

type DebugVersionsDB struct {
	ResourceVersions []DebugResourceVersion
	BuildOutputs     []DebugBuildOutput
	BuildInputs      []DebugBuildInput
	BuildReruns      []DebugBuildRerun
	JobIDs           map[string]int
	ResourceIDs      map[string]int
}

type DebugResourceVersion struct {
	VersionID  int
	ResourceID int
	CheckOrder int
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
	RerunOf int
}
