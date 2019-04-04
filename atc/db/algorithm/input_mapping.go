package algorithm

type InputMapping map[string]InputSource

type InputSource struct {
	InputVersion   InputVersion
	PassedBuildIDs []int
}

type InputVersion struct {
	ResourceID      int
	VersionID       int
	FirstOccurrence bool
}
