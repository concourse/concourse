package db

type BuildPreparationStatus string

const (
	BuildPreparationStatusBlocking    BuildPreparationStatus = "blocking"
	BuildPreparationStatusNotBlocking BuildPreparationStatus = "not_blocking"
)

const MissingBuildInput string = "input is not included in resolved candidates"

type MissingInputReasons map[string]string

func (m MissingInputReasons) RegisterMissingInput(inputName string) {
	m[inputName] = MissingBuildInput
}

func (m MissingInputReasons) RegisterResolveError(inputName string, resolveErr string) {
	m[inputName] = resolveErr
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
