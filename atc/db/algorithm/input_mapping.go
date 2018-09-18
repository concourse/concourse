package algorithm

type InputMapping map[string]InputVersion

type InputVersion struct {
	VersionID       int
	FirstOccurrence bool
}
