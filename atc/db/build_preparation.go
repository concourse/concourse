package db

type BuildPreparationStatus string

const (
	BuildPreparationStatusUnknown     BuildPreparationStatus = "unknown"
	BuildPreparationStatusBlocking    BuildPreparationStatus = "blocking"
	BuildPreparationStatusNotBlocking BuildPreparationStatus = "not_blocking"
)

const MissingBuildInput string = "input has not yet attempted to be resolved"

type MissingInputReasons map[string]string

func (m MissingInputReasons) RegisterMissingInput(inputName string) {
	m[inputName] = MissingBuildInput
}

func (m MissingInputReasons) RegisterResolveError(inputName string, resolveErr error) {
	m[inputName] = resolveErr.Error()
}

type BuildPreparation struct {
	BuildID             int
	PausedPipeline      BuildPreparationStatus
	PausedJob           BuildPreparationStatus
	MaxRunningBuilds    BuildPreparationStatus
	Inputs              map[string]BuildPreparationStatus
	InputsSatisfied     BuildPreparationStatus
	MissingInputReasons MissingInputReasons
}
