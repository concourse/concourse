package db

type JobSet map[int]bool

type InputMapping map[string]InputResult

type InputResult struct {
	Input          AlgorithmInput
	PassedBuildIDs []int
	ResolveError   error
}

type AlgorithmVersion struct {
	ResourceID int
	VersionID  int
}

type AlgorithmInput struct {
	AlgorithmVersion
	FirstOccurrence bool
}
