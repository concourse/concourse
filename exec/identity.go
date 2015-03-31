package exec

import "os"

type Identity struct{}

func (Identity) Using(prev Step, repo *SourceRepository) Step {
	return identityStep{prev}
}

type identityStep struct {
	Step
}

func (identityStep) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}
