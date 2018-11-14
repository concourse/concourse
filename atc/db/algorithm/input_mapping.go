package algorithm

type InputMapping map[string]InputVersion

type InputVersion struct {
	ResourceID      int
	VersionID       int
	FirstOccurrence bool
}
