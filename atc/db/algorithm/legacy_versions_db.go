package algorithm

type LegacyVersionsDB struct {
	ResourceVersions []LegacyResourceVersion
	BuildOutputs     []LegacyBuildOutput
	BuildInputs      []LegacyBuildInput
	JobIDs           map[string]int
	ResourceIDs      map[string]int
}

type LegacyResourceVersion struct {
	VersionID  int
	ResourceID int
	CheckOrder int
}

type LegacyBuildOutput struct {
	LegacyResourceVersion
	BuildID int
	JobID   int
}

type LegacyBuildInput struct {
	LegacyResourceVersion
	BuildID   int
	JobID     int
	InputName string
}
