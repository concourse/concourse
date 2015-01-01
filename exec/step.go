package exec

//go:generate counterfeiter . Step
type Step interface {
	Using(ArtifactSource) ArtifactSource
}
