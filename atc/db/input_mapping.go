package db

type JobSet map[int]bool

type InputMapping map[string]InputResult

type InputResult struct {
	Input          *AlgorithmInput
	PassedBuildIDs []int
	ResolveError   error
}

type ResourceVersion string

type AlgorithmVersion struct {
	ResourceID int
	Version    ResourceVersion
}

type AlgorithmInput struct {
	AlgorithmVersion
	FirstOccurrence bool
}

type AlgorithmOutput struct {
	AlgorithmVersion
	InputName string
}
